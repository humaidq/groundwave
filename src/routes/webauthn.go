/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"

	"github.com/humaidq/groundwave/db"
)

const (
	webauthnLoginSessionKey      = "webauthn_login"
	webauthnRegisterSessionKey   = "webauthn_register"
	webauthnSetupSessionKey      = "webauthn_setup"
	webauthnBreakGlassSessionKey = "webauthn_break_glass"

	webauthnSetupUserIDKey      = "webauthn_setup_user_id"
	webauthnSetupDisplayNameKey = "webauthn_setup_display_name"
	webauthnSetupLabelKey       = "webauthn_setup_label"

	webauthnRegisterUserIDKey = "webauthn_register_user_id"
	webauthnRegisterLabelKey  = "webauthn_register_label"

	webauthnBootstrapAllowedKey = "webauthn_bootstrap_allowed"
)

func init() {
	gob.Register(webauthn.SessionData{})
}

// NewWebAuthnFromEnv builds the WebAuthn configuration using environment variables.
func NewWebAuthnFromEnv() (*webauthn.WebAuthn, error) {
	rpID := strings.TrimSpace(os.Getenv("WEBAUTHN_RP_ID"))
	rpOrigins := splitEnvList(os.Getenv("WEBAUTHN_RP_ORIGINS"))
	rpName := strings.TrimSpace(os.Getenv("WEBAUTHN_RP_NAME"))
	if rpName == "" {
		rpName = "Groundwave"
	}

	if rpID == "" {
		return nil, fmt.Errorf("WEBAUTHN_RP_ID is required")
	}
	if len(rpOrigins) == 0 {
		return nil, fmt.Errorf("WEBAUTHN_RP_ORIGINS is required")
	}

	config := &webauthn.Config{
		RPID:                  rpID,
		RPDisplayName:         rpName,
		RPOrigins:             rpOrigins,
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			UserVerification: protocol.VerificationRequired,
		},
	}

	return webauthn.New(config)
}

// SetupForm renders the admin bootstrap screen.
func SetupForm(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	data["HeaderOnly"] = true

	count, err := db.CountUsers(c.Request().Context())
	if err != nil {
		data["Error"] = "Failed to load authentication state"
		t.HTML(http.StatusInternalServerError, "setup")
		return
	}

	if count > 0 {
		s.Delete(webauthnBootstrapAllowedKey)
		SetInfoFlash(s, "Setup already completed")
		c.Redirect("/login", http.StatusSeeOther)
		return
	}

	bootstrapToken := strings.TrimSpace(os.Getenv("BOOTSTRAP_TOKEN"))
	if bootstrapToken == "" {
		s.Delete(webauthnBootstrapAllowedKey)
		data["Error"] = "BOOTSTRAP_TOKEN is not configured"
		t.HTML(http.StatusForbidden, "setup")
		return
	}

	if token := strings.TrimSpace(c.Query("token")); token == "" || token != bootstrapToken {
		s.Delete(webauthnBootstrapAllowedKey)
		data["Error"] = "Invalid setup link"
		t.HTML(http.StatusForbidden, "setup")
		return
	}

	s.Set(webauthnBootstrapAllowedKey, true)
	data["BootstrapReady"] = true
	data["DisplayName"] = "Admin"

	t.HTML(http.StatusOK, "setup")
}

// SetupStart begins WebAuthn registration for the first admin.
func SetupStart(c flamego.Context, s session.Session, web *webauthn.WebAuthn) {
	if !isBootstrapAllowed(s) {
		writeJSONError(c, http.StatusForbidden, "setup not permitted")
		return
	}

	count, err := db.CountUsers(c.Request().Context())
	if err != nil {
		writeJSONError(c, http.StatusInternalServerError, "failed to load authentication state")
		return
	}
	if count > 0 {
		writeJSONError(c, http.StatusBadRequest, "setup already completed")
		return
	}

	bootstrapToken := strings.TrimSpace(os.Getenv("BOOTSTRAP_TOKEN"))
	if bootstrapToken == "" {
		writeJSONError(c, http.StatusForbidden, "bootstrap token not configured")
		return
	}

	var request struct {
		DisplayName string `json:"displayName"`
		Label       string `json:"label"`
	}
	if err := json.NewDecoder(c.Request().Body().ReadCloser()).Decode(&request); err != nil {
		writeJSONError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	displayName := strings.TrimSpace(request.DisplayName)
	if displayName == "" {
		writeJSONError(c, http.StatusBadRequest, "display name is required")
		return
	}

	userID := uuid.New()
	label := strings.TrimSpace(request.Label)

	user := newWebAuthnUser(&db.User{
		ID:          userID,
		DisplayName: displayName,
		IsAdmin:     true,
	}, nil)

	options, sessionData, err := web.BeginRegistration(user,
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
	)
	if err != nil {
		writeJSONError(c, http.StatusInternalServerError, "failed to start registration")
		return
	}

	s.Set(webauthnSetupSessionKey, *sessionData)
	s.Set(webauthnSetupUserIDKey, userID.String())
	s.Set(webauthnSetupDisplayNameKey, displayName)
	s.Set(webauthnSetupLabelKey, label)

	writeJSON(c, options)
}

// SetupFinish completes WebAuthn registration for the first admin.
func SetupFinish(c flamego.Context, s session.Session, web *webauthn.WebAuthn) {
	if !isBootstrapAllowed(s) {
		writeJSONError(c, http.StatusForbidden, "setup not permitted")
		return
	}

	count, err := db.CountUsers(c.Request().Context())
	if err != nil {
		writeJSONError(c, http.StatusInternalServerError, "failed to load authentication state")
		return
	}
	if count > 0 {
		writeJSONError(c, http.StatusBadRequest, "setup already completed")
		return
	}

	setupSession, ok := getSessionData(s, webauthnSetupSessionKey)
	if !ok {
		writeJSONError(c, http.StatusBadRequest, "setup session missing")
		return
	}

	userID, displayName, label, err := getSetupSessionData(s)
	if err != nil {
		writeJSONError(c, http.StatusBadRequest, err.Error())
		return
	}

	user := newWebAuthnUser(&db.User{
		ID:          userID,
		DisplayName: displayName,
		IsAdmin:     true,
	}, nil)

	credential, err := web.FinishRegistration(user, *setupSession, c.Request().Request)
	if err != nil {
		writeJSONError(c, http.StatusBadRequest, "failed to finish registration")
		return
	}

	createdUser, err := db.CreateUser(c.Request().Context(), db.CreateUserInput{
		ID:          &userID,
		DisplayName: displayName,
		IsAdmin:     true,
	})
	if err != nil {
		writeJSONError(c, http.StatusInternalServerError, "failed to create user")
		return
	}

	var labelPtr *string
	if label != "" {
		labelPtr = &label
	}
	if _, err := db.AddUserPasskey(c.Request().Context(), createdUser.ID.String(), *credential, labelPtr); err != nil {
		_ = db.DeleteUser(c.Request().Context(), createdUser.ID.String())
		writeJSONError(c, http.StatusInternalServerError, "failed to save passkey")
		return
	}

	setAuthenticatedSession(s, createdUser)
	clearSetupSession(s)
	writeJSON(c, map[string]string{"redirect": "/"})
}

// PasskeyLoginStart begins a discoverable passkey login.
func PasskeyLoginStart(c flamego.Context, s session.Session, web *webauthn.WebAuthn) {
	options, sessionData, err := web.BeginDiscoverableLogin()
	if err != nil {
		writeJSONError(c, http.StatusInternalServerError, "failed to start login")
		return
	}

	s.Set(webauthnLoginSessionKey, *sessionData)

	writeJSON(c, options)
}

// PasskeyLoginFinish validates a passkey login.
func PasskeyLoginFinish(c flamego.Context, s session.Session, web *webauthn.WebAuthn) {
	loginSession, ok := getSessionData(s, webauthnLoginSessionKey)
	if !ok {
		writeJSONError(c, http.StatusBadRequest, "login session missing")
		return
	}

	user, credential, err := web.FinishPasskeyLogin(func(rawID, userHandle []byte) (webauthn.User, error) {
		return loadWebAuthnUserByHandle(c.Request().Context(), rawID, userHandle)
	}, *loginSession, c.Request().Request)
	if err != nil {
		writeJSONError(c, http.StatusUnauthorized, "failed to verify passkey")
		return
	}

	waUser, ok := user.(*webauthnUser)
	if !ok {
		writeJSONError(c, http.StatusInternalServerError, "unexpected user type")
		return
	}

	if err := db.UpdateUserPasskeyCredential(c.Request().Context(), waUser.user.ID.String(), *credential, time.Now()); err != nil {
		writeJSONError(c, http.StatusInternalServerError, "failed to update passkey")
		return
	}

	setAuthenticatedSession(s, waUser.user)
	s.Delete(webauthnLoginSessionKey)

	writeJSON(c, map[string]string{"redirect": "/"})
}

// PasskeyRegistrationStart begins registration for an additional passkey.
func PasskeyRegistrationStart(c flamego.Context, s session.Session, web *webauthn.WebAuthn) {
	userID, ok := getSessionUserID(s)
	if !ok {
		writeJSONError(c, http.StatusUnauthorized, "not authenticated")
		return
	}

	waUser, err := loadWebAuthnUser(c.Request().Context(), userID)
	if err != nil {
		writeJSONError(c, http.StatusInternalServerError, "failed to load user")
		return
	}

	var request struct {
		Label string `json:"label"`
	}
	if err := json.NewDecoder(c.Request().Body().ReadCloser()).Decode(&request); err != nil {
		writeJSONError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	label := strings.TrimSpace(request.Label)

	exclude := webauthn.Credentials(waUser.credentials).CredentialDescriptors()
	options, sessionData, err := web.BeginRegistration(waUser,
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		webauthn.WithExclusions(exclude),
	)
	if err != nil {
		writeJSONError(c, http.StatusInternalServerError, "failed to start registration")
		return
	}

	s.Set(webauthnRegisterSessionKey, *sessionData)
	s.Set(webauthnRegisterUserIDKey, waUser.user.ID.String())
	s.Set(webauthnRegisterLabelKey, label)

	writeJSON(c, options)
}

// PasskeyRegistrationFinish completes registration for an additional passkey.
func PasskeyRegistrationFinish(c flamego.Context, s session.Session, web *webauthn.WebAuthn) {
	registerSession, ok := getSessionData(s, webauthnRegisterSessionKey)
	if !ok {
		writeJSONError(c, http.StatusBadRequest, "registration session missing")
		return
	}

	userID, label, err := getRegisterSessionData(s)
	if err != nil {
		writeJSONError(c, http.StatusBadRequest, err.Error())
		return
	}

	waUser, err := loadWebAuthnUser(c.Request().Context(), userID)
	if err != nil {
		writeJSONError(c, http.StatusInternalServerError, "failed to load user")
		return
	}

	credential, err := web.FinishRegistration(waUser, *registerSession, c.Request().Request)
	if err != nil {
		writeJSONError(c, http.StatusBadRequest, "failed to finish registration")
		return
	}

	var labelPtr *string
	if label != "" {
		labelPtr = &label
	}
	if _, err := db.AddUserPasskey(c.Request().Context(), waUser.user.ID.String(), *credential, labelPtr); err != nil {
		writeJSONError(c, http.StatusInternalServerError, "failed to save passkey")
		return
	}

	clearRegisterSession(s)
	writeJSON(c, map[string]bool{"ok": true})
}

// BreakGlassStart begins WebAuthn verification for sensitive access.
func BreakGlassStart(c flamego.Context, s session.Session, web *webauthn.WebAuthn) {
	userID, ok := getSessionUserID(s)
	if !ok {
		writeJSONError(c, http.StatusUnauthorized, "not authenticated")
		return
	}

	waUser, err := loadWebAuthnUser(c.Request().Context(), userID)
	if err != nil {
		writeJSONError(c, http.StatusInternalServerError, "failed to load user")
		return
	}

	options, sessionData, err := web.BeginLogin(waUser)
	if err != nil {
		writeJSONError(c, http.StatusInternalServerError, "failed to start verification")
		return
	}

	s.Set(webauthnBreakGlassSessionKey, *sessionData)
	writeJSON(c, options)
}

// BreakGlassFinish completes WebAuthn verification for sensitive access.
func BreakGlassFinish(c flamego.Context, s session.Session, web *webauthn.WebAuthn) {
	breakSession, ok := getSessionData(s, webauthnBreakGlassSessionKey)
	if !ok {
		writeJSONError(c, http.StatusBadRequest, "verification session missing")
		return
	}

	userID, ok := getSessionUserID(s)
	if !ok {
		writeJSONError(c, http.StatusUnauthorized, "not authenticated")
		return
	}

	waUser, err := loadWebAuthnUser(c.Request().Context(), userID)
	if err != nil {
		writeJSONError(c, http.StatusInternalServerError, "failed to load user")
		return
	}

	credential, err := web.FinishLogin(waUser, *breakSession, c.Request().Request)
	if err != nil {
		writeJSONError(c, http.StatusUnauthorized, "failed to verify passkey")
		return
	}

	if err := db.UpdateUserPasskeyCredential(c.Request().Context(), waUser.user.ID.String(), *credential, time.Now()); err != nil {
		writeJSONError(c, http.StatusInternalServerError, "failed to update passkey")
		return
	}

	s.Set(sensitiveAccessSessionKey, time.Now().Unix())
	s.Delete(webauthnBreakGlassSessionKey)

	next := sanitizeNextPath(c.Query("next"))
	if next == "" {
		next = "/contacts"
	}

	writeJSON(c, map[string]string{"redirect": next})
}

func setAuthenticatedSession(s session.Session, user *db.User) {
	s.Set("authenticated", true)
	s.Set("user_id", user.ID.String())
	s.Set("user_display_name", user.DisplayName)
	s.Set("userID", user.ID.String())
}

func getSessionUserID(s session.Session) (string, bool) {
	if val := s.Get("user_id"); val != nil {
		if userID, ok := val.(string); ok && userID != "" {
			return userID, true
		}
	}
	return "", false
}

func getSessionData(s session.Session, key string) (*webauthn.SessionData, bool) {
	val := s.Get(key)
	if val == nil {
		return nil, false
	}
	if data, ok := val.(webauthn.SessionData); ok {
		return &data, true
	}
	if data, ok := val.(*webauthn.SessionData); ok && data != nil {
		return data, true
	}
	return nil, false
}

func getSetupSessionData(s session.Session) (uuid.UUID, string, string, error) {
	userIDRaw, ok := s.Get(webauthnSetupUserIDKey).(string)
	if !ok || userIDRaw == "" {
		return uuid.UUID{}, "", "", fmt.Errorf("setup user missing")
	}
	userID, err := uuid.Parse(userIDRaw)
	if err != nil {
		return uuid.UUID{}, "", "", fmt.Errorf("invalid setup user")
	}

	displayName, ok := s.Get(webauthnSetupDisplayNameKey).(string)
	if !ok || displayName == "" {
		return uuid.UUID{}, "", "", fmt.Errorf("display name missing")
	}

	label, _ := s.Get(webauthnSetupLabelKey).(string)
	return userID, displayName, strings.TrimSpace(label), nil
}

func getRegisterSessionData(s session.Session) (string, string, error) {
	userID, ok := s.Get(webauthnRegisterUserIDKey).(string)
	if !ok || userID == "" {
		return "", "", fmt.Errorf("registration user missing")
	}
	label, _ := s.Get(webauthnRegisterLabelKey).(string)
	return userID, strings.TrimSpace(label), nil
}

func clearSetupSession(s session.Session) {
	s.Delete(webauthnSetupSessionKey)
	s.Delete(webauthnSetupUserIDKey)
	s.Delete(webauthnSetupDisplayNameKey)
	s.Delete(webauthnSetupLabelKey)
	s.Delete(webauthnBootstrapAllowedKey)
}

func clearRegisterSession(s session.Session) {
	s.Delete(webauthnRegisterSessionKey)
	s.Delete(webauthnRegisterUserIDKey)
	s.Delete(webauthnRegisterLabelKey)
}

func isBootstrapAllowed(s session.Session) bool {
	allowed, ok := s.Get(webauthnBootstrapAllowedKey).(bool)
	return ok && allowed
}

type webauthnUser struct {
	user        *db.User
	credentials []webauthn.Credential
}

func newWebAuthnUser(user *db.User, credentials []webauthn.Credential) *webauthnUser {
	return &webauthnUser{
		user:        user,
		credentials: credentials,
	}
}

func (u *webauthnUser) WebAuthnID() []byte {
	return u.user.ID[:]
}

func (u *webauthnUser) WebAuthnName() string {
	return u.user.DisplayName
}

func (u *webauthnUser) WebAuthnDisplayName() string {
	return u.user.DisplayName
}

func (u *webauthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.credentials
}

func loadWebAuthnUser(ctx context.Context, userID string) (*webauthnUser, error) {
	user, err := db.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	credentials, err := db.LoadUserCredentials(ctx, user.ID.String())
	if err != nil {
		return nil, err
	}
	return newWebAuthnUser(user, credentials), nil
}

func loadWebAuthnUserByHandle(ctx context.Context, rawID, userHandle []byte) (*webauthnUser, error) {
	user, err := db.GetUserByWebAuthnID(ctx, userHandle)
	if err != nil {
		return nil, err
	}
	credentials, err := db.LoadUserCredentials(ctx, user.ID.String())
	if err != nil {
		return nil, err
	}
	return newWebAuthnUser(user, credentials), nil
}

func splitEnvList(raw string) []string {
	var values []string
	for _, item := range strings.Split(raw, ",") {
		if value := strings.TrimSpace(item); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func writeJSON(c flamego.Context, payload any) {
	c.ResponseWriter().Header().Set("Content-Type", "application/json")
	json.NewEncoder(c.ResponseWriter()).Encode(payload)
}

func writeJSONError(c flamego.Context, status int, message string) {
	c.ResponseWriter().Header().Set("Content-Type", "application/json")
	c.ResponseWriter().WriteHeader(status)
	json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": message})
}
