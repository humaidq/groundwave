/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package cmd

import (
	"context"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/flamego/csrf"
	"github.com/flamego/flamego"
	"github.com/flamego/session"
	flamegoTemplate "github.com/flamego/template"
	"github.com/urfave/cli/v3"

	"github.com/humaidq/groundwave/db"
	"github.com/humaidq/groundwave/routes"
	"github.com/humaidq/groundwave/static"
	"github.com/humaidq/groundwave/templates"
	"github.com/humaidq/groundwave/whatsapp"
)

var CmdStart = &cli.Command{
	Name:    "start",
	Aliases: []string{"run"},
	Usage:   "Start the web server",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "port",
			Value: "8080",
			Usage: "the web server port",
		},
		&cli.StringFlag{
			Name:    "database-url",
			Sources: cli.EnvVars("DATABASE_URL"),
			Usage:   "PostgreSQL connection string (e.g., postgres://user:pass@localhost/dbname)",
		},
		&cli.BoolFlag{
			Name:  "dev",
			Value: false,
			Usage: "enables development mode (for templates)",
		},
	},
	Action: start,
}

func start(ctx context.Context, cmd *cli.Command) (err error) {
	// Get database URL
	databaseURL := cmd.String("database-url")
	if databaseURL == "" {
		return fmt.Errorf("database-url is required (set via --database-url or DATABASE_URL env var)")
	}

	csrfSecret := os.Getenv("CSRF_SECRET")
	if csrfSecret == "" {
		return fmt.Errorf("CSRF_SECRET is required")
	}

	webAuthn, err := routes.NewWebAuthnFromEnv()
	if err != nil {
		return fmt.Errorf("failed to configure WebAuthn: %w", err)
	}

	// Set DATABASE_URL for db package
	if err := os.Setenv("DATABASE_URL", databaseURL); err != nil {
		return fmt.Errorf("failed to set DATABASE_URL: %w", err)
	}

	// Initialize database connection
	appLogger.Info("Connecting to database")
	if err := db.Init(ctx); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	// Sync schema
	appLogger.Info("Syncing database schema")
	if err := db.SyncSchema(ctx); err != nil {
		return fmt.Errorf("failed to sync schema: %w", err)
	}
	appLogger.Info("Database schema synced successfully")

	// Start cache rebuild worker
	db.StartRebuildCacheWorker(context.Background())

	// Sync CardDAV contacts
	appLogger.Info("Syncing contacts from CardDAV")
	if err := db.SyncAllCardDAVContacts(ctx); err != nil {
		appLogger.Warn("CardDAV sync failed", "error", err)
		// Don't fail startup, just log the error
	}

	// Initialize WhatsApp client (optional feature)
	whatsappLogger.Info("Initializing WhatsApp client")
	if err := whatsapp.Initialize(ctx, databaseURL, handleWhatsAppMessage); err != nil {
		whatsappLogger.Warn("WhatsApp initialization failed", "error", err)
		// Don't fail startup, WhatsApp is optional
	} else {
		whatsappLogger.Info("WhatsApp client initialized successfully")
	}

	// Create maps directory if it doesn't exist
	if err := os.MkdirAll("maps", 0755); err != nil {
		return fmt.Errorf("failed to create maps directory: %w", err)
	}

	f := flamego.New()
	f.Use(flamego.Recovery())
	f.Map(requestLogger)
	f.Map(requestStdLogger)
	f.Map(webAuthn)

	// Setup flamego
	fs, err := flamegoTemplate.EmbedFS(templates.Templates, ".", []string{".html"})
	if err != nil {
		panic(err)
	}

	// Custom template functions
	funcMap := template.FuncMap{
		"contains": func(slice []string, item string) bool {
			for _, s := range slice {
				if s == item {
					return true
				}
			}
			return false
		},
		"filterLabel": func(filter string) string {
			labels := map[string]string{
				"no_phone":    "No phone number",
				"no_email":    "No email address",
				"no_carddav":  "Not linked to CardDAV",
				"no_linkedin": "No LinkedIn",
			}
			if label, ok := labels[filter]; ok {
				return label
			}
			return filter
		},
		"inventoryStatusLabel": func(status db.InventoryStatus) string {
			return db.InventoryStatusLabel(status)
		},
		"formatFileSize": func(size int64) string {
			if size < 1000 {
				return fmt.Sprintf("%d bytes", size)
			}
			units := []string{"kB", "MB", "GB", "TB", "PB"}
			value := float64(size)
			unitIndex := -1
			for value >= 1000 && unitIndex < len(units)-1 {
				value /= 1000
				unitIndex++
			}
			formatted := strings.TrimSuffix(fmt.Sprintf("%.1f", value), ".0")
			return fmt.Sprintf("%s %s", formatted, units[unitIndex])
		},
		"truncateBreadcrumb": func(title string) string {
			const maxLength = 40
			if title == "" {
				return title
			}
			runes := []rune(title)
			if len(runes) <= maxLength {
				return title
			}
			return string(runes[:maxLength]) + "..."
		},
		"safeImageURL": func(raw *string) template.URL {
			if raw == nil {
				return ""
			}
			value := strings.TrimSpace(*raw)
			if value == "" {
				return ""
			}
			lower := strings.ToLower(value)
			if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "data:image/") {
				return template.URL(value)
			}
			return ""
		},
		"urlIconClass": func(urlType db.URLType) string {
			switch urlType {
			case db.URLWebsite:
				return "fa-solid fa-globe"
			case db.URLBlog:
				return "fa-solid fa-pen-nib"
			case db.URLTwitter:
				return "fa-brands fa-x-twitter"
			case db.URLMastodon:
				return "fa-brands fa-mastodon"
			case db.URLBluesky:
				return "fa-brands fa-bluesky"
			case db.URLThreads:
				return "fa-brands fa-threads"
			case db.URLFacebook:
				return "fa-brands fa-facebook"
			case db.URLInstagram:
				return "fa-brands fa-instagram"
			case db.URLLinkedIn:
				return "fa-brands fa-linkedin"
			case db.URLOrcid:
				return "fa-brands fa-orcid"
			case db.URLGoogleScholar:
				return "fa-solid fa-graduation-cap"
			case db.URLGitHub:
				return "fa-brands fa-github"
			case db.URLGitLab:
				return "fa-brands fa-gitlab"
			case db.URLCodeberg:
				return "fa-solid fa-code-branch"
			case db.URLYouTube:
				return "fa-brands fa-youtube"
			case db.URLTwitch:
				return "fa-brands fa-twitch"
			case db.URLTikTok:
				return "fa-brands fa-tiktok"
			case db.URLSignal:
				return "fa-brands fa-signal-messenger"
			case db.URLTelegram:
				return "fa-brands fa-telegram"
			case db.URLWhatsApp:
				return "fa-brands fa-whatsapp"
			case db.URLMatrix:
				return "fa-solid fa-hashtag"
			case db.URLQRZ:
				return "fa-solid fa-tower-broadcast"
			case db.URLOther:
				return "fa-solid fa-link"
			default:
				return "fa-solid fa-link"
			}
		},
		"urlIconLabel": func(urlType db.URLType) string {
			switch urlType {
			case db.URLWebsite:
				return "Website"
			case db.URLBlog:
				return "Blog"
			case db.URLTwitter:
				return "X (Twitter)"
			case db.URLMastodon:
				return "Mastodon"
			case db.URLBluesky:
				return "Bluesky"
			case db.URLThreads:
				return "Threads"
			case db.URLFacebook:
				return "Facebook"
			case db.URLInstagram:
				return "Instagram"
			case db.URLLinkedIn:
				return "LinkedIn"
			case db.URLOrcid:
				return "ORCID"
			case db.URLGoogleScholar:
				return "Google Scholar"
			case db.URLGitHub:
				return "GitHub"
			case db.URLGitLab:
				return "GitLab"
			case db.URLCodeberg:
				return "Codeberg"
			case db.URLYouTube:
				return "YouTube"
			case db.URLTwitch:
				return "Twitch"
			case db.URLTikTok:
				return "TikTok"
			case db.URLSignal:
				return "Signal"
			case db.URLTelegram:
				return "Telegram"
			case db.URLWhatsApp:
				return "WhatsApp"
			case db.URLMatrix:
				return "Matrix"
			case db.URLQRZ:
				return "QRZ"
			case db.URLOther:
				return "Link"
			default:
				return "Link"
			}
		},
		"formatAmount": func(value float64) string {
			sign := ""
			if value < 0 {
				sign = "-"
				value = math.Abs(value)
			}
			formatted := fmt.Sprintf("%.2f", value)
			parts := strings.SplitN(formatted, ".", 2)
			integer := parts[0]
			fraction := "00"
			if len(parts) == 2 {
				fraction = parts[1]
			}
			for i := len(integer) - 3; i > 0; i -= 3 {
				integer = integer[:i] + "," + integer[i:]
			}
			return sign + integer + "." + fraction
		},
	}
	// Configure PostgreSQL session store with 30-day expiry
	f.Use(session.Sessioner(session.Options{
		Initer: db.PostgresSessionIniter(),
		Config: db.PostgresSessionConfig{
			Lifetime:  30 * 24 * time.Hour, // 30 days
			TableName: "flamego_sessions",
		},
		Cookie: session.CookieOptions{
			MaxAge:   30 * 24 * 60 * 60, // 30 days in seconds
			HTTPOnly: true,
			SameSite: http.SameSiteLaxMode,
		},
	}))
	f.Use(routes.RequestLogger)
	f.Use(csrf.Csrfer(csrf.Options{
		Secret: csrfSecret,
	}))
	f.Use(flamegoTemplate.Templater(flamegoTemplate.Options{
		FileSystem: fs,
		FuncMaps:   []template.FuncMap{funcMap},
	}))
	f.Use(routes.CSRFInjector())
	f.Use(routes.UserContextInjector())
	f.Use(routes.NoCacheHeaders())
	// Session metadata middleware - captures device and IP info
	f.Use(routes.SessionMetadataMiddleware())
	// Flash message middleware - retrieve flash from session and pass to templates
	f.Use(func(data flamegoTemplate.Data, flash session.Flash) {
		if msg, ok := flash.(routes.FlashMessage); ok {
			data["Flash"] = msg
		}
	})
	f.Use(flamego.Static())
	f.Use(flamego.Static(flamego.StaticOptions{
		FileSystem: http.FS(static.Static),
	}))
	// Serve maps directory for grid square maps
	f.Use(flamego.Static(flamego.StaticOptions{
		Directory: "maps",
	}))

	// Public routes (no authentication required)
	f.Get("/security.txt", func(c flamego.Context) {
		c.Redirect("https://huma.id/.well-known/security.txt", http.StatusMovedPermanently)
	})
	f.Get("/.well-known/security.txt", func(c flamego.Context) {
		c.Redirect("https://huma.id/.well-known/security.txt", http.StatusMovedPermanently)
	})
	f.Get("/login", routes.LoginForm)
	f.Get("/setup", routes.SetupForm)
	f.Post("/webauthn/login/start", csrf.Validate, routes.PasskeyLoginStart)
	f.Post("/webauthn/login/finish", csrf.Validate, routes.PasskeyLoginFinish)
	f.Post("/webauthn/setup/start", csrf.Validate, routes.SetupStart)
	f.Post("/webauthn/setup/finish", csrf.Validate, routes.SetupFinish)
	f.Get("/connectivity", func(c flamego.Context) {
		if _, err := c.ResponseWriter().Write([]byte("1")); err != nil {
			appLogger.Error("Error writing connectivity response", "error", err)
		}
	})
	f.Get("/note/{id}", routes.ViewPublicNote)
	f.Get("/ext/auth", routes.RequireAuth, routes.RequireAdmin, routes.ExtensionAuth)
	f.Get("/ext/complete", routes.ExtensionComplete)
	f.Get("/ext/validate", routes.ExtensionValidate)
	f.Get("/ext/contacts-no-linkedin", routes.ExtensionContactsWithoutLinkedIn)
	f.Post("/ext/linkedin-lookup", routes.ExtensionLinkedInLookup)
	f.Post("/ext/linkedin-assign", routes.ExtensionLinkedInAssign)
	f.Options("/ext/validate", routes.ExtensionValidate)
	f.Options("/ext/contacts-no-linkedin", routes.ExtensionContactsWithoutLinkedIn)
	f.Options("/ext/linkedin-lookup", routes.ExtensionLinkedInLookup)
	f.Options("/ext/linkedin-assign", routes.ExtensionLinkedInAssign)

	// Protected routes (require authentication)
	f.Group("", func() {
		// Shared routes
		f.Get("/", routes.Welcome)
		f.Post("/logout", csrf.Validate, routes.Logout)
		f.Post("/sensitive-access/lock", csrf.Validate, routes.LockSensitiveAccess)
		f.Get("/break-glass", routes.BreakGlassForm)
		f.Post("/break-glass/start", csrf.Validate, routes.BreakGlassStart)
		f.Post("/break-glass/finish", csrf.Validate, routes.BreakGlassFinish)

		// Inventory read-only routes
		f.Get("/inventory", routes.InventoryList)
		f.Get("/inventory/{id}", routes.ViewInventoryItem)
		f.Get("/inventory/{id}/file/{filename}", routes.DownloadInventoryFile)

		// Files read-only routes
		f.Get("/files", routes.FilesList)
		f.Get("/files/view", routes.FilesView)
		f.Get("/files/file", routes.DownloadFilesFile)

		// Health read-only routes
		f.Get("/health/break-glass", routes.BreakGlassForm)
		f.Post("/health/break-glass/start", csrf.Validate, routes.BreakGlassStart)
		f.Post("/health/break-glass/finish", csrf.Validate, routes.BreakGlassFinish)
		f.Get("/health", routes.ListHealthProfiles)
		f.Get("/health/{id}", routes.ViewHealthProfile)
		f.Get("/health/{profile_id}/followup/{id}", routes.ViewFollowup)
		f.Post("/health/{profile_id}/followup/{id}/ai-summary", csrf.Validate, routes.GenerateAISummary)

		// Security & passkeys
		f.Get("/security", routes.Security)
		f.Post("/security/invalidate-other", csrf.Validate, routes.InvalidateOtherSessions)
		f.Post("/security/sessions/{id}/invalidate", csrf.Validate, routes.InvalidateSession)
		f.Post("/webauthn/passkey/start", csrf.Validate, routes.PasskeyRegistrationStart)
		f.Post("/webauthn/passkey/finish", csrf.Validate, routes.PasskeyRegistrationFinish)
		f.Post("/security/passkeys/{id}/delete", csrf.Validate, routes.DeletePasskey)
	}, routes.RequireAuth, routes.RequireSensitiveAccessForHealth)

	// Admin-only routes
	f.Group("", func() {
		f.Get("/todo", routes.Todo)
		f.Get("/timeline", routes.RequireSensitiveAccess, routes.Timeline)
		f.Get("/journal/{date}", routes.RequireSensitiveAccess, routes.ViewJournalEntry)
		f.Post("/journal/{date}/location", routes.RequireSensitiveAccess, csrf.Validate, routes.AddJournalLocation)
		f.Post("/journal/{date}/location/{location_id}/delete", routes.RequireSensitiveAccess, csrf.Validate, routes.DeleteJournalLocation)
		f.Get("/ledger", routes.RequireSensitiveAccess, routes.LedgerIndex)
		f.Get("/ledger/budgets/new", routes.RequireSensitiveAccess, routes.LedgerBudgetNewForm)
		f.Get("/ledger/budgets/{id}/edit", routes.RequireSensitiveAccess, routes.LedgerBudgetEditForm)
		f.Get("/ledger/accounts/new", routes.RequireSensitiveAccess, routes.LedgerAccountNewForm)
		f.Get("/ledger/accounts/{id}", routes.RequireSensitiveAccess, routes.LedgerAccountView)
		f.Get("/ledger/accounts/{id}/edit", routes.RequireSensitiveAccess, routes.LedgerAccountEditForm)
		f.Get("/ledger/accounts/{id}/transactions/new", routes.RequireSensitiveAccess, routes.LedgerTransactionNewForm)
		f.Get("/ledger/accounts/{id}/transactions/{tx_id}/edit", routes.RequireSensitiveAccess, routes.LedgerTransactionEditForm)
		f.Get("/ledger/accounts/{id}/reconcile/new", routes.RequireSensitiveAccess, routes.LedgerReconcileNewForm)
		f.Get("/ledger/accounts/{id}/reconciliations/{rec_id}/edit", routes.RequireSensitiveAccess, routes.LedgerReconciliationEditForm)
		f.Post("/ledger/budgets/new", routes.RequireSensitiveAccess, csrf.Validate, routes.CreateLedgerBudget)
		f.Post("/ledger/budgets/{id}/edit", routes.RequireSensitiveAccess, csrf.Validate, routes.UpdateLedgerBudget)
		f.Post("/ledger/budgets/{id}/delete", routes.RequireSensitiveAccess, csrf.Validate, routes.DeleteLedgerBudget)
		f.Post("/ledger/accounts/new", routes.RequireSensitiveAccess, csrf.Validate, routes.CreateLedgerAccount)
		f.Post("/ledger/accounts/{id}/edit", routes.RequireSensitiveAccess, csrf.Validate, routes.UpdateLedgerAccount)
		f.Post("/ledger/accounts/{id}/delete", routes.RequireSensitiveAccess, csrf.Validate, routes.DeleteLedgerAccount)
		f.Post("/ledger/accounts/{id}/transactions", routes.RequireSensitiveAccess, csrf.Validate, routes.CreateLedgerTransaction)
		f.Post("/ledger/accounts/{id}/transactions/{tx_id}/edit", routes.RequireSensitiveAccess, csrf.Validate, routes.UpdateLedgerTransaction)
		f.Post("/ledger/accounts/{id}/transactions/{tx_id}/delete", routes.RequireSensitiveAccess, csrf.Validate, routes.DeleteLedgerTransaction)
		f.Post("/ledger/accounts/{id}/reconcile", routes.RequireSensitiveAccess, csrf.Validate, routes.CreateLedgerReconciliation)
		f.Post("/ledger/accounts/{id}/reconciliations/{rec_id}/edit", routes.RequireSensitiveAccess, csrf.Validate, routes.UpdateLedgerReconciliation)
		f.Post("/ledger/accounts/{id}/reconciliations/{rec_id}/delete", routes.RequireSensitiveAccess, csrf.Validate, routes.DeleteLedgerReconciliation)
		f.Get("/contacts", routes.Home)
		f.Get("/overdue", routes.Overdue)
		f.Get("/qsl", routes.QSL)
		f.Get("/qsl/{id}", routes.ViewQSO)
		f.Post("/qsl/import", csrf.Validate, routes.ImportADIF)
		f.Get("/contact/new", routes.RequireSensitiveAccess, routes.NewContactForm)
		f.Post("/contact/new", routes.RequireSensitiveAccess, csrf.Validate, routes.CreateContact)
		f.Get("/contact/{id}", routes.ViewContact)
		f.Get("/contact/{id}/chats", routes.ViewContactChats)
		f.Post("/contact/{id}/chats", csrf.Validate, routes.AddContactChat)
		f.Post("/contact/{id}/chat-summary", csrf.Validate, routes.GenerateContactChatSummary)
		f.Get("/contact/{id}/edit", routes.RequireSensitiveAccess, routes.EditContactForm)
		f.Post("/contact/{id}/edit", routes.RequireSensitiveAccess, csrf.Validate, routes.UpdateContact)
		f.Post("/contact/{id}/email", csrf.Validate, routes.AddEmail)
		f.Post("/contact/{id}/phone", csrf.Validate, routes.AddPhone)
		f.Post("/contact/{id}/url", csrf.Validate, routes.AddURL)
		f.Post("/contact/{id}/email/{email_id}/delete", csrf.Validate, routes.DeleteEmail)
		f.Post("/contact/{id}/email/{email_id}/edit", csrf.Validate, routes.UpdateEmail)
		f.Post("/contact/{id}/phone/{phone_id}/delete", csrf.Validate, routes.DeletePhone)
		f.Post("/contact/{id}/phone/{phone_id}/edit", csrf.Validate, routes.UpdatePhone)
		f.Post("/contact/{id}/url/{url_id}/delete", csrf.Validate, routes.DeleteURL)
		f.Post("/contact/{id}/log", routes.RequireSensitiveAccess, csrf.Validate, routes.AddLog)
		f.Post("/contact/{id}/log/{log_id}/delete", routes.RequireSensitiveAccess, csrf.Validate, routes.DeleteLog)
		f.Post("/contact/{id}/note", routes.RequireSensitiveAccess, csrf.Validate, routes.AddNote)
		f.Post("/contact/{id}/note/{note_id}/delete", routes.RequireSensitiveAccess, csrf.Validate, routes.DeleteNote)
		f.Post("/contact/{id}/carddav/link", routes.RequireSensitiveAccess, csrf.Validate, routes.LinkCardDAV)
		f.Post("/contact/{id}/carddav/unlink", routes.RequireSensitiveAccess, csrf.Validate, routes.UnlinkCardDAV)
		f.Post("/contact/{id}/carddav/migrate", routes.RequireSensitiveAccess, csrf.Validate, routes.MigrateToCardDAV)
		f.Get("/carddav/contacts", routes.ListCardDAVContacts)
		f.Get("/carddav/picker", routes.CardDAVPicker)
		f.Post("/contact/{id}/delete", routes.RequireSensitiveAccess, csrf.Validate, routes.DeleteContact)
		f.Post("/contact/{id}/tag", routes.RequireSensitiveAccess, csrf.Validate, routes.AddTag)
		f.Post("/contact/{id}/tag/{tag_id}/delete", routes.RequireSensitiveAccess, csrf.Validate, routes.RemoveTag)

		// Bulk contact operations
		f.Get("/bulk-contact-log", routes.RequireSensitiveAccess, routes.BulkContactLogForm)
		f.Post("/bulk-contact-log", routes.RequireSensitiveAccess, csrf.Validate, routes.BulkAddLog)

		// Service contacts routes
		f.Get("/service-contacts", routes.ListServiceContacts)

		// Tag management routes
		f.Get("/tags", routes.ListTags)
		f.Get("/tags/{id}", routes.ViewTagContacts)
		f.Get("/tags/{id}/edit", routes.EditTagForm)
		f.Post("/tags/{id}/edit", csrf.Validate, routes.UpdateTag)
		f.Post("/tags/{id}/delete", csrf.Validate, routes.DeleteTag)

		// Zettelkasten routes
		f.Get("/zk", routes.ZettelkastenIndex)
		f.Get("/zk/random", routes.ZettelkastenRandom)
		f.Get("/zk/list", routes.ZettelkastenList)
		f.Get("/zk/chat", routes.ZettelkastenChat)
		f.Post("/zk/chat/links", csrf.Validate, routes.ZettelkastenChatLinks)
		f.Post("/zk/chat/backlinks", csrf.Validate, routes.ZettelkastenChatBacklinks)
		f.Post("/zk/chat/stream", csrf.Validate, routes.ZettelkastenChatStream)
		f.Get("/zk/{id}", routes.ViewZKNote)
		f.Post("/zk/{id}/comment", csrf.Validate, routes.AddZettelComment)
		f.Post("/zk/{id}/comment/{comment_id}/delete", csrf.Validate, routes.DeleteZettelComment)
		f.Get("/zettel-inbox", routes.ZettelCommentsInbox)
		f.Post("/rebuild-cache", csrf.Validate, routes.RebuildCache)

		// Inventory routes (admin)
		f.Get("/inventory/new", routes.NewInventoryItemForm)
		f.Post("/inventory/new", csrf.Validate, routes.CreateInventoryItem)
		f.Get("/inventory/{id}/edit", routes.EditInventoryItemForm)
		f.Post("/inventory/{id}/edit", csrf.Validate, routes.UpdateInventoryItem)
		f.Post("/inventory/{id}/delete", csrf.Validate, routes.DeleteInventoryItem)
		f.Post("/inventory/{id}/comment", csrf.Validate, routes.AddInventoryComment)
		f.Post("/inventory/{id}/comment/{comment_id}/delete", csrf.Validate, routes.DeleteInventoryComment)

		// Health tracking routes (admin)
		f.Get("/health/new", routes.NewHealthProfileForm)
		f.Post("/health/new", csrf.Validate, routes.CreateHealthProfile)
		f.Get("/health/{id}/edit", routes.EditHealthProfileForm)
		f.Post("/health/{id}/edit", csrf.Validate, routes.UpdateHealthProfile)
		f.Post("/health/{id}/delete", csrf.Validate, routes.DeleteHealthProfile)
		f.Get("/health/{profile_id}/followup/new", routes.NewFollowupForm)
		f.Post("/health/{profile_id}/followup/new", csrf.Validate, routes.CreateFollowup)
		f.Get("/health/{profile_id}/followup/{id}/edit", routes.EditFollowupForm)
		f.Post("/health/{profile_id}/followup/{id}/edit", csrf.Validate, routes.UpdateFollowup)
		f.Post("/health/{profile_id}/followup/{id}/delete", csrf.Validate, routes.DeleteFollowup)
		f.Post("/health/{profile_id}/followup/{followup_id}/result", csrf.Validate, routes.AddLabResult)
		f.Get("/health/{profile_id}/followup/{followup_id}/result/{id}/edit", routes.EditLabResultForm)
		f.Post("/health/{profile_id}/followup/{followup_id}/result/{id}/edit", csrf.Validate, routes.UpdateLabResult)
		f.Post("/health/{profile_id}/followup/{followup_id}/result/{id}/delete", csrf.Validate, routes.DeleteLabResult)

		// WhatsApp routes
		f.Get("/whatsapp", routes.WhatsAppPairing)
		f.Post("/whatsapp/connect", csrf.Validate, routes.WhatsAppConnect)
		f.Post("/whatsapp/disconnect", csrf.Validate, routes.WhatsAppDisconnect)
		f.Get("/whatsapp/status", routes.WhatsAppStatusAPI)

		// Security admin actions
		f.Post("/security/invites", csrf.Validate, routes.CreateUserInvite)
		f.Post("/security/invites/{id}/delete", csrf.Validate, routes.DeleteUserInvite)
		f.Post("/security/users/{id}/delete", csrf.Validate, routes.DeleteUserAccount)
		f.Post("/security/users/{id}/health-shares", csrf.Validate, routes.UpdateHealthProfileShares)
	}, routes.RequireAuth, routes.RequireAdmin, routes.RequireSensitiveAccessForHealth)

	port := cmd.String("port")

	appLogger.Info("Starting web server", "port", port)
	srv := &http.Server{
		Addr:         fmt.Sprintf("0.0.0.0:%s", port),
		Handler:      f,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute, // Extended for SSE streaming (AI summary)
	}

	if err := srv.ListenAndServe(); err != nil {
		appLogger.Fatal("web server failed", "error", err)
	}

	return nil
}

// handleWhatsAppMessage is called when a WhatsApp message is sent or received.
// It updates the last_auto_contact timestamp for matching contacts.
func handleWhatsAppMessage(jid string, timestamp time.Time, isOutgoing bool, message string) {
	ctx := context.Background()

	// Extract phone number from JID
	phone := whatsapp.JIDToPhone(jid)

	// Find contact by phone number
	contactID, err := db.FindContactByPhone(ctx, phone)
	if err != nil {
		whatsappLogger.Error("Failed to find contact by phone", "phone", phone, "error", err)
		return
	}

	if contactID == nil {
		// No matching contact found, ignore
		return
	}

	// Update the contact's auto-contact timestamp
	err = db.UpdateContactAutoTimestamp(ctx, *contactID, timestamp)
	if err != nil {
		whatsappLogger.Error("Failed to update auto contact timestamp", "contact_id", *contactID, "error", err)
		return
	}

	cleanMessage := strings.TrimSpace(message)
	if cleanMessage != "" {
		sentAt := timestamp.Format(time.RFC3339Nano)
		sender := db.ChatSenderThem
		if isOutgoing {
			sender = db.ChatSenderMe
		}

		err = db.AddChat(ctx, db.AddChatInput{
			ContactID: *contactID,
			Platform:  db.ChatPlatformWhatsApp,
			Sender:    sender,
			Message:   cleanMessage,
			SentAt:    &sentAt,
		})
		if err != nil {
			whatsappLogger.Error("Failed to add WhatsApp chat entry", "contact_id", *contactID, "error", err)
		}
	}

	direction := "received"
	if isOutgoing {
		direction = "sent"
	}
	whatsappLogger.Info("Updated last_auto_contact", "contact_id", *contactID, "direction", direction)
}
