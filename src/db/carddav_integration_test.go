// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"bytes"
	"encoding/xml"
	"github.com/emersion/go-vcard"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type carddavTestServer struct {
	server *httptest.Server
	mu     sync.Mutex
	cards  map[string]vcard.Card
}

func newCardDAVTestServer(t *testing.T) *carddavTestServer {
	t.Helper()

	cd := &carddavTestServer{cards: make(map[string]vcard.Card)}
	cd.server = httptest.NewServer(http.HandlerFunc(cd.handle))
	return cd
}

func (c *carddavTestServer) close() {
	if c.server != nil {
		c.server.Close()
	}
}

func (c *carddavTestServer) handle(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/addressbook") {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		c.handleGet(w, path)
	case http.MethodPut:
		c.handlePut(w, r, path)
	case "REPORT":
		c.handleReport(w, path)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (c *carddavTestServer) handleGet(w http.ResponseWriter, path string) {
	if path == "/addressbook" || path == "/addressbook/" {
		c.mu.Lock()
		cards := make([]vcard.Card, 0, len(c.cards))
		for _, card := range c.cards {
			cards = append(cards, card)
		}
		c.mu.Unlock()

		var buf bytes.Buffer
		enc := vcard.NewEncoder(&buf)
		for _, card := range cards {
			_ = enc.Encode(card)
		}
		w.Header().Set("Content-Type", "text/vcard")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buf.Bytes())
		return
	}

	key := strings.TrimPrefix(path, "/addressbook/")
	c.mu.Lock()
	card, ok := c.cards[key]
	c.mu.Unlock()
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var buf bytes.Buffer
	_ = vcard.NewEncoder(&buf).Encode(card)
	w.Header().Set("Content-Type", "text/vcard")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

func (c *carddavTestServer) handlePut(w http.ResponseWriter, r *http.Request, path string) {
	decoder := vcard.NewDecoder(r.Body)
	card, err := decoder.Decode()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	key := strings.TrimPrefix(path, "/addressbook/")
	c.mu.Lock()
	c.cards[key] = card
	c.mu.Unlock()
	w.WriteHeader(http.StatusCreated)
}

func (c *carddavTestServer) handleReport(w http.ResponseWriter, _ string) {
	c.mu.Lock()
	entries := make(map[string]vcard.Card, len(c.cards))
	for path, card := range c.cards {
		entries[path] = card
	}
	c.mu.Unlock()

	body := buildCardDAVMultiStatus(entries)
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = w.Write(body)
}

type cardDAVMultiStatus struct {
	XMLName  xml.Name          `xml:"DAV: multistatus"`
	Response []cardDAVResponse `xml:"response"`
}

type cardDAVResponse struct {
	Href     string          `xml:"href"`
	PropStat cardDAVPropStat `xml:"propstat"`
}

type cardDAVPropStat struct {
	Prop   cardDAVProp `xml:"prop"`
	Status string      `xml:"status"`
}

type cardDAVProp struct {
	AddressData cardDAVAddressData `xml:"urn:ietf:params:xml:ns:carddav address-data"`
}

type cardDAVAddressData struct {
	Data string `xml:",chardata"`
}

func buildCardDAVMultiStatus(cards map[string]vcard.Card) []byte {
	responses := make([]cardDAVResponse, 0, len(cards))
	for path, card := range cards {
		var buf bytes.Buffer
		_ = vcard.NewEncoder(&buf).Encode(card)
		responses = append(responses, cardDAVResponse{
			Href: "/addressbook/" + path,
			PropStat: cardDAVPropStat{
				Prop:   cardDAVProp{AddressData: cardDAVAddressData{Data: buf.String()}},
				Status: "HTTP/1.1 200 OK",
			},
		})
	}

	ms := cardDAVMultiStatus{Response: responses}
	output, _ := xml.Marshal(ms)
	return append([]byte(xml.Header), output...)
}

func TestCardDAVListAndHelpers(t *testing.T) {
	resetDatabase(t)
	server := newCardDAVTestServer(t)
	defer server.close()

	card := make(vcard.Card)
	card.SetValue(vcard.FieldUID, "card-1")
	card.SetValue(vcard.FieldFormattedName, "Alice Example")
	card.AddName(&vcard.Name{GivenName: "Alice", FamilyName: "Example"})
	card.Add(vcard.FieldEmail, &vcard.Field{Value: "alice@example.com", Params: vcard.Params{vcard.ParamType: []string{"work"}, vcard.ParamPreferred: []string{"1"}}})
	card.Add(vcard.FieldTelephone, &vcard.Field{Value: "+1 555 0000", Params: vcard.Params{vcard.ParamType: []string{"cell"}}})
	card.Add(vcard.FieldPhoto, &vcard.Field{Value: "dGVzdA==", Params: vcard.Params{vcard.ParamType: []string{"jpg"}}})
	server.cards["card-1.vcf"] = card

	card2 := make(vcard.Card)
	card2.SetValue(vcard.FieldUID, "card-2")
	card2.SetValue(vcard.FieldFormattedName, "Bob Example")
	card2.AddName(&vcard.Name{GivenName: "Bob", FamilyName: "Example"})
	server.cards["card-2.vcf"] = card2

	t.Setenv("CARDDAV_URL", server.server.URL+"/addressbook/")
	t.Setenv("CARDDAV_USERNAME", "user")
	t.Setenv("CARDDAV_PASSWORD", "pass")

	if _, err := GetCardDAVConfig(); err != nil {
		t.Fatalf("GetCardDAVConfig failed: %v", err)
	}

	contacts, err := ListCardDAVContacts(testContext())
	if err != nil {
		t.Fatalf("ListCardDAVContacts failed: %v", err)
	}
	if len(contacts) != 2 {
		t.Fatalf("expected 2 contacts, got %d", len(contacts))
	}

	contact, err := GetCardDAVContact(testContext(), "card-1")
	if err != nil {
		t.Fatalf("GetCardDAVContact failed: %v", err)
	}
	if contact.DisplayName != "Alice Example" {
		t.Fatalf("expected display name, got %q", contact.DisplayName)
	}

	if _, err := GetCardDAVContact(testContext(), "missing"); err == nil {
		t.Fatalf("expected error for missing contact")
	}

	photo := normalizeCardDAVPhoto(card.Get(vcard.FieldPhoto))
	if !strings.HasPrefix(photo, "data:image/jpeg;base64,") {
		t.Fatalf("expected data URL photo, got %q", photo)
	}

	if mediaTypeFromPhotoParams(vcard.Params{vcard.ParamType: []string{"png"}}) != "image/png" {
		t.Fatalf("expected png media type")
	}

	emails := []CardDAVEmail{{Email: "a@example.com", Preferred: true}, {Email: "b@example.com"}}
	if preferredEmailValue(emails) != "a@example.com" {
		t.Fatalf("expected preferred email")
	}
	phones := []CardDAVPhone{{Phone: "123", Preferred: true}, {Phone: "456"}}
	if preferredPhoneValue(phones) != "123" {
		t.Fatalf("expected preferred phone")
	}
	if normalizeCardDAVEmail(" Alice@Example.com ") != "alice@example.com" {
		t.Fatalf("expected normalized email")
	}
	if normalizeCardDAVPhone("+1 (555) 0000") != "15550000" {
		t.Fatalf("expected normalized phone")
	}
	if selectPrimaryEmail("a@example.com", []string{"b@example.com", "a@example.com"}) != "a@example.com" {
		t.Fatalf("expected selected primary email")
	}
	if selectPrimaryPhone("123", []string{"123", "456"}) != "123" {
		t.Fatalf("expected selected primary phone")
	}
	if _, err := parseDateString("2024-01-02"); err != nil {
		t.Fatalf("parseDateString failed: %v", err)
	}
}

func TestCardDAVSyncAndUpdate(t *testing.T) {
	resetDatabase(t)
	server := newCardDAVTestServer(t)
	defer server.close()

	card := make(vcard.Card)
	card.SetValue(vcard.FieldUID, "sync-1")
	card.SetValue(vcard.FieldFormattedName, "Sync User")
	card.AddName(&vcard.Name{GivenName: "Sync", FamilyName: "User"})
	card.Add(vcard.FieldEmail, &vcard.Field{Value: "sync@example.com", Params: vcard.Params{vcard.ParamType: []string{"home"}, vcard.ParamPreferred: []string{"1"}}})
	card.Add(vcard.FieldTelephone, &vcard.Field{Value: "+1 555 1111", Params: vcard.Params{vcard.ParamType: []string{"cell"}}})
	server.cards["sync-1.vcf"] = card

	t.Setenv("CARDDAV_URL", server.server.URL+"/addressbook/")
	t.Setenv("CARDDAV_USERNAME", "user")
	t.Setenv("CARDDAV_PASSWORD", "pass")

	carddavID := "sync-1"
	contactID := mustCreateContact(t, CreateContactInput{NameGiven: "Local", CardDAVUUID: &carddavID, Tier: TierB})

	if err := SyncContactFromCardDAV(testContext(), contactID, carddavID); err != nil {
		t.Fatalf("SyncContactFromCardDAV failed: %v", err)
	}

	detail, err := GetContact(testContext(), contactID)
	if err != nil {
		t.Fatalf("GetContact failed: %v", err)
	}
	if detail.NameDisplay == "" || len(detail.Emails) == 0 || len(detail.Phones) == 0 {
		t.Fatalf("expected synced contact details")
	}

	available, err := isCardDAVEmailAvailable(testContext(), contactID, "sync@example.com")
	if err != nil {
		t.Fatalf("isCardDAVEmailAvailable failed: %v", err)
	}
	if !available {
		t.Fatalf("expected email to be available for same contact")
	}

	if err := SyncAllCardDAVContacts(testContext()); err != nil {
		t.Fatalf("SyncAllCardDAVContacts failed: %v", err)
	}

	newUUID, err := CreateCardDAVContact(testContext(), detail)
	if err != nil {
		t.Fatalf("CreateCardDAVContact failed: %v", err)
	}
	if newUUID == "" {
		t.Fatalf("expected new uuid")
	}

	contact, err := GetContact(testContext(), contactID)
	if err != nil {
		t.Fatalf("GetContact failed: %v", err)
	}
	contact.CardDAVUUID = &carddavID
	if err := UpdateCardDAVContact(testContext(), contact); err != nil {
		t.Fatalf("UpdateCardDAVContact failed: %v", err)
	}

	localEmail := "local@example.com"
	localPhone := "+1 555 9999"
	localContactID := mustCreateContact(t, CreateContactInput{NameGiven: "Local", Email: &localEmail, Phone: &localPhone, Tier: TierB})
	if err := MigrateContactToCardDAV(testContext(), localContactID); err != nil {
		t.Fatalf("MigrateContactToCardDAV failed: %v", err)
	}
	updated, err := GetContact(testContext(), localContactID)
	if err != nil {
		t.Fatalf("GetContact failed: %v", err)
	}
	if updated.CardDAVUUID == nil || *updated.CardDAVUUID == "" {
		t.Fatalf("expected contact to be linked to CardDAV")
	}
	if len(updated.Emails) == 0 || len(updated.Phones) == 0 {
		t.Fatalf("expected emails and phones after migration")
	}
}
