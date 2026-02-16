/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-vcard"
	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"
	"github.com/skip2/go-qrcode"

	"github.com/humaidq/groundwave/db"
)

// ContactExchangeInfo represents the active public exchange link for contact view.
type ContactExchangeInfo struct {
	URL            string
	QRCode         string
	ExpiresAt      time.Time
	ExpiresIn      string
	CollectPhone   bool
	CollectEmail   bool
	AdditionalNote string
}

// CreateContactExchangeLink creates a one-time public link for collecting contact details.
func CreateContactExchangeLink(c flamego.Context, s session.Session) {
	contactID := strings.TrimSpace(c.Param("id"))
	if contactID == "" {
		SetErrorFlash(s, "Contact ID is required")
		c.Redirect("/contacts", http.StatusSeeOther)

		return
	}

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing contact exchange form", "contact_id", contactID, "error", err)
		SetErrorFlash(s, "Failed to parse form")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)

		return
	}

	collectPhone := isPrimaryChecked(c.Request().Form.Get("collect_phone"))
	collectEmail := isPrimaryChecked(c.Request().Form.Get("collect_email"))
	additionalNote := strings.TrimSpace(c.Request().Form.Get("additional_note"))

	_, err := db.CreateContactExchangeLink(
		c.Request().Context(),
		contactID,
		collectPhone,
		collectEmail,
		additionalNote,
		db.ContactExchangeLinkDefaultTTL,
	)
	if err != nil {
		logger.Error("Error creating contact exchange link", "contact_id", contactID, "error", err)

		if errors.Is(err, db.ErrContactExchangeCollectFieldEmpty) {
			SetErrorFlash(s, "Select at least one field to collect")
		} else {
			SetErrorFlash(s, "Failed to generate secure contact link")
		}

		c.Redirect("/contact/"+contactID, http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "Secure contact link generated")
	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
}

// ViewContactExchange renders the public exchange form.
func ViewContactExchange(c flamego.Context, t template.Template, data template.Data) {
	token := strings.TrimSpace(c.Param("token"))
	data["HideNav"] = true

	link, contact, meContact, err := loadContactExchangeContext(c.Request().Context(), token)
	if err != nil {
		logger.Error("Error loading contact exchange page", "error", err)
		renderInvalidContactExchange(t, data)

		return
	}

	if link == nil || contact == nil {
		renderInvalidContactExchange(t, data)

		return
	}

	populateContactExchangeTemplateData(data, contact, link, token, meContact)

	t.HTML(http.StatusOK, "contact_exchange_public")
}

// SubmitContactExchange handles the public exchange form submission.
func SubmitContactExchange(c flamego.Context, t template.Template, data template.Data) {
	token := strings.TrimSpace(c.Param("token"))
	data["HideNav"] = true

	link, contact, meContact, err := loadContactExchangeContext(c.Request().Context(), token)
	if err != nil {
		logger.Error("Error loading contact exchange submission", "error", err)
		renderInvalidContactExchange(t, data)

		return
	}

	if link == nil || contact == nil {
		renderInvalidContactExchange(t, data)

		return
	}

	populateContactExchangeTemplateData(data, contact, link, token, meContact)

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing public contact exchange form", "error", err)

		data["Error"] = "Failed to parse form"

		t.HTML(http.StatusBadRequest, "contact_exchange_public")

		return
	}

	phoneInput := strings.TrimSpace(c.Request().Form.Get("phone"))
	emailInput := strings.TrimSpace(c.Request().Form.Get("email"))

	data["FormPhone"] = phoneInput
	data["FormEmail"] = emailInput

	existingPhones, _ := data["ExistingPhones"].([]string)
	existingEmails, _ := data["ExistingEmails"].([]string)
	requirePhone, _ := data["RequirePhone"].(bool)
	requireEmail, _ := data["RequireEmail"].(bool)

	if !link.CollectPhone {
		phoneInput = ""
	}

	if !link.CollectEmail {
		emailInput = ""
	}

	if requirePhone && strings.TrimSpace(phoneInput) == "" {
		data["Error"] = "Phone number is required"

		t.HTML(http.StatusBadRequest, "contact_exchange_public")

		return
	}

	if requireEmail && strings.TrimSpace(emailInput) == "" {
		data["Error"] = "Email address is required"

		t.HTML(http.StatusBadRequest, "contact_exchange_public")

		return
	}

	if phoneInput != "" && !isValidPhone(phoneInput) {
		data["Error"] = "Phone number must have at least 7 digits"

		t.HTML(http.StatusBadRequest, "contact_exchange_public")

		return
	}

	emailToAdd := strings.TrimSpace(emailInput)
	if emailToAdd != "" && containsEmail(existingEmails, emailToAdd) {
		emailToAdd = ""
	}

	phoneToAdd := strings.TrimSpace(phoneInput)
	if phoneToAdd != "" && containsPhoneByDigits(existingPhones, phoneToAdd) {
		phoneToAdd = ""
	}

	if contact.CardDAVUUID != nil && *contact.CardDAVUUID != "" {
		if err := saveCardDAVContactExchangeData(c.Request().Context(), contact, emailToAdd, phoneToAdd); err != nil {
			logger.Error("Error saving CardDAV contact exchange data", "contact_id", contact.ID.String(), "error", err)

			data["Error"] = "Failed to save contact details"

			t.HTML(http.StatusInternalServerError, "contact_exchange_public")

			return
		}
	} else {
		if err := saveLocalContactExchangeData(c.Request().Context(), contact.ID.String(), emailToAdd, phoneToAdd); err != nil {
			logger.Error("Error saving local contact exchange data", "contact_id", contact.ID.String(), "error", err)

			data["Error"] = "Failed to save contact details"

			t.HTML(http.StatusInternalServerError, "contact_exchange_public")

			return
		}
	}

	if err := db.MarkContactExchangeLinkUsed(c.Request().Context(), token); err != nil {
		logger.Error("Error marking contact exchange link as used", "error", err)
		renderInvalidContactExchange(t, data)

		return
	}

	data["Success"] = true
	data["AddedPhone"] = phoneToAdd != ""
	data["AddedEmail"] = emailToAdd != ""
	data["SkippedPhoneDuplicate"] = phoneInput != "" && phoneToAdd == ""
	data["SkippedEmailDuplicate"] = emailInput != "" && emailToAdd == ""
	data["FormPhone"] = ""
	data["FormEmail"] = ""

	t.HTML(http.StatusOK, "contact_exchange_public")
}

// DownloadContactExchangeMeVCF serves the configured "Me" contact as a vCard.
func DownloadContactExchangeMeVCF(c flamego.Context) {
	token := strings.TrimSpace(c.Param("token"))
	if token == "" {
		c.ResponseWriter().WriteHeader(http.StatusNotFound)

		return
	}

	link, err := db.GetContactExchangeLinkByTokenAllowUsed(c.Request().Context(), token)
	if err != nil {
		logger.Error("Error loading contact exchange link for vcf", "error", err)
		c.ResponseWriter().WriteHeader(http.StatusNotFound)

		return
	}

	if link == nil {
		c.ResponseWriter().WriteHeader(http.StatusNotFound)

		return
	}

	meContact, err := db.GetMeContact(c.Request().Context())
	if err != nil {
		logger.Error("Error loading me-contact for vcf", "error", err)
		c.ResponseWriter().WriteHeader(http.StatusInternalServerError)

		return
	}

	if meContact == nil {
		c.ResponseWriter().WriteHeader(http.StatusNotFound)

		return
	}

	vCardBytes, err := buildContactVCard(meContact, link.AdditionalNote)
	if err != nil {
		logger.Error("Error building vcf", "error", err)
		c.ResponseWriter().WriteHeader(http.StatusInternalServerError)

		return
	}

	headers := c.ResponseWriter().Header()
	headers.Set("Content-Type", "text/vcard; charset=utf-8")
	headers.Set("Content-Disposition", "attachment; filename=\"my-contact.vcf\"")
	headers.Set("Content-Length", strconv.Itoa(len(vCardBytes)))
	headers.Set("X-Content-Type-Options", "nosniff")

	c.ResponseWriter().WriteHeader(http.StatusOK)

	if _, err := c.ResponseWriter().Write(vCardBytes); err != nil {
		logger.Error("Error writing vcf response", "error", err)
	}
}

func renderInvalidContactExchange(t template.Template, data template.Data) {
	data["HideNav"] = true
	data["Error"] = "This secure link is invalid or expired."

	t.HTML(http.StatusNotFound, "contact_exchange_public")
}

func loadContactExchangeContext(ctx context.Context, token string) (*db.ContactExchangeLink, *db.ContactDetail, *db.ContactDetail, error) {
	link, err := db.GetContactExchangeLinkByToken(ctx, token)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load contact exchange link by token: %w", err)
	}

	if link == nil {
		return nil, nil, nil, nil
	}

	contact, err := db.GetContact(ctx, link.ContactID.String())
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load contact for exchange link: %w", err)
	}

	meContact, err := db.GetMeContact(ctx)
	if err != nil {
		logger.Error("Error loading me-contact for exchange page", "error", err)

		meContact = nil
	}

	return link, contact, meContact, nil
}

func populateContactExchangeTemplateData(data template.Data, contact *db.ContactDetail, link *db.ContactExchangeLink, token string, meContact *db.ContactDetail) {
	existingPhones := make([]string, 0)
	if link.CollectPhone {
		existingPhones = collectContactPhoneValues(contact)
	}

	existingEmails := make([]string, 0)
	if link.CollectEmail {
		existingEmails = collectContactEmailValues(contact)
	}

	data["Contact"] = contact
	data["ContactName"] = contactExchangeGreetingName(contact)
	data["Token"] = token
	data["CollectPhone"] = link.CollectPhone
	data["CollectEmail"] = link.CollectEmail
	data["ExistingPhones"] = existingPhones
	data["ExistingEmails"] = existingEmails
	data["RequirePhone"] = link.CollectPhone && len(existingPhones) == 0
	data["RequireEmail"] = link.CollectEmail && len(existingEmails) == 0
	data["ExchangeExpiresIn"] = formatDuration(time.Until(link.ExpiresAt))
	data["ExchangeExpiresAt"] = link.ExpiresAt
	data["ExchangePath"] = "/xc/" + token

	if meContact != nil {
		data["MeContactName"] = meContact.NameDisplay
		data["MeVCFPath"] = "/xc/" + token + "/me.vcf"
	}
}

func contactExchangeGreetingName(contact *db.ContactDetail) string {
	if contact == nil {
		return ""
	}

	givenFirstWord := ""

	if contact.NameGiven != nil {
		givenFields := strings.Fields(strings.TrimSpace(*contact.NameGiven))
		if len(givenFields) > 0 {
			givenFirstWord = givenFields[0]
		}
	}

	familyName := ""
	if contact.NameFamily != nil {
		familyName = strings.TrimSpace(*contact.NameFamily)
	}

	switch {
	case givenFirstWord != "" && familyName != "":
		return givenFirstWord + " " + familyName
	case givenFirstWord != "":
		return givenFirstWord
	case familyName != "":
		return familyName
	default:
		return contact.NameDisplay
	}
}

func saveLocalContactExchangeData(ctx context.Context, contactID string, emailToAdd string, phoneToAdd string) error {
	if emailToAdd != "" {
		if err := db.AddEmail(ctx, db.AddEmailInput{
			ContactID: contactID,
			Email:     emailToAdd,
			EmailType: db.EmailPersonal,
			IsPrimary: false,
		}); err != nil {
			return fmt.Errorf("failed to add email: %w", err)
		}
	}

	if phoneToAdd != "" {
		if err := db.AddPhone(ctx, db.AddPhoneInput{
			ContactID: contactID,
			Phone:     phoneToAdd,
			PhoneType: db.PhoneCell,
			IsPrimary: false,
		}); err != nil {
			return fmt.Errorf("failed to add phone: %w", err)
		}
	}

	return nil
}

func saveCardDAVContactExchangeData(ctx context.Context, contact *db.ContactDetail, emailToAdd string, phoneToAdd string) error {
	if contact == nil {
		return db.ErrContactNotFound
	}

	if contact.CardDAVUUID == nil || *contact.CardDAVUUID == "" {
		return db.ErrContactNotLinkedToCardDAV
	}

	modified := false

	if emailToAdd != "" {
		contact.Emails = append(contact.Emails, db.ContactEmail{
			Email:     emailToAdd,
			EmailType: db.EmailPersonal,
			IsPrimary: false,
			Source:    "carddav",
		})
		modified = true
	}

	if phoneToAdd != "" {
		contact.Phones = append(contact.Phones, db.ContactPhone{
			Phone:     phoneToAdd,
			PhoneType: db.PhoneCell,
			IsPrimary: false,
			Source:    "carddav",
		})
		modified = true
	}

	if !modified {
		return nil
	}

	if err := db.UpdateCardDAVContact(ctx, contact); err != nil {
		return fmt.Errorf("failed to push update to CardDAV: %w", err)
	}

	if err := db.SyncContactFromCardDAV(ctx, contact.ID.String(), *contact.CardDAVUUID); err != nil {
		return fmt.Errorf("failed to sync contact from CardDAV after update: %w", err)
	}

	return nil
}

func collectContactPhoneValues(contact *db.ContactDetail) []string {
	phones := make([]string, 0, len(contact.Phones))
	for _, phone := range contact.Phones {
		trimmed := strings.TrimSpace(phone.Phone)
		if trimmed == "" {
			continue
		}

		phones = append(phones, trimmed)
	}

	return phones
}

func collectContactEmailValues(contact *db.ContactDetail) []string {
	emails := make([]string, 0, len(contact.Emails))
	for _, email := range contact.Emails {
		trimmed := strings.TrimSpace(email.Email)
		if trimmed == "" {
			continue
		}

		emails = append(emails, trimmed)
	}

	return emails
}

func containsEmail(existing []string, candidate string) bool {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return false
	}

	for _, value := range existing {
		if strings.EqualFold(strings.TrimSpace(value), candidate) {
			return true
		}
	}

	return false
}

func containsPhoneByDigits(existing []string, candidate string) bool {
	candidateDigits := phoneDigits(candidate)
	if candidateDigits == "" {
		return false
	}

	for _, value := range existing {
		if phoneDigits(value) == candidateDigits {
			return true
		}
	}

	return false
}

func phoneDigits(value string) string {
	return strings.Join(digitRegex.FindAllString(value, -1), "")
}

func generateQRCodeBase64(value string) (string, error) {
	png, err := qrcode.Encode(value, qrcode.Medium, 256)
	if err != nil {
		return "", fmt.Errorf("failed to generate qr code: %w", err)
	}

	return base64.StdEncoding.EncodeToString(png), nil
}

func buildContactVCard(contact *db.ContactDetail, additionalNote string) ([]byte, error) {
	if contact == nil {
		return nil, db.ErrContactNotFound
	}

	card := make(vcard.Card)
	card.SetValue(vcard.FieldUID, contact.ID.String())
	card.SetValue(vcard.FieldFormattedName, contact.NameDisplay)

	givenName := ""
	if contact.NameGiven != nil {
		givenName = strings.TrimSpace(*contact.NameGiven)
	}

	familyName := ""
	if contact.NameFamily != nil {
		familyName = strings.TrimSpace(*contact.NameFamily)
	}

	card.AddName(&vcard.Name{
		GivenName:  givenName,
		FamilyName: familyName,
	})

	if contact.PhotoURL != nil {
		photoURL := strings.TrimSpace(*contact.PhotoURL)
		if photoURL != "" {
			card.Add(vcard.FieldPhoto, &vcard.Field{
				Value: photoURL,
				Params: vcard.Params{
					vcard.ParamValue: []string{"uri"},
				},
			})
		}
	}

	if contact.Organization != nil && strings.TrimSpace(*contact.Organization) != "" {
		card.SetValue(vcard.FieldOrganization, strings.TrimSpace(*contact.Organization))
	}

	if contact.Title != nil && strings.TrimSpace(*contact.Title) != "" {
		card.SetValue(vcard.FieldTitle, strings.TrimSpace(*contact.Title))
	}

	trimmedAdditionalNote := strings.TrimSpace(additionalNote)
	if trimmedAdditionalNote != "" {
		card.SetValue(vcard.FieldNote, trimmedAdditionalNote)
	}

	for _, email := range contact.Emails {
		value := strings.TrimSpace(email.Email)
		if value == "" {
			continue
		}

		emailType := "home"

		switch email.EmailType {
		case db.EmailPersonal:
			emailType = "home"
		case db.EmailWork:
			emailType = "work"
		case db.EmailOther:
			emailType = "other"
		}

		params := vcard.Params{vcard.ParamType: []string{emailType}}
		if email.IsPrimary {
			params.Set(vcard.ParamPreferred, "1")
		}

		card.Add(vcard.FieldEmail, &vcard.Field{
			Value:  value,
			Params: params,
		})
	}

	for _, phone := range contact.Phones {
		value := strings.TrimSpace(phone.Phone)
		if value == "" {
			continue
		}

		phoneType := "cell"

		switch phone.PhoneType {
		case db.PhoneCell:
			phoneType = "cell"
		case db.PhoneHome:
			phoneType = "home"
		case db.PhoneWork:
			phoneType = "work"
		case db.PhoneFax:
			phoneType = "fax"
		case db.PhonePager:
			phoneType = "pager"
		case db.PhoneOther:
			phoneType = "other"
		}

		params := vcard.Params{vcard.ParamType: []string{phoneType}}
		if phone.IsPrimary {
			params.Set(vcard.ParamPreferred, "1")
		}

		card.Add(vcard.FieldTelephone, &vcard.Field{
			Value:  value,
			Params: params,
		})
	}

	vcard.ToV4(card)

	var buffer bytes.Buffer

	encoder := vcard.NewEncoder(&buffer)
	if err := encoder.Encode(card); err != nil {
		return nil, fmt.Errorf("failed to encode vcard: %w", err)
	}

	return buffer.Bytes(), nil
}
