// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/google/uuid"

	"github.com/humaidq/groundwave/db"
)

var (
	errTestBoom              = errors.New("boom")
	errTestLookupFailed      = errors.New("lookup failed")
	errTestShouldNotBeCalled = errors.New("should not be called")
)

func newMutatingHandlersTestApp(s session.Session) *flamego.Flame {
	f := flamego.New()
	f.Use(func(c flamego.Context) {
		c.MapTo(s, (*session.Session)(nil))
		c.Next()
	})

	f.Post("/contact/{id}/log/{log_id}/edit", func(c flamego.Context, sess session.Session) {
		UpdateLog(c, sess)
	})
	f.Post("/contact/{id}/note/{note_id}/edit", func(c flamego.Context, sess session.Session) {
		UpdateNote(c, sess)
	})
	f.Post("/contact/{id}/chats/{chat_id}/edit", func(c flamego.Context, sess session.Session) {
		UpdateContactChat(c, sess)
	})
	f.Post("/security/invites/{id}/regenerate", func(c flamego.Context, sess session.Session) {
		RegenerateUserInvite(c, sess)
	})
	f.Post("/zk/{id}/comment/{comment_id}/edit", func(c flamego.Context, sess session.Session) {
		UpdateZettelComment(c, sess, nil, nil)
	})

	return f
}

func performFormPOST(
	t *testing.T,
	f *flamego.Flame,
	path string,
	form url.Values,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)

	return rec
}

func performMalformedFormPOST(
	t *testing.T,
	f *flamego.Flame,
	path string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("%"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)

	return rec
}

func assertRedirect(t *testing.T, rec *httptest.ResponseRecorder, wantLocation string) {
	t.Helper()

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	if got := rec.Header().Get("Location"); got != wantLocation {
		t.Fatalf("expected redirect %q, got %q", wantLocation, got)
	}
}

func assertFlash(t *testing.T, s *testSession, wantType FlashType, wantMessage string) {
	t.Helper()

	msg, ok := s.flash.(FlashMessage)
	if !ok {
		t.Fatalf("expected flash message, got %T", s.flash)
	}

	if msg.Type != wantType || msg.Message != wantMessage {
		t.Fatalf("unexpected flash message: %#v", msg)
	}
}

func assertNoFlash(t *testing.T, s *testSession) {
	t.Helper()

	if s.flash != nil {
		t.Fatalf("expected no flash message, got %#v", s.flash)
	}
}

func setAdminSession(s *testSession) {
	s.Set("user_id", uuid.NewString())
	s.Set("user_is_admin", true)
	s.Set("user_display_name", "Admin")
}

func TestParseLogTypeAllValues(t *testing.T) {
	tests := []struct {
		input string
		want  db.LogType
	}{
		{input: "general", want: db.LogGeneral},
		{input: "email_sent", want: db.LogEmailSent},
		{input: "email_received", want: db.LogEmailReceived},
		{input: "call", want: db.LogCall},
		{input: "meeting", want: db.LogMeeting},
		{input: "message", want: db.LogMessage},
		{input: "gift_sent", want: db.LogGiftSent},
		{input: "gift_received", want: db.LogGiftReceived},
		{input: "intro", want: db.LogIntro},
		{input: "other", want: db.LogOther},
		{input: "  EMAIL_SENT  ", want: db.LogEmailSent},
		{input: "unknown", want: db.LogGeneral},
		{input: "", want: db.LogGeneral},
	}

	for _, tt := range tests {
		if got := parseLogType(tt.input); got != tt.want {
			t.Fatalf("parseLogType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestUpdateLogParseFormError(t *testing.T) {
	s := newTestSession()
	f := newMutatingHandlersTestApp(s)

	rec := performMalformedFormPOST(
		t,
		f,
		"/contact/contact-1/log/log-1/edit",
	)

	assertRedirect(t, rec, "/contact/contact-1")
	assertFlash(t, s, FlashError, "Failed to parse form")
}

func TestUpdateLogDatabaseFailure(t *testing.T) {
	originalUpdateLogDBFn := updateLogDBFn
	updateLogDBFn = func(context.Context, db.UpdateLogInput) error {
		return errTestBoom
	}

	t.Cleanup(func() {
		updateLogDBFn = originalUpdateLogDBFn
	})

	s := newTestSession()
	f := newMutatingHandlersTestApp(s)
	rec := performFormPOST(
		t,
		f,
		"/contact/contact-1/log/log-1/edit",
		url.Values{
			"log_type": {"meeting"},
			"content":  {"Met at conference"},
		},
		nil,
	)

	assertRedirect(t, rec, "/contact/contact-1")
	assertFlash(t, s, FlashError, "Failed to update log")
}

func TestUpdateLogSuccessPassesSanitizedInput(t *testing.T) {
	originalUpdateLogDBFn := updateLogDBFn

	var captured db.UpdateLogInput

	updateLogDBFn = func(_ context.Context, input db.UpdateLogInput) error {
		captured = input
		return nil
	}

	t.Cleanup(func() {
		updateLogDBFn = originalUpdateLogDBFn
	})

	s := newTestSession()
	f := newMutatingHandlersTestApp(s)
	rec := performFormPOST(
		t,
		f,
		"/contact/contact-1/log/log-1/edit",
		url.Values{
			"log_type":  {"  EMAIL_SENT  "},
			"logged_at": {"   "},
			"subject":   {"  Subject line  "},
			"content":   {"  Body text  "},
		},
		nil,
	)

	assertRedirect(t, rec, "/contact/contact-1")
	assertNoFlash(t, s)

	if captured.ID != "log-1" || captured.ContactID != "contact-1" {
		t.Fatalf("unexpected IDs in captured input: %#v", captured)
	}

	if captured.LogType != db.LogEmailSent {
		t.Fatalf("unexpected log type: %q", captured.LogType)
	}

	if captured.LoggedAt != nil {
		t.Fatalf("expected nil logged_at, got %#v", captured.LoggedAt)
	}

	if captured.Subject == nil || *captured.Subject != "Subject line" {
		t.Fatalf("unexpected subject: %#v", captured.Subject)
	}

	if captured.Content == nil || *captured.Content != "Body text" {
		t.Fatalf("unexpected content: %#v", captured.Content)
	}
}

func TestUpdateNoteParseFormError(t *testing.T) {
	s := newTestSession()
	f := newMutatingHandlersTestApp(s)
	rec := performMalformedFormPOST(
		t,
		f,
		"/contact/contact-1/note/note-1/edit",
	)

	assertRedirect(t, rec, "/contact/contact-1")
	assertFlash(t, s, FlashError, "Failed to parse form")
}

func TestUpdateNoteRejectsBlankContent(t *testing.T) {
	s := newTestSession()
	f := newMutatingHandlersTestApp(s)
	rec := performFormPOST(
		t,
		f,
		"/contact/contact-1/note/note-1/edit",
		url.Values{"content": {"   \t"}},
		nil,
	)

	assertRedirect(t, rec, "/contact/contact-1")
	assertFlash(t, s, FlashError, "Note content is required")
}

func TestUpdateNoteDatabaseFailure(t *testing.T) {
	originalUpdateNoteDBFn := updateNoteDBFn
	updateNoteDBFn = func(context.Context, db.UpdateNoteInput) error {
		return errTestBoom
	}

	t.Cleanup(func() {
		updateNoteDBFn = originalUpdateNoteDBFn
	})

	s := newTestSession()
	f := newMutatingHandlersTestApp(s)
	rec := performFormPOST(
		t,
		f,
		"/contact/contact-1/note/note-1/edit",
		url.Values{"content": {"Important note"}},
		nil,
	)

	assertRedirect(t, rec, "/contact/contact-1")
	assertFlash(t, s, FlashError, "Failed to update note")
}

func TestUpdateNoteSuccessPassesSanitizedInput(t *testing.T) {
	originalUpdateNoteDBFn := updateNoteDBFn

	var captured db.UpdateNoteInput

	updateNoteDBFn = func(_ context.Context, input db.UpdateNoteInput) error {
		captured = input
		return nil
	}

	t.Cleanup(func() {
		updateNoteDBFn = originalUpdateNoteDBFn
	})

	s := newTestSession()
	f := newMutatingHandlersTestApp(s)
	rec := performFormPOST(
		t,
		f,
		"/contact/contact-1/note/note-1/edit",
		url.Values{
			"content":  {"  Follow up next week  "},
			"noted_at": {"   "},
		},
		nil,
	)

	assertRedirect(t, rec, "/contact/contact-1")
	assertNoFlash(t, s)

	if captured.ID != "note-1" || captured.ContactID != "contact-1" {
		t.Fatalf("unexpected IDs in captured input: %#v", captured)
	}

	if captured.Content != "Follow up next week" {
		t.Fatalf("unexpected content: %q", captured.Content)
	}

	if captured.NotedAt != nil {
		t.Fatalf("expected nil noted_at, got %#v", captured.NotedAt)
	}
}

func TestUpdateContactChatRequiresSensitiveAccessBeforeDBLookup(t *testing.T) {
	originalIsServiceContactDBFn := isServiceContactDBFn
	called := false
	isServiceContactDBFn = func(context.Context, string) (bool, error) {
		called = true
		return false, nil
	}

	t.Cleanup(func() {
		isServiceContactDBFn = originalIsServiceContactDBFn
	})

	s := newTestSession()
	f := newMutatingHandlersTestApp(s)
	rec := performFormPOST(
		t,
		f,
		"/contact/contact-1/chats/chat-1/edit",
		url.Values{"message": {"hello"}},
		map[string]string{"Referer": "/contact/contact-1/chats"},
	)

	assertRedirect(t, rec, "/break-glass?next=%2Fcontact%2Fcontact-1%2Fchats")

	if called {
		t.Fatalf("expected service-contact lookup to be skipped while locked")
	}

	assertNoFlash(t, s)
}

func TestUpdateContactChatServiceLookupFailureSetsFlash(t *testing.T) {
	originalIsServiceContactDBFn := isServiceContactDBFn
	isServiceContactDBFn = func(context.Context, string) (bool, error) {
		return false, errTestLookupFailed
	}

	t.Cleanup(func() {
		isServiceContactDBFn = originalIsServiceContactDBFn
	})

	s := newTestSession()
	s.Set(sensitiveAccessSessionKey, time.Now().Unix())
	f := newMutatingHandlersTestApp(s)
	rec := performFormPOST(
		t,
		f,
		"/contact/contact-1/chats/chat-1/edit",
		url.Values{"message": {"hello"}},
		nil,
	)

	assertRedirect(t, rec, "/contact/contact-1/chats")
	assertFlash(t, s, FlashError, "Failed to load contact chat settings")
}

func TestUpdateContactChatBlocksServiceContacts(t *testing.T) {
	originalIsServiceContactDBFn := isServiceContactDBFn
	isServiceContactDBFn = func(context.Context, string) (bool, error) {
		return true, nil
	}

	t.Cleanup(func() {
		isServiceContactDBFn = originalIsServiceContactDBFn
	})

	s := newTestSession()
	s.Set(sensitiveAccessSessionKey, time.Now().Unix())
	f := newMutatingHandlersTestApp(s)
	rec := performFormPOST(
		t,
		f,
		"/contact/contact-1/chats/chat-1/edit",
		url.Values{"message": {"hello"}},
		nil,
	)

	assertRedirect(t, rec, "/contact/contact-1")
	assertFlash(t, s, FlashError, "Chats are not available for service contacts")
}

func TestUpdateContactChatParseFormError(t *testing.T) {
	originalIsServiceContactDBFn := isServiceContactDBFn
	isServiceContactDBFn = func(context.Context, string) (bool, error) {
		return false, nil
	}

	t.Cleanup(func() {
		isServiceContactDBFn = originalIsServiceContactDBFn
	})

	s := newTestSession()
	s.Set(sensitiveAccessSessionKey, time.Now().Unix())
	f := newMutatingHandlersTestApp(s)
	rec := performMalformedFormPOST(
		t,
		f,
		"/contact/contact-1/chats/chat-1/edit",
	)

	assertRedirect(t, rec, "/contact/contact-1/chats")
	assertFlash(t, s, FlashError, "Failed to parse form")
}

func TestUpdateContactChatRejectsBlankMessage(t *testing.T) {
	originalIsServiceContactDBFn := isServiceContactDBFn
	isServiceContactDBFn = func(context.Context, string) (bool, error) {
		return false, nil
	}

	t.Cleanup(func() {
		isServiceContactDBFn = originalIsServiceContactDBFn
	})

	s := newTestSession()
	s.Set(sensitiveAccessSessionKey, time.Now().Unix())
	f := newMutatingHandlersTestApp(s)
	rec := performFormPOST(
		t,
		f,
		"/contact/contact-1/chats/chat-1/edit",
		url.Values{"message": {"  \t  "}},
		nil,
	)

	assertRedirect(t, rec, "/contact/contact-1/chats")
	assertFlash(t, s, FlashError, "Message content is required")
}

func TestUpdateContactChatDatabaseFailure(t *testing.T) {
	originalIsServiceContactDBFn := isServiceContactDBFn
	originalUpdateChatDBFn := updateChatDBFn
	isServiceContactDBFn = func(context.Context, string) (bool, error) {
		return false, nil
	}
	updateChatDBFn = func(context.Context, db.UpdateChatInput) error {
		return errTestBoom
	}

	t.Cleanup(func() {
		isServiceContactDBFn = originalIsServiceContactDBFn
		updateChatDBFn = originalUpdateChatDBFn
	})

	s := newTestSession()
	s.Set(sensitiveAccessSessionKey, time.Now().Unix())
	f := newMutatingHandlersTestApp(s)
	rec := performFormPOST(
		t,
		f,
		"/contact/contact-1/chats/chat-1/edit",
		url.Values{"message": {"hello"}},
		nil,
	)

	assertRedirect(t, rec, "/contact/contact-1/chats")
	assertFlash(t, s, FlashError, "Failed to update chat entry")
}

func TestUpdateContactChatSuccessPassesSanitizedInput(t *testing.T) {
	originalIsServiceContactDBFn := isServiceContactDBFn
	originalUpdateChatDBFn := updateChatDBFn
	isServiceContactDBFn = func(context.Context, string) (bool, error) {
		return false, nil
	}

	var captured db.UpdateChatInput

	updateChatDBFn = func(_ context.Context, input db.UpdateChatInput) error {
		captured = input
		return nil
	}

	t.Cleanup(func() {
		isServiceContactDBFn = originalIsServiceContactDBFn
		updateChatDBFn = originalUpdateChatDBFn
	})

	s := newTestSession()
	s.Set(sensitiveAccessSessionKey, time.Now().Unix())
	f := newMutatingHandlersTestApp(s)
	rec := performFormPOST(
		t,
		f,
		"/contact/contact-1/chats/chat-1/edit",
		url.Values{
			"platform": {"  sLaCk  "},
			"sender":   {"  MIX  "},
			"message":  {"  hi there  "},
			"sent_at":  {"   "},
		},
		nil,
	)

	assertRedirect(t, rec, "/contact/contact-1/chats")
	assertNoFlash(t, s)

	if captured.ID != "chat-1" || captured.ContactID != "contact-1" {
		t.Fatalf("unexpected IDs in captured input: %#v", captured)
	}

	if captured.Platform != db.ChatPlatformSlack {
		t.Fatalf("unexpected platform: %q", captured.Platform)
	}

	if captured.Sender != db.ChatSenderMix {
		t.Fatalf("unexpected sender: %q", captured.Sender)
	}

	if captured.Message != "hi there" {
		t.Fatalf("unexpected message: %q", captured.Message)
	}

	if captured.SentAt != nil {
		t.Fatalf("expected nil sent_at, got %#v", captured.SentAt)
	}
}

func TestRegenerateUserInviteAccessRestricted(t *testing.T) {
	s := newTestSession()
	f := newMutatingHandlersTestApp(s)
	rec := performFormPOST(
		t,
		f,
		"/security/invites/invite-1/regenerate",
		url.Values{},
		nil,
	)

	assertRedirect(t, rec, "/security")
	assertFlash(t, s, FlashError, "Access restricted")
}

func TestRegenerateUserInviteRejectsBlankID(t *testing.T) {
	originalRegenerateExpiredUserInviteDBFn := regenerateExpiredUserInviteDBFn
	regenerateExpiredUserInviteDBFn = func(context.Context, string) (*db.UserInvite, error) {
		return nil, errTestShouldNotBeCalled
	}

	t.Cleanup(func() {
		regenerateExpiredUserInviteDBFn = originalRegenerateExpiredUserInviteDBFn
	})

	s := newTestSession()
	setAdminSession(s)
	f := newMutatingHandlersTestApp(s)
	rec := performFormPOST(
		t,
		f,
		"/security/invites/%20/regenerate",
		url.Values{},
		nil,
	)

	assertRedirect(t, rec, "/security")
	assertFlash(t, s, FlashError, "Missing invite ID")
}

func TestRegenerateUserInviteNotExpiredWarning(t *testing.T) {
	originalRegenerateExpiredUserInviteDBFn := regenerateExpiredUserInviteDBFn

	var capturedInviteID string

	regenerateExpiredUserInviteDBFn = func(_ context.Context, inviteID string) (*db.UserInvite, error) {
		capturedInviteID = inviteID
		return nil, db.ErrInviteNotExpired
	}

	t.Cleanup(func() {
		regenerateExpiredUserInviteDBFn = originalRegenerateExpiredUserInviteDBFn
	})

	s := newTestSession()
	setAdminSession(s)
	f := newMutatingHandlersTestApp(s)
	rec := performFormPOST(
		t,
		f,
		"/security/invites/%20invite-1%20/regenerate",
		url.Values{},
		nil,
	)

	assertRedirect(t, rec, "/security")
	assertFlash(t, s, FlashWarning, "Invite has not expired yet")

	if capturedInviteID != "invite-1" {
		t.Fatalf("expected trimmed invite ID, got %q", capturedInviteID)
	}
}

func TestRegenerateUserInviteDatabaseFailure(t *testing.T) {
	originalRegenerateExpiredUserInviteDBFn := regenerateExpiredUserInviteDBFn
	regenerateExpiredUserInviteDBFn = func(context.Context, string) (*db.UserInvite, error) {
		return nil, errTestBoom
	}

	t.Cleanup(func() {
		regenerateExpiredUserInviteDBFn = originalRegenerateExpiredUserInviteDBFn
	})

	s := newTestSession()
	setAdminSession(s)
	f := newMutatingHandlersTestApp(s)
	rec := performFormPOST(
		t,
		f,
		"/security/invites/invite-1/regenerate",
		url.Values{},
		nil,
	)

	assertRedirect(t, rec, "/security")
	assertFlash(t, s, FlashError, "Failed to regenerate invite")
}

func TestRegenerateUserInviteSuccess(t *testing.T) {
	originalRegenerateExpiredUserInviteDBFn := regenerateExpiredUserInviteDBFn
	regenerateExpiredUserInviteDBFn = func(context.Context, string) (*db.UserInvite, error) {
		return &db.UserInvite{}, nil
	}

	t.Cleanup(func() {
		regenerateExpiredUserInviteDBFn = originalRegenerateExpiredUserInviteDBFn
	})

	s := newTestSession()
	setAdminSession(s)
	f := newMutatingHandlersTestApp(s)
	rec := performFormPOST(
		t,
		f,
		"/security/invites/invite-1/regenerate",
		url.Values{},
		nil,
	)

	assertRedirect(t, rec, "/security")
	assertFlash(t, s, FlashSuccess, "Invite link regenerated")
}

func TestUpdateZettelCommentParseFormError(t *testing.T) {
	s := newTestSession()
	f := newMutatingHandlersTestApp(s)
	commentID := uuid.NewString()
	rec := performMalformedFormPOST(
		t,
		f,
		"/zk/id:abc/comment/"+commentID+"/edit",
	)

	assertRedirect(t, rec, "/zk/abc")
	assertFlash(t, s, FlashError, "Failed to update comment")
}

func TestUpdateZettelCommentInvalidCommentIDInboxRedirect(t *testing.T) {
	s := newTestSession()
	f := newMutatingHandlersTestApp(s)
	rec := performFormPOST(
		t,
		f,
		"/zk/id:abc/comment/not-a-uuid/edit",
		url.Values{
			"redirect_to": {"inbox"},
			"content":     {"Hello"},
		},
		nil,
	)

	assertRedirect(t, rec, "/zettel-inbox")
	assertFlash(t, s, FlashError, "Invalid comment ID")
}

func TestUpdateZettelCommentRejectsBlankContent(t *testing.T) {
	s := newTestSession()
	f := newMutatingHandlersTestApp(s)
	commentID := uuid.NewString()
	rec := performFormPOST(
		t,
		f,
		"/zk/id:abc/comment/"+commentID+"/edit",
		url.Values{
			"redirect_to": {"inbox"},
			"content":     {"  \t"},
		},
		nil,
	)

	assertRedirect(t, rec, "/zettel-inbox")
	assertFlash(t, s, FlashError, "Comment content is required")
}

func TestUpdateZettelCommentDatabaseFailure(t *testing.T) {
	originalUpdateZettelCommentDBFn := updateZettelCommentDBFn
	updateZettelCommentDBFn = func(context.Context, string, uuid.UUID, string) error {
		return errTestBoom
	}

	t.Cleanup(func() {
		updateZettelCommentDBFn = originalUpdateZettelCommentDBFn
	})

	s := newTestSession()
	f := newMutatingHandlersTestApp(s)
	commentID := uuid.NewString()
	rec := performFormPOST(
		t,
		f,
		"/zk/id:abc/comment/"+commentID+"/edit",
		url.Values{"content": {"hello"}},
		nil,
	)

	assertRedirect(t, rec, "/zk/abc")
	assertFlash(t, s, FlashError, "Failed to update comment")
}

func TestUpdateZettelCommentSuccessPassesScopedIDs(t *testing.T) {
	originalUpdateZettelCommentDBFn := updateZettelCommentDBFn

	var (
		capturedZettelID  string
		capturedCommentID uuid.UUID
		capturedContent   string
	)

	updateZettelCommentDBFn = func(_ context.Context, zettelID string, commentID uuid.UUID, content string) error {
		capturedZettelID = zettelID
		capturedCommentID = commentID
		capturedContent = content

		return nil
	}

	t.Cleanup(func() {
		updateZettelCommentDBFn = originalUpdateZettelCommentDBFn
	})

	s := newTestSession()
	f := newMutatingHandlersTestApp(s)
	commentID := uuid.NewString()
	rec := performFormPOST(
		t,
		f,
		"/zk/id:abc/comment/"+commentID+"/edit",
		url.Values{"content": {"  updated content  "}},
		nil,
	)

	assertRedirect(t, rec, "/zk/abc")
	assertFlash(t, s, FlashSuccess, "Comment updated successfully")

	if capturedZettelID != "abc" {
		t.Fatalf("expected zettel ID 'abc', got %q", capturedZettelID)
	}

	if capturedCommentID.String() != commentID {
		t.Fatalf("unexpected captured comment ID: %s", capturedCommentID)
	}

	if capturedContent != "updated content" {
		t.Fatalf("unexpected captured content: %q", capturedContent)
	}
}

func TestUpdateZettelCommentSuccessInboxRedirect(t *testing.T) {
	originalUpdateZettelCommentDBFn := updateZettelCommentDBFn
	updateZettelCommentDBFn = func(context.Context, string, uuid.UUID, string) error {
		return nil
	}

	t.Cleanup(func() {
		updateZettelCommentDBFn = originalUpdateZettelCommentDBFn
	})

	s := newTestSession()
	f := newMutatingHandlersTestApp(s)
	commentID := uuid.NewString()
	rec := performFormPOST(
		t,
		f,
		"/zk/id:abc/comment/"+commentID+"/edit",
		url.Values{
			"redirect_to": {"inbox"},
			"content":     {"hello"},
		},
		nil,
	)

	assertRedirect(t, rec, "/zettel-inbox")
	assertFlash(t, s, FlashSuccess, "Comment updated successfully")
}
