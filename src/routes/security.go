/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"
	"github.com/skip2/go-qrcode"

	"github.com/humaidq/groundwave/db"
)

// SessionMetadataMiddleware captures and stores device and IP info in the session
func SessionMetadataMiddleware() flamego.Handler {
	return func(c flamego.Context, s session.Session) {
		deviceLabel := parseUserAgent(c.Request().Header.Get("User-Agent"))
		if val, ok := s.Get("device_label").(string); !ok || val == "" || val != deviceLabel {
			s.Set("device_label", deviceLabel)
		}

		ip := getClientIP(c.Request())
		if val, ok := s.Get("device_ip").(string); !ok || val == "" || val != ip {
			s.Set("device_ip", ip)
		}

		c.Next()
	}
}

// parseUserAgent creates a simple device label from User-Agent string
func parseUserAgent(ua string) string {
	if ua == "" {
		return "Unknown device"
	}

	ua = strings.ToLower(ua)
	os := "Unknown OS"
	browser := "Unknown browser"

	// Simple OS detection
	if strings.Contains(ua, "android") {
		os = "Android"
	} else if strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") || strings.Contains(ua, "ios") {
		os = "iOS"
	} else if strings.Contains(ua, "windows") {
		os = "Windows"
	} else if strings.Contains(ua, "macintosh") || strings.Contains(ua, "mac os") {
		os = "macOS"
	} else if strings.Contains(ua, "linux") {
		os = "Linux"
	}

	// Simple browser detection
	if strings.Contains(ua, "edg/") {
		browser = "Edge"
	} else if strings.Contains(ua, "chrome") && !strings.Contains(ua, "edg/") {
		browser = "Chrome"
	} else if strings.Contains(ua, "firefox") {
		browser = "Firefox"
	} else if strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome") {
		browser = "Safari"
	}

	return os + " / " + browser
}

// getClientIP extracts the real client IP address
func getClientIP(r *flamego.Request) string {
	// Check X-Forwarded-For header (first entry is the client)
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		first := xff
		if idx := strings.Index(xff, ","); idx != -1 {
			first = xff[:idx]
		}
		if first = strings.TrimSpace(first); first != "" {
			return first
		}
	}

	// Check X-Real-IP header
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		return xri
	}

	// Fallback to RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// formatDuration creates a human-readable duration like "in 5d 10m"
func formatDuration(d time.Duration) string {
	if d < 0 {
		return "expired"
	}

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if days > 0 {
		if hours > 0 {
			parts = append(parts, fmt.Sprintf("%dh", hours))
		} else if minutes > 0 {
			parts = append(parts, fmt.Sprintf("%dm", minutes))
		}
	} else if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
		if minutes > 0 {
			parts = append(parts, fmt.Sprintf("%dm", minutes))
		}
	} else {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}

	if len(parts) == 0 {
		return "in 0m"
	}
	if len(parts) == 1 {
		return "in " + parts[0]
	}
	return "in " + parts[0] + " " + parts[1]
}

func buildExternalURL(r *flamego.Request, path string) string {
	scheme := "http"
	if proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		scheme = strings.TrimSpace(strings.Split(proto, ",")[0])
	} else if r.TLS != nil {
		scheme = "https"
	}

	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	if host == "" {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return scheme + "://" + host + path
}

// SessionInfo represents a session for the security page
type SessionInfo struct {
	ID        string
	UserID    string
	UserName  string
	ExpiresAt time.Time
	ExpiresIn string
	Device    string
	IP        string
	IsCurrent bool
}

// PasskeyInfo represents a passkey entry on the security page.
type PasskeyInfo struct {
	ID        string
	Label     string
	CreatedAt time.Time
	LastUsed  *time.Time
}

// InviteInfo represents a provisioning invite.
type InviteInfo struct {
	ID          string
	DisplayName string
	CreatedAt   time.Time
	SetupURL    string
	QRCode      string
}

// HealthProfileOption represents a health profile for sharing.
type HealthProfileOption struct {
	ID        string
	Name      string
	IsPrimary bool
}

// UserShareInfo represents a user with shared health profile IDs.
type UserShareInfo struct {
	ID               string
	DisplayName      string
	SharedProfileIDs map[string]bool
}

// Security renders the security page listing valid authenticated sessions
func Security(c flamego.Context, s session.Session, store session.Store, t template.Template, data template.Data) {
	userID, ok := getSessionUserID(s)
	if !ok {
		data["Error"] = "Unable to resolve current user"
		t.HTML(http.StatusInternalServerError, "security")
		return
	}

	ctx := c.Request().Context()
	var err error
	isAdmin := false
	if admin, err := resolveSessionIsAdmin(ctx, s); err == nil {
		isAdmin = admin
		data["IsAdmin"] = admin
	} else {
		log.Printf("Failed to resolve admin state: %v", err)
	}

	var users []db.User
	userNameByID := make(map[string]string)
	if isAdmin {
		users, err = db.ListUsers(ctx)
		if err != nil {
			log.Printf("Failed to load users: %v", err)
			data["ShareError"] = "Failed to load user list"
		} else {
			for _, user := range users {
				userNameByID[user.ID.String()] = user.DisplayName
			}
		}
	}

	// Type assert to our Postgres store to access ListValidSessions
	postgresStore, ok := store.(*db.PostgresSessionStore)
	if !ok {
		data["Error"] = "Unable to access session information"
		t.HTML(http.StatusInternalServerError, "security")
		return
	}

	// Get current session ID
	currentSessionID := s.ID()

	// List all valid sessions
	sessions, err := postgresStore.ListValidSessions(ctx)
	if err != nil {
		data["Error"] = "Failed to load session information"
		t.HTML(http.StatusInternalServerError, "security")
		return
	}

	// Convert to view models and mark current session
	var sessionInfos []SessionInfo
	for _, sess := range sessions {
		if !isAdmin {
			if sess.UserID == "" || sess.UserID != userID {
				continue
			}
		}
		isCurrent := sess.ID == currentSessionID
		expiresIn := formatDuration(time.Until(sess.ExpiresAt))
		userName := strings.TrimSpace(sess.UserDisplay)
		if userName == "" && sess.UserID != "" {
			if name, ok := userNameByID[sess.UserID]; ok {
				userName = name
			}
		}
		if userName == "" {
			userName = "Unknown user"
		}

		sessionInfos = append(sessionInfos, SessionInfo{
			ID:        sess.ID,
			UserID:    sess.UserID,
			UserName:  userName,
			ExpiresAt: sess.ExpiresAt,
			ExpiresIn: expiresIn,
			Device:    sess.DeviceLabel,
			IP:        sess.DeviceIP,
			IsCurrent: isCurrent,
		})
	}

	passkeys, err := db.ListUserPasskeys(ctx, userID)
	if err != nil {
		data["Error"] = "Failed to load passkey information"
		t.HTML(http.StatusInternalServerError, "security")
		return
	}

	passkeyInfos := make([]PasskeyInfo, 0, len(passkeys))
	for i, passkey := range passkeys {
		label := fmt.Sprintf("Passkey %d", i+1)
		if passkey.Label != nil && strings.TrimSpace(*passkey.Label) != "" {
			label = strings.TrimSpace(*passkey.Label)
		}
		passkeyInfos = append(passkeyInfos, PasskeyInfo{
			ID:        passkey.ID.String(),
			Label:     label,
			CreatedAt: passkey.CreatedAt,
			LastUsed:  passkey.LastUsedAt,
		})
	}

	data["Sessions"] = sessionInfos
	data["Passkeys"] = passkeyInfos
	data["PasskeyCount"] = len(passkeyInfos)

	if isAdmin {
		invites, err := db.ListPendingUserInvites(ctx)
		if err != nil {
			log.Printf("Failed to load invites: %v", err)
			data["InviteError"] = "Failed to load user invites"
		} else {
			inviteInfos := make([]InviteInfo, 0, len(invites))
			baseSetupURL := buildExternalURL(c.Request(), "/setup")
			for _, invite := range invites {
				displayName := "New user"
				if invite.DisplayName != nil && strings.TrimSpace(*invite.DisplayName) != "" {
					displayName = strings.TrimSpace(*invite.DisplayName)
				}
				setupURL := baseSetupURL + "?token=" + url.QueryEscape(invite.Token)
				qrCode := ""
				if png, err := qrcode.Encode(setupURL, qrcode.Medium, 256); err == nil {
					qrCode = base64.StdEncoding.EncodeToString(png)
				} else {
					log.Printf("Failed to generate invite QR code: %v", err)
				}
				inviteInfos = append(inviteInfos, InviteInfo{
					ID:          invite.ID.String(),
					DisplayName: displayName,
					CreatedAt:   invite.CreatedAt,
					SetupURL:    setupURL,
					QRCode:      qrCode,
				})
			}
			data["UserInvites"] = inviteInfos
		}

		if err == nil {
			profiles, err := db.ListHealthProfiles(ctx)
			if err != nil {
				log.Printf("Failed to load health profiles: %v", err)
				data["ShareError"] = "Failed to load health profiles"
			} else {
				shareRows, err := db.ListHealthProfileShares(ctx)
				if err != nil {
					log.Printf("Failed to load health shares: %v", err)
					data["ShareError"] = "Failed to load health shares"
				} else {
					shareMap := make(map[string]map[string]bool)
					for _, share := range shareRows {
						userKey := share.UserID.String()
						profileKey := share.ProfileID.String()
						if shareMap[userKey] == nil {
							shareMap[userKey] = make(map[string]bool)
						}
						shareMap[userKey][profileKey] = true
					}

					profileOptions := make([]HealthProfileOption, 0, len(profiles))
					for _, profile := range profiles {
						profileOptions = append(profileOptions, HealthProfileOption{
							ID:        profile.ID.String(),
							Name:      profile.Name,
							IsPrimary: profile.IsPrimary,
						})
					}

					userShares := make([]UserShareInfo, 0)
					for _, user := range users {
						if user.IsAdmin {
							continue
						}
						userKey := user.ID.String()
						sharedProfiles := shareMap[userKey]
						if sharedProfiles == nil {
							sharedProfiles = make(map[string]bool)
						}
						userShares = append(userShares, UserShareInfo{
							ID:               userKey,
							DisplayName:      user.DisplayName,
							SharedProfileIDs: sharedProfiles,
						})
					}

					data["ShareProfiles"] = profileOptions
					data["UserShares"] = userShares
				}
			}
		}
	}

	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Security", URL: "", IsCurrent: true},
	}
	data["PageTitle"] = "Security"

	t.HTML(http.StatusOK, "security")
}

// InvalidateOtherSessions logs out all other authenticated sessions.
func InvalidateOtherSessions(c flamego.Context, s session.Session, store session.Store) {
	postgresStore, ok := store.(*db.PostgresSessionStore)
	if !ok {
		SetErrorFlash(s, "Unable to access session information")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	userID, ok := getSessionUserID(s)
	if !ok {
		SetErrorFlash(s, "Unable to resolve current user")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	filterUserID := userID
	if isAdmin, err := resolveSessionIsAdmin(c.Request().Context(), s); err == nil && isAdmin {
		filterUserID = ""
	}

	deleted, err := postgresStore.InvalidateOtherSessions(c.Request().Context(), s.ID(), filterUserID)
	if err != nil {
		SetErrorFlash(s, "Failed to invalidate other sessions")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	if deleted == 0 {
		SetInfoFlash(s, "No other sessions to invalidate")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	SetSuccessFlash(s, fmt.Sprintf("Invalidated %d other session(s)", deleted))
	c.Redirect("/security", http.StatusSeeOther)
}

// InvalidateSession logs out a specific session.
func InvalidateSession(c flamego.Context, s session.Session, store session.Store) {
	postgresStore, ok := store.(*db.PostgresSessionStore)
	if !ok {
		SetErrorFlash(s, "Unable to access session information")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	sessionID := c.Param("id")
	if sessionID == "" {
		SetErrorFlash(s, "Missing session ID")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	userID, ok := getSessionUserID(s)
	if !ok {
		SetErrorFlash(s, "Unable to resolve current user")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	isAdmin, err := resolveSessionIsAdmin(c.Request().Context(), s)
	if err != nil {
		isAdmin = false
	}

	sessions, err := postgresStore.ListValidSessions(c.Request().Context())
	if err != nil {
		SetErrorFlash(s, "Failed to load session information")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	var target *db.SessionData
	for _, sess := range sessions {
		if sess.ID == sessionID {
			target = &sess
			break
		}
	}

	if target == nil {
		SetErrorFlash(s, "Session not found")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	if !isAdmin {
		if target.UserID == "" || target.UserID != userID {
			SetErrorFlash(s, "Access restricted")
			c.Redirect("/security", http.StatusSeeOther)
			return
		}
	}

	if err := postgresStore.Destroy(c.Request().Context(), sessionID); err != nil {
		SetErrorFlash(s, "Failed to invalidate session")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	if sessionID == s.ID() {
		Logout(s, c)
		return
	}

	SetSuccessFlash(s, "Session invalidated")
	c.Redirect("/security", http.StatusSeeOther)
}

// DeletePasskey removes a passkey for the current user.
func DeletePasskey(c flamego.Context, s session.Session) {
	userID, ok := getSessionUserID(s)
	if !ok {
		SetErrorFlash(s, "Unable to resolve current user")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	count, err := db.CountUserPasskeys(c.Request().Context(), userID)
	if err != nil {
		SetErrorFlash(s, "Failed to load passkeys")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}
	if count <= 1 {
		SetWarningFlash(s, "You must keep at least one passkey")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	passkeyID := c.Param("id")
	if passkeyID == "" {
		SetErrorFlash(s, "Missing passkey ID")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	if err := db.DeleteUserPasskey(c.Request().Context(), userID, passkeyID); err != nil {
		SetErrorFlash(s, "Failed to delete passkey")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	SetSuccessFlash(s, "Passkey deleted")
	c.Redirect("/security", http.StatusSeeOther)
}

// CreateUserInvite generates a new invite token (admin only).
func CreateUserInvite(c flamego.Context, s session.Session) {
	ctx := c.Request().Context()
	isAdmin, err := resolveSessionIsAdmin(ctx, s)
	if err != nil || !isAdmin {
		SetErrorFlash(s, "Access restricted")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	if err := c.Request().ParseForm(); err != nil {
		SetErrorFlash(s, "Failed to parse form")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	userID, _ := getSessionUserID(s)
	displayName := strings.TrimSpace(c.Request().Form.Get("display_name"))

	if _, err := db.CreateUserInvite(ctx, userID, displayName); err != nil {
		SetErrorFlash(s, "Failed to create invite")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	SetSuccessFlash(s, "Invite created")
	c.Redirect("/security", http.StatusSeeOther)
}

// DeleteUserInvite revokes a pending invite (admin only).
func DeleteUserInvite(c flamego.Context, s session.Session) {
	ctx := c.Request().Context()
	isAdmin, err := resolveSessionIsAdmin(ctx, s)
	if err != nil || !isAdmin {
		SetErrorFlash(s, "Access restricted")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	inviteID := c.Param("id")
	if inviteID == "" {
		SetErrorFlash(s, "Missing invite ID")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	if err := db.DeleteUserInvite(ctx, inviteID); err != nil {
		SetErrorFlash(s, "Failed to revoke invite")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	SetSuccessFlash(s, "Invite revoked")
	c.Redirect("/security", http.StatusSeeOther)
}

// UpdateHealthProfileShares updates shared profiles for a user (admin only).
func UpdateHealthProfileShares(c flamego.Context, s session.Session) {
	ctx := c.Request().Context()
	isAdmin, err := resolveSessionIsAdmin(ctx, s)
	if err != nil || !isAdmin {
		SetErrorFlash(s, "Access restricted")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	if err := c.Request().ParseForm(); err != nil {
		SetErrorFlash(s, "Failed to parse form")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	userID := c.Param("id")
	if strings.TrimSpace(userID) == "" {
		SetErrorFlash(s, "Missing user ID")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	targetUser, err := db.GetUserByID(ctx, userID)
	if err != nil {
		SetErrorFlash(s, "User not found")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}
	if targetUser.IsAdmin {
		SetErrorFlash(s, "Cannot update shares for admin user")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	profileIDs := c.Request().Form["profile_id"]
	createdBy, _ := getSessionUserID(s)
	if err := db.SetHealthProfileShares(ctx, userID, profileIDs, createdBy); err != nil {
		SetErrorFlash(s, "Failed to update health shares")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	SetSuccessFlash(s, "Health access updated")
	c.Redirect("/security", http.StatusSeeOther)
}
