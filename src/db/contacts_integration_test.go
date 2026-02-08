// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"
	"time"
)

func TestContactListFiltersAndServiceContacts(t *testing.T) {
	resetDatabase(t)
	ctx := testContext()

	primaryEmail := "alice@example.com"
	primaryPhone := "+1 555-0000"
	callSign := "AB1CD"

	contact1 := mustCreateContact(t, CreateContactInput{
		NameGiven: "Alice",
		Email:     &primaryEmail,
		Phone:     &primaryPhone,
		CallSign:  &callSign,
		Tier:      TierA,
	})
	contact2 := mustCreateContact(t, CreateContactInput{
		NameGiven: "Bob",
		Tier:      TierB,
	})

	serviceContact := mustCreateContact(t, CreateContactInput{
		NameGiven:    "Service",
		Organization: stringPtr("Service Org"),
		IsService:    true,
		Tier:         TierC,
	})

	if err := AddURL(ctx, AddURLInput{
		ContactID: contact1,
		URL:       "https://linkedin.com/in/alice",
		URLType:   URLLinkedIn,
	}); err != nil {
		t.Fatalf("AddURL failed: %v", err)
	}

	if err := AddTagToContact(ctx, contact1, "vip"); err != nil {
		t.Fatalf("AddTagToContact failed: %v", err)
	}

	contacts, err := ListContacts(ctx)
	if err != nil {
		t.Fatalf("ListContacts failed: %v", err)
	}
	if len(contacts) != 2 {
		t.Fatalf("expected 2 non-service contacts, got %d", len(contacts))
	}

	noEmail, err := ListContactsWithFilters(ctx, ContactListOptions{
		Filters:   []ContactFilter{FilterNoEmail},
		IsService: false,
	})
	if err != nil {
		t.Fatalf("ListContactsWithFilters failed: %v", err)
	}
	if len(noEmail) != 1 || noEmail[0].ID != contact2 {
		t.Fatalf("expected contact without email")
	}

	noPhone, err := ListContactsWithFilters(ctx, ContactListOptions{
		Filters:   []ContactFilter{FilterNoPhone},
		IsService: false,
	})
	if err != nil {
		t.Fatalf("ListContactsWithFilters failed: %v", err)
	}
	if len(noPhone) != 1 || noPhone[0].ID != contact2 {
		t.Fatalf("expected contact without phone")
	}

	noLinkedIn, err := ListContactsWithFilters(ctx, ContactListOptions{
		Filters:   []ContactFilter{FilterNoLinkedIn},
		IsService: false,
	})
	if err != nil {
		t.Fatalf("ListContactsWithFilters failed: %v", err)
	}
	if len(noLinkedIn) != 1 || noLinkedIn[0].ID != contact2 {
		t.Fatalf("expected contact without LinkedIn")
	}

	noCardDAV, err := ListContactsWithFilters(ctx, ContactListOptions{
		Filters:   []ContactFilter{FilterNoCardDAV},
		IsService: false,
	})
	if err != nil {
		t.Fatalf("ListContactsWithFilters failed: %v", err)
	}
	if len(noCardDAV) != 2 {
		t.Fatalf("expected contacts without CardDAV, got %d", len(noCardDAV))
	}

	tags, err := ListAllTags(ctx)
	if err != nil {
		t.Fatalf("ListAllTags failed: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(tags))
	}

	withTags, err := ListContactsWithFilters(ctx, ContactListOptions{
		TagIDs:    []string{tags[0].ID.String()},
		IsService: false,
	})
	if err != nil {
		t.Fatalf("ListContactsWithFilters failed: %v", err)
	}
	if len(withTags) != 1 || withTags[0].ID != contact1 {
		t.Fatalf("expected tagged contact")
	}

	if _, err := ListContactsWithFilters(ctx, ContactListOptions{AlphabeticSort: true}); err != nil {
		t.Fatalf("ListContactsWithFilters alphabetic failed: %v", err)
	}

	if _, err := ListContactsWithFilters(ctx, ContactListOptions{IsService: true}); err != nil {
		t.Fatalf("ListContactsWithFilters service failed: %v", err)
	}

	services, err := ListServiceContacts(ctx)
	if err != nil {
		t.Fatalf("ListServiceContacts failed: %v", err)
	}
	if len(services) != 1 || services[0].ID != serviceContact {
		t.Fatalf("expected 1 service contact")
	}

	if err := AddChat(ctx, AddChatInput{ContactID: serviceContact, Message: "ignored"}); err != nil {
		t.Fatalf("AddChat for service contact failed: %v", err)
	}
	serviceChats, err := GetContactChats(ctx, serviceContact)
	if err != nil {
		t.Fatalf("GetContactChats failed: %v", err)
	}
	if len(serviceChats) != 0 {
		t.Fatalf("expected no chats for service contact")
	}

	if err := ToggleServiceStatus(ctx, contact2, true); err != nil {
		t.Fatalf("ToggleServiceStatus failed: %v", err)
	}

	isService, err := IsServiceContact(ctx, contact2)
	if err != nil {
		t.Fatalf("IsServiceContact failed: %v", err)
	}
	if !isService {
		t.Fatalf("expected contact to be service")
	}

	services, err = ListServiceContacts(ctx)
	if err != nil {
		t.Fatalf("ListServiceContacts failed: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("expected 2 service contacts, got %d", len(services))
	}

	count, err := GetContactsCount(ctx)
	if err != nil {
		t.Fatalf("GetContactsCount failed: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 contacts, got %d", count)
	}

	recent, err := GetRecentContacts(ctx, 2)
	if err != nil {
		t.Fatalf("GetRecentContacts failed: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("expected 2 recent contacts, got %d", len(recent))
	}

	overdueTime := time.Now().AddDate(0, 0, -40)
	if _, err := pool.Exec(ctx, `UPDATE contacts SET created_at = $1, last_auto_contact = $1 WHERE id = $2`, overdueTime, contact1); err != nil {
		t.Fatalf("failed to adjust contact timestamps: %v", err)
	}

	overdue, err := GetOverdueContacts(ctx)
	if err != nil {
		t.Fatalf("GetOverdueContacts failed: %v", err)
	}
	if len(overdue) == 0 {
		t.Fatalf("expected overdue contacts")
	}
}

func TestContactDetailsAndLogs(t *testing.T) {
	resetDatabase(t)
	ctx := testContext()

	primaryEmail := "carlos@example.com"
	primaryPhone := "+1 555-1111"
	contactID := mustCreateContact(t, CreateContactInput{
		NameGiven: "Carlos",
		Email:     &primaryEmail,
		Phone:     &primaryPhone,
		Tier:      TierB,
	})

	if err := AddEmail(ctx, AddEmailInput{ContactID: contactID, Email: "alt@example.com", EmailType: EmailWork, IsPrimary: false}); err != nil {
		t.Fatalf("AddEmail failed: %v", err)
	}
	if err := AddPhone(ctx, AddPhoneInput{ContactID: contactID, Phone: "+1 555-2222", PhoneType: PhoneHome, IsPrimary: false}); err != nil {
		t.Fatalf("AddPhone failed: %v", err)
	}
	if err := AddURL(ctx, AddURLInput{ContactID: contactID, URL: "https://example.com", URLType: URLWebsite}); err != nil {
		t.Fatalf("AddURL failed: %v", err)
	}

	detail, err := GetContact(ctx, contactID)
	if err != nil {
		t.Fatalf("GetContact failed: %v", err)
	}
	if detail.NameDisplay != "Carlos" {
		t.Fatalf("expected name Carlos, got %q", detail.NameDisplay)
	}
	if len(detail.Emails) != 2 || len(detail.Phones) != 2 || len(detail.URLs) != 1 {
		t.Fatalf("expected emails/phones/urls to be populated")
	}

	primaryEmailID := detail.Emails[0].ID.String()
	if err := UpdateEmail(ctx, UpdateEmailInput{ID: primaryEmailID, ContactID: contactID, Email: "carlos@example.com", EmailType: EmailPersonal, IsPrimary: true}); err != nil {
		t.Fatalf("UpdateEmail failed: %v", err)
	}
	primaryPhoneID := detail.Phones[0].ID.String()
	if err := UpdatePhone(ctx, UpdatePhoneInput{ID: primaryPhoneID, ContactID: contactID, Phone: "+1 555-3333", PhoneType: PhoneCell, IsPrimary: true}); err != nil {
		t.Fatalf("UpdatePhone failed: %v", err)
	}

	if _, err := GetContactIDByEmailID(ctx, primaryEmailID); err != nil {
		t.Fatalf("GetContactIDByEmailID failed: %v", err)
	}
	if _, err := GetContactIDByPhoneID(ctx, primaryPhoneID); err != nil {
		t.Fatalf("GetContactIDByPhoneID failed: %v", err)
	}

	if err := UpdateContact(ctx, UpdateContactInput{ID: contactID, NameGiven: "Carlos", NameFamily: stringPtr("Diaz"), Tier: TierC}); err != nil {
		t.Fatalf("UpdateContact failed: %v", err)
	}

	logSubject := "Check in"
	logContent := "Met at conference"
	if err := AddLog(ctx, AddLogInput{ContactID: contactID, LogType: LogGeneral, Subject: &logSubject, Content: &logContent}); err != nil {
		t.Fatalf("AddLog failed: %v", err)
	}

	start := time.Now().AddDate(0, 0, -7)
	end := time.Now().AddDate(0, 0, 7)
	counts, err := ListContactWeeklyActivityCounts(ctx, contactID, start, end)
	if err != nil {
		t.Fatalf("ListContactWeeklyActivityCounts failed: %v", err)
	}
	if len(counts) == 0 {
		t.Fatalf("expected weekly activity counts")
	}

	timeline, err := ListContactLogsTimeline(ctx)
	if err != nil {
		t.Fatalf("ListContactLogsTimeline failed: %v", err)
	}
	if len(timeline) != 1 {
		t.Fatalf("expected 1 timeline entry, got %d", len(timeline))
	}
	if err := DeleteLog(ctx, timeline[0].ID.String()); err != nil {
		t.Fatalf("DeleteLog failed: %v", err)
	}
	counts, err = ListContactWeeklyActivityCounts(ctx, contactID, start, end)
	if err != nil {
		t.Fatalf("ListContactWeeklyActivityCounts failed: %v", err)
	}
	if len(counts) != 0 {
		t.Fatalf("expected no weekly counts after log deletion")
	}

	chatMessage := "Hello"
	if err := AddChat(ctx, AddChatInput{ContactID: contactID, Platform: ChatPlatformWhatsApp, Sender: ChatSenderThem, Message: chatMessage}); err != nil {
		t.Fatalf("AddChat failed: %v", err)
	}

	chats, err := GetContactChats(ctx, contactID)
	if err != nil {
		t.Fatalf("GetContactChats failed: %v", err)
	}
	if len(chats) != 1 {
		t.Fatalf("expected 1 chat, got %d", len(chats))
	}

	chatsSince, err := GetContactChatsSince(ctx, contactID, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("GetContactChatsSince failed: %v", err)
	}
	if len(chatsSince) != 1 {
		t.Fatalf("expected 1 chat since, got %d", len(chatsSince))
	}

	if err := AddNote(ctx, AddNoteInput{ContactID: contactID, Content: "Important note"}); err != nil {
		t.Fatalf("AddNote failed: %v", err)
	}

	detail, err = GetContact(ctx, contactID)
	if err != nil {
		t.Fatalf("GetContact failed: %v", err)
	}
	if len(detail.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(detail.Notes))
	}
	if err := DeleteNote(ctx, detail.Notes[0].ID.String()); err != nil {
		t.Fatalf("DeleteNote failed: %v", err)
	}

	if err := LinkCardDAV(ctx, contactID, "carddav-uuid"); err != nil {
		t.Fatalf("LinkCardDAV failed: %v", err)
	}
	linked, err := IsCardDAVUUIDLinked(ctx, "carddav-uuid")
	if err != nil {
		t.Fatalf("IsCardDAVUUIDLinked failed: %v", err)
	}
	if !linked {
		t.Fatalf("expected carddav uuid to be linked")
	}

	linkedUUIDs, err := GetLinkedCardDAVUUIDs(ctx)
	if err != nil {
		t.Fatalf("GetLinkedCardDAVUUIDs failed: %v", err)
	}
	if len(linkedUUIDs) != 1 {
		t.Fatalf("expected 1 linked uuid, got %d", len(linkedUUIDs))
	}

	statusMap, err := GetLinkedCardDAVUUIDsWithServiceStatus(ctx)
	if err != nil {
		t.Fatalf("GetLinkedCardDAVUUIDsWithServiceStatus failed: %v", err)
	}
	if len(statusMap) != 1 {
		t.Fatalf("expected 1 status map entry, got %d", len(statusMap))
	}

	if err := UnlinkCardDAV(ctx, contactID); err != nil {
		t.Fatalf("UnlinkCardDAV failed: %v", err)
	}

	for _, email := range detail.Emails {
		if err := DeleteEmail(ctx, email.ID.String(), contactID); err != nil {
			t.Fatalf("DeleteEmail failed: %v", err)
		}
	}
	for _, phone := range detail.Phones {
		if err := DeletePhone(ctx, phone.ID.String(), contactID); err != nil {
			t.Fatalf("DeletePhone failed: %v", err)
		}
	}
	for _, url := range detail.URLs {
		if err := DeleteURL(ctx, url.ID.String()); err != nil {
			t.Fatalf("DeleteURL failed: %v", err)
		}
	}

	if err := DeleteContact(ctx, contactID); err != nil {
		t.Fatalf("DeleteContact failed: %v", err)
	}
}
