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
	"net/url"
	"os"
	"regexp"
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

// CmdStart defines the command that starts the web server.
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
			Usage: "enables development mode (overrides GROUNDWAVE_ENV)",
		},
	},
	Action: start,
}

const runtimeEnvVar = "GROUNDWAVE_ENV"

var safeImageDataURLPattern = regexp.MustCompile(`(?i)^data:image/(?:png|jpe?g|gif|webp|bmp);base64,[a-z0-9+/=]+$`)

func safeImageURL(raw *string) template.URL {
	if raw == nil {
		return ""
	}

	value := strings.TrimSpace(*raw)
	if value == "" {
		return ""
	}

	parsed, err := url.Parse(value)
	if err == nil {
		scheme := strings.ToLower(parsed.Scheme)
		if (scheme == "http" || scheme == "https") && parsed.Host != "" {
			return template.URL(value) //nolint:gosec // Value is constrained to validated absolute http(s) URLs.
		}
	}

	if safeImageDataURLPattern.MatchString(value) {
		return template.URL(value) //nolint:gosec // Value is constrained to a strict image data URL allowlist.
	}

	return ""
}

func resolveRuntimeEnv(cmd *cli.Command) (flamego.EnvType, error) {
	if cmd.Bool("dev") {
		return flamego.EnvTypeDev, nil
	}

	rawEnv := strings.ToLower(strings.TrimSpace(os.Getenv(runtimeEnvVar)))
	switch rawEnv {
	case "", "development", "dev":
		return flamego.EnvTypeDev, nil
	case "production", "prod":
		return flamego.EnvTypeProd, nil
	default:
		return "", errInvalidRuntimeEnv
	}
}

func start(ctx context.Context, cmd *cli.Command) (err error) {
	// Get database URL
	databaseURL := cmd.String("database-url")
	if databaseURL == "" {
		return errDatabaseURLRequired
	}

	csrfSecret := os.Getenv("CSRF_SECRET")
	if csrfSecret == "" {
		return errCSRFSecretRequired
	}

	runtimeEnv, err := resolveRuntimeEnv(cmd)
	if err != nil {
		return err
	}

	flamego.SetEnv(runtimeEnv)
	isProduction := runtimeEnv == flamego.EnvTypeProd

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
	db.StartRebuildCacheWorker(ctx)

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
	if err := os.MkdirAll("maps", 0o750); err != nil {
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
			value := float64(size) / 1000
			unitIndex := 0

			for value >= 1000 && unitIndex < len(units)-1 {
				value /= 1000
				unitIndex++
			}

			formatted := strings.TrimSuffix(fmt.Sprintf("%.1f", value), ".0")

			unit := "kB"
			if unitIndex >= 0 && unitIndex < len(units) {
				unit = units[unitIndex]
			}

			return fmt.Sprintf("%s %s", formatted, unit)
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
		"safeImageURL": safeImageURL,
		"phoneDialValue": func(phone string) string {
			trimmed := strings.TrimSpace(phone)
			if trimmed == "" {
				return ""
			}

			if strings.HasPrefix(strings.ToLower(trimmed), "tel:") {
				trimmed = strings.TrimSpace(trimmed[len("tel:"):])
			}

			var b strings.Builder

			for i, r := range trimmed {
				if r >= '0' && r <= '9' {
					b.WriteRune(r)
					continue
				}

				if r == '+' && i == 0 {
					b.WriteRune(r)
				}
			}

			return b.String()
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
	// Configure PostgreSQL session store with 14-day absolute expiry
	f.Use(session.Sessioner(session.Options{
		Initer: db.PostgresSessionIniter(),
		Config: db.PostgresSessionConfig{
			Lifetime:  14 * 24 * time.Hour, // 14 days
			TableName: "flamego_sessions",
		},
		Cookie: session.CookieOptions{
			MaxAge:   14 * 24 * 60 * 60, // 14 days in seconds
			Secure:   isProduction,
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
	f.Group("", func() {
		f.Post("/webauthn/login/start", routes.PasskeyLoginStart)
		f.Post("/webauthn/login/finish", routes.PasskeyLoginFinish)
		f.Post("/webauthn/setup/start", routes.SetupStart)
		f.Post("/webauthn/setup/finish", routes.SetupFinish)
		f.Post("/oqrs", routes.OQRSFind)
		f.Post("/oqrs/request", routes.OQRSRequestCard)
	}, csrf.Validate)
	f.Get("/connectivity", func(c flamego.Context) {
		if _, err := c.ResponseWriter().Write([]byte("1")); err != nil {
			appLogger.Error("Error writing connectivity response", "error", err)
		}
	})
	f.Get("/oqrs", routes.OQRSIndex)
	f.Get("/oqrs/{path: **}", routes.OQRSView)
	f.Get("/qrz", routes.QRZ)
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
		f.Get("/home", routes.HomeWikiIndex)
		f.Get("/home/{id}", routes.ViewHomeWikiNote)
		f.Get("/break-glass", routes.BreakGlassForm)

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
		f.Get("/health", routes.ListHealthProfiles)
		f.Get("/health/{id}", routes.ViewHealthProfile)
		f.Get("/health/{profile_id}/followup/{id}", routes.ViewFollowup)

		// Security & passkeys
		f.Get("/security", routes.Security)

		f.Group("", func() {
			f.Post("/logout", routes.Logout)
			f.Post("/sensitive-access/lock", routes.LockSensitiveAccess)
			f.Post("/break-glass/start", routes.BreakGlassStart)
			f.Post("/break-glass/finish", routes.BreakGlassFinish)
			f.Post("/health/break-glass/start", routes.BreakGlassStart)
			f.Post("/health/break-glass/finish", routes.BreakGlassFinish)
			f.Post("/files/upload", routes.UploadFilesFile)
			f.Post("/health/{profile_id}/followup/{id}/ai-summary", routes.GenerateAISummary)
			f.Post("/security/invalidate-other", routes.InvalidateOtherSessions)
			f.Post("/security/sessions/{id}/invalidate", routes.InvalidateSession)
			f.Post("/webauthn/passkey/start", routes.PasskeyRegistrationStart)
			f.Post("/webauthn/passkey/finish", routes.PasskeyRegistrationFinish)
			f.Post("/security/passkeys/{id}/delete", routes.DeletePasskey)
		}, csrf.Validate)
	}, routes.RequireAuth, routes.RequireSensitiveAccessForHealth)

	// Admin-only routes
	f.Group("", func() {
		f.Get("/todo", routes.Todo)

		// Sensitive admin routes
		f.Group("", func() {
			f.Get("/timeline", routes.Timeline)
			f.Get("/journal/{date}", routes.ViewJournalEntry)
			f.Get("/ledger", routes.LedgerIndex)
			f.Get("/ledger/history", routes.LedgerHistoryView)
			f.Get("/ledger/budgets/new", routes.LedgerBudgetNewForm)
			f.Get("/ledger/budgets/{id}/edit", routes.LedgerBudgetEditForm)
			f.Get("/ledger/accounts/new", routes.LedgerAccountNewForm)
			f.Get("/ledger/accounts/{id}", routes.LedgerAccountView)
			f.Get("/ledger/accounts/{id}/edit", routes.LedgerAccountEditForm)
			f.Get("/ledger/accounts/{id}/transactions/new", routes.LedgerTransactionNewForm)
			f.Get("/ledger/accounts/{id}/transactions/{tx_id}/edit", routes.LedgerTransactionEditForm)
			f.Get("/ledger/accounts/{id}/reconcile/new", routes.LedgerReconcileNewForm)
			f.Get("/ledger/accounts/{id}/reconciliations/{rec_id}/edit", routes.LedgerReconciliationEditForm)
			f.Get("/contact/new", routes.NewContactForm)
			f.Get("/contact/{id}/edit", routes.EditContactForm)

			// Bulk contact operations
			f.Get("/bulk-contact-log", routes.BulkContactLogForm)

			f.Group("", func() {
				f.Post("/journal/{date}/location", routes.AddJournalLocation)
				f.Post("/journal/{date}/location/{location_id}/delete", routes.DeleteJournalLocation)
				f.Post("/ledger/budgets/new", routes.CreateLedgerBudget)
				f.Post("/ledger/budgets/{id}/edit", routes.UpdateLedgerBudget)
				f.Post("/ledger/budgets/{id}/delete", routes.DeleteLedgerBudget)
				f.Post("/ledger/accounts/new", routes.CreateLedgerAccount)
				f.Post("/ledger/accounts/{id}/edit", routes.UpdateLedgerAccount)
				f.Post("/ledger/accounts/{id}/delete", routes.DeleteLedgerAccount)
				f.Post("/ledger/accounts/{id}/transactions", routes.CreateLedgerTransaction)
				f.Post("/ledger/accounts/{id}/transactions/{tx_id}/edit", routes.UpdateLedgerTransaction)
				f.Post("/ledger/accounts/{id}/transactions/{tx_id}/delete", routes.DeleteLedgerTransaction)
				f.Post("/ledger/accounts/{id}/reconcile", routes.CreateLedgerReconciliation)
				f.Post("/ledger/accounts/{id}/reconciliations/{rec_id}/edit", routes.UpdateLedgerReconciliation)
				f.Post("/ledger/accounts/{id}/reconciliations/{rec_id}/delete", routes.DeleteLedgerReconciliation)
				f.Post("/contact/new", routes.CreateContact)
				f.Post("/contact/{id}/edit", routes.UpdateContact)
				f.Post("/contact/{id}/log", routes.AddLog)
				f.Post("/contact/{id}/log/{log_id}/edit", routes.UpdateLog)
				f.Post("/contact/{id}/log/{log_id}/delete", routes.DeleteLog)
				f.Post("/contact/{id}/note", routes.AddNote)
				f.Post("/contact/{id}/note/{note_id}/edit", routes.UpdateNote)
				f.Post("/contact/{id}/note/{note_id}/delete", routes.DeleteNote)
				f.Post("/contact/{id}/carddav/link", routes.LinkCardDAV)
				f.Post("/contact/{id}/carddav/unlink", routes.UnlinkCardDAV)
				f.Post("/contact/{id}/carddav/migrate", routes.MigrateToCardDAV)
				f.Post("/contact/{id}/delete", routes.DeleteContact)
				f.Post("/contact/{id}/tag", routes.AddTag)
				f.Post("/contact/{id}/tag/{tag_id}/delete", routes.RemoveTag)
				f.Post("/bulk-contact-log", routes.BulkAddLog)
			}, csrf.Validate)
		}, routes.RequireSensitiveAccess)

		f.Get("/contacts", routes.Home)
		f.Get("/overdue", routes.Overdue)
		f.Get("/qsl", routes.QSL)
		f.Get("/qsl/callsigns", routes.QSLCallsigns)
		f.Get("/qsl/export", routes.ExportADIF)
		f.Get("/qsl/{id}", routes.ViewQSO)
		f.Get("/qrz/{callsign: **}", routes.ViewQRZCallsign)
		f.Get("/files/edit", routes.FilesEditForm)
		f.Get("/contact/{id}", routes.ViewContact)
		f.Get("/contact/{id}/chats", routes.ViewContactChats)
		f.Get("/carddav/contacts", routes.ListCardDAVContacts)
		f.Get("/carddav/picker", routes.CardDAVPicker)

		// Service contacts routes
		f.Get("/service-contacts", routes.ListServiceContacts)

		// Tag management routes
		f.Get("/tags", routes.ListTags)
		f.Get("/tags/{id}", routes.ViewTagContacts)
		f.Get("/tags/{id}/edit", routes.EditTagForm)

		// Zettelkasten routes
		f.Get("/zk", routes.ZettelkastenIndex)
		f.Get("/zk/random", routes.ZettelkastenRandom)
		f.Get("/zk/list", routes.ZettelkastenList)
		f.Get("/zk/chat", routes.ZettelkastenChat)
		f.Get("/zk/{id}", routes.ViewZKNote)
		f.Get("/zettel-inbox", routes.ZettelCommentsInbox)

		// Inventory routes (admin)
		f.Get("/inventory/new", routes.NewInventoryItemForm)
		f.Get("/inventory/{id}/edit", routes.EditInventoryItemForm)

		// Health tracking routes (admin)
		f.Get("/health/new", routes.NewHealthProfileForm)
		f.Get("/health/{id}/edit", routes.EditHealthProfileForm)
		f.Get("/health/{profile_id}/followup/new", routes.NewFollowupForm)
		f.Get("/health/{profile_id}/followup/{id}/edit", routes.EditFollowupForm)
		f.Get("/health/{profile_id}/followup/{followup_id}/result/{id}/edit", routes.EditLabResultForm)

		// WhatsApp routes
		f.Get("/whatsapp", routes.WhatsAppPairing)
		f.Get("/whatsapp/status", routes.WhatsAppStatusAPI)

		f.Group("", func() {
			f.Post("/qsl/import", routes.ImportADIF)
			f.Post("/qsl/import/qrz", routes.ImportQRZLogs)
			f.Post("/qsl/requests/{id}/dismiss", routes.DismissQSLCardRequest)
			f.Post("/qrz/{callsign: **}/sync", routes.SyncQRZCallsign)
			f.Post("/files/mkdir", routes.CreateFilesDirectory)
			f.Post("/files/new", routes.CreateFilesTextFile)
			f.Post("/files/edit", routes.UpdateFilesFile)
			f.Post("/files/rename", routes.RenameFilesEntry)
			f.Post("/files/move", routes.MoveFilesEntry)
			f.Post("/files/delete", routes.DeleteFilesEntry)
			f.Post("/files/rmdir", routes.DeleteFilesDirectory)
			f.Post("/contact/{id}/chats", routes.AddContactChat)
			f.Post("/contact/{id}/chats/{chat_id}/edit", routes.UpdateContactChat)
			f.Post("/contact/{id}/chat-summary", routes.GenerateContactChatSummary)
			f.Post("/contact/{id}/email", routes.AddEmail)
			f.Post("/contact/{id}/phone", routes.AddPhone)
			f.Post("/contact/{id}/url", routes.AddURL)
			f.Post("/contact/{id}/email/{email_id}/delete", routes.DeleteEmail)
			f.Post("/contact/{id}/email/{email_id}/edit", routes.UpdateEmail)
			f.Post("/contact/{id}/phone/{phone_id}/delete", routes.DeletePhone)
			f.Post("/contact/{id}/phone/{phone_id}/edit", routes.UpdatePhone)
			f.Post("/contact/{id}/url/{url_id}/delete", routes.DeleteURL)
			f.Post("/tags/{id}/edit", routes.UpdateTag)
			f.Post("/tags/{id}/delete", routes.DeleteTag)
			f.Post("/zk/chat/links", routes.ZettelkastenChatLinks)
			f.Post("/zk/chat/backlinks", routes.ZettelkastenChatBacklinks)
			f.Post("/zk/chat/stream", routes.ZettelkastenChatStream)
			f.Post("/zk/{id}/comment", routes.AddZettelComment)
			f.Post("/zk/{id}/comment/{comment_id}/edit", routes.UpdateZettelComment)
			f.Post("/zk/{id}/comment/{comment_id}/delete", routes.DeleteZettelComment)
			f.Post("/zk/{id}/comments/delete", routes.DeleteAllZettelComments)
			f.Post("/rebuild-cache", routes.RebuildCache)
			f.Post("/inventory/new", routes.CreateInventoryItem)
			f.Post("/inventory/{id}/edit", routes.UpdateInventoryItem)
			f.Post("/inventory/{id}/delete", routes.DeleteInventoryItem)
			f.Post("/inventory/{id}/tag", routes.AddInventoryTag)
			f.Post("/inventory/{id}/tag/{tag_id}/delete", routes.RemoveInventoryTag)
			f.Post("/inventory/{id}/comment", routes.AddInventoryComment)
			f.Post("/inventory/{id}/comment/{comment_id}/delete", routes.DeleteInventoryComment)
			f.Post("/health/new", routes.CreateHealthProfile)
			f.Post("/health/{id}/edit", routes.UpdateHealthProfile)
			f.Post("/health/{id}/delete", routes.DeleteHealthProfile)
			f.Post("/health/{profile_id}/followup/new", routes.CreateFollowup)
			f.Post("/health/{profile_id}/followup/{id}/edit", routes.UpdateFollowup)
			f.Post("/health/{profile_id}/followup/{id}/delete", routes.DeleteFollowup)
			f.Post("/health/{profile_id}/followup/{followup_id}/result", routes.AddLabResult)
			f.Post("/health/{profile_id}/followup/{followup_id}/result/{id}/edit", routes.UpdateLabResult)
			f.Post("/health/{profile_id}/followup/{followup_id}/result/{id}/delete", routes.DeleteLabResult)
			f.Post("/whatsapp/connect", routes.WhatsAppConnect)
			f.Post("/whatsapp/disconnect", routes.WhatsAppDisconnect)
			f.Post("/security/invites", routes.CreateUserInvite)
			f.Post("/security/invites/{id}/regenerate", routes.RegenerateUserInvite)
			f.Post("/security/invites/{id}/delete", routes.DeleteUserInvite)
			f.Post("/security/users/{id}/delete", routes.DeleteUserAccount)
			f.Post("/security/users/{id}/health-shares", routes.UpdateHealthProfileShares)
		}, csrf.Validate)
	}, routes.RequireAuth, routes.RequireAdmin, routes.RequireSensitiveAccessForHealth)

	port := cmd.String("port")

	appLogger.Info("Starting web server", "port", port)
	srv := &http.Server{
		Addr:         "0.0.0.0:" + port,
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
