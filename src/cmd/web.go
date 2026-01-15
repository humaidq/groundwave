/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package cmd

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
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

	// Set DATABASE_URL for db package
	os.Setenv("DATABASE_URL", databaseURL)

	// Initialize database connection
	log.Println("Connecting to database...")
	if err := db.Init(ctx); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	// Sync schema
	log.Println("Syncing database schema...")
	if err := db.SyncSchema(ctx); err != nil {
		return fmt.Errorf("failed to sync schema: %w", err)
	}
	log.Println("Database schema synced successfully")

	// Start backlink cache refresh worker
	db.StartBacklinkRefreshWorker(context.Background())

	// Sync CardDAV contacts
	log.Println("Syncing contacts from CardDAV...")
	if err := db.SyncAllCardDAVContacts(ctx); err != nil {
		log.Printf("Warning: CardDAV sync failed: %v", err)
		// Don't fail startup, just log the error
	}

	// Initialize WhatsApp client (optional feature)
	log.Println("Initializing WhatsApp client...")
	if err := whatsapp.Initialize(ctx, databaseURL, handleWhatsAppMessage); err != nil {
		log.Printf("Warning: WhatsApp initialization failed: %v", err)
		// Don't fail startup, WhatsApp is optional
	} else {
		log.Println("WhatsApp client initialized successfully")
	}

	// Create maps directory if it doesn't exist
	if err := os.MkdirAll("maps", 0755); err != nil {
		return fmt.Errorf("failed to create maps directory: %w", err)
	}

	f := flamego.Classic()

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
	f.Use(csrf.Csrfer())
	f.Use(flamegoTemplate.Templater(flamegoTemplate.Options{
		FileSystem: fs,
		FuncMaps:   []template.FuncMap{funcMap},
	}))
	// Flash message middleware - retrieve flash from session and pass to templates
	f.Use(func(data flamegoTemplate.Data, flash session.Flash) {
		if msg, ok := flash.(routes.FlashMessage); ok {
			data["Flash"] = msg
		}
	})
	f.Use(flamego.Static(flamego.StaticOptions{
		FileSystem: http.FS(static.Static),
	}))
	// Serve maps directory for grid square maps
	f.Use(flamego.Static(flamego.StaticOptions{
		Directory: "maps",
	}))

	// Add request logging middleware
	f.Use(func(c flamego.Context) {
		start := time.Now()
		c.Next()

		// Log the request
		logEntry := fmt.Sprintf("[%s] %s %s %s - %v\n",
			start.Format("2006-01-02 15:04:05"),
			c.Request().Method,
			c.Request().URL.Path,
			c.Request().RemoteAddr,
			time.Since(start))

		// Append to log file
		logFile, err := os.OpenFile("groundwave-access.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			logFile.WriteString(logEntry)
			logFile.Close()
		}
	})

	// Public routes (no authentication required)
	f.Get("/login", routes.LoginForm)
	f.Post("/login", routes.Login)
	f.Get("/connectivity", func(c flamego.Context) { c.ResponseWriter().Write([]byte("1")) })
	f.Get("/ext/auth", routes.RequireAuth, routes.ExtensionAuth)
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
		f.Get("/", routes.Welcome)
		f.Get("/logout", routes.Logout)
		f.Post("/private-mode/toggle", routes.TogglePrivateMode)
		f.Get("/contacts", routes.Home)
		f.Get("/overdue", routes.Overdue)
		f.Get("/qsl", routes.QSL)
		f.Get("/qsl/{id}", routes.ViewQSO)
		f.Post("/qsl/import", routes.ImportADIF)
		f.Get("/contact/new", routes.NewContactForm)
		f.Post("/contact/new", routes.CreateContact)
		f.Get("/contact/{id}", routes.ViewContact)
		f.Get("/contact/{id}/edit", routes.EditContactForm)
		f.Post("/contact/{id}/edit", routes.UpdateContact)
		f.Post("/contact/{id}/email", routes.AddEmail)
		f.Post("/contact/{id}/phone", routes.AddPhone)
		f.Post("/contact/{id}/url", routes.AddURL)
		f.Post("/contact/{id}/email/{email_id}/delete", routes.DeleteEmail)
		f.Post("/contact/{id}/email/{email_id}/edit", routes.UpdateEmail)
		f.Post("/contact/{id}/phone/{phone_id}/delete", routes.DeletePhone)
		f.Post("/contact/{id}/phone/{phone_id}/edit", routes.UpdatePhone)
		f.Post("/contact/{id}/url/{url_id}/delete", routes.DeleteURL)
		f.Post("/contact/{id}/log", routes.AddLog)
		f.Post("/contact/{id}/log/{log_id}/delete", routes.DeleteLog)
		f.Post("/contact/{id}/note", routes.AddNote)
		f.Post("/contact/{id}/note/{note_id}/delete", routes.DeleteNote)
		f.Post("/contact/{id}/carddav/link", routes.LinkCardDAV)
		f.Post("/contact/{id}/carddav/unlink", routes.UnlinkCardDAV)
		f.Post("/contact/{id}/carddav/migrate", routes.MigrateToCardDAV)
		f.Get("/carddav/contacts", routes.ListCardDAVContacts)
		f.Get("/carddav/picker", routes.CardDAVPicker)
		f.Post("/contact/{id}/delete", routes.DeleteContact)
		f.Post("/contact/{id}/tag", routes.AddTag)
		f.Post("/contact/{id}/tag/{tag_id}/delete", routes.RemoveTag)

		// Service contacts routes
		f.Get("/service-contacts", routes.ListServiceContacts)

		// Tag management routes
		f.Get("/tags", routes.ListTags)
		f.Get("/tags/{id}", routes.ViewTagContacts)
		f.Get("/tags/{id}/edit", routes.EditTagForm)
		f.Post("/tags/{id}/edit", routes.UpdateTag)
		f.Post("/tags/{id}/delete", routes.DeleteTag)

		// Zettelkasten routes
		f.Get("/zk", routes.ZettelkastenIndex)
		f.Get("/zk/chat", routes.ZettelkastenChat)
		f.Post("/zk/chat/links", routes.ZettelkastenChatLinks)
		f.Post("/zk/chat/backlinks", routes.ZettelkastenChatBacklinks)
		f.Post("/zk/chat/stream", routes.ZettelkastenChatStream)
		f.Get("/zk/{id}", routes.ViewZKNote)
		f.Post("/zk/{id}/comment", routes.AddZettelComment)
		f.Post("/zk/{id}/comment/{comment_id}/delete", routes.DeleteZettelComment)
		f.Get("/zettel-inbox", routes.ZettelCommentsInbox)
		f.Post("/zk/refresh-links", routes.RefreshBacklinks)

		// Inventory routes
		f.Get("/inventory", routes.InventoryList)
		f.Get("/inventory/new", routes.NewInventoryItemForm)
		f.Post("/inventory/new", routes.CreateInventoryItem)
		f.Get("/inventory/{id}", routes.ViewInventoryItem)
		f.Get("/inventory/{id}/edit", routes.EditInventoryItemForm)
		f.Post("/inventory/{id}/edit", routes.UpdateInventoryItem)
		f.Post("/inventory/{id}/delete", routes.DeleteInventoryItem)
		f.Post("/inventory/{id}/comment", routes.AddInventoryComment)
		f.Post("/inventory/{id}/comment/{comment_id}/delete", routes.DeleteInventoryComment)
		f.Get("/inventory/{id}/file/{filename}", routes.DownloadInventoryFile)

		// Health tracking routes
		f.Get("/health", routes.ListHealthProfiles)
		f.Get("/health/new", routes.NewHealthProfileForm)
		f.Post("/health/new", routes.CreateHealthProfile)
		f.Get("/health/{id}", routes.ViewHealthProfile)
		f.Get("/health/{id}/edit", routes.EditHealthProfileForm)
		f.Post("/health/{id}/edit", routes.UpdateHealthProfile)
		f.Post("/health/{id}/delete", routes.DeleteHealthProfile)
		f.Get("/health/{profile_id}/followup/new", routes.NewFollowupForm)
		f.Post("/health/{profile_id}/followup/new", routes.CreateFollowup)
		f.Get("/health/{profile_id}/followup/{id}", routes.ViewFollowup)
		f.Get("/health/{profile_id}/followup/{id}/edit", routes.EditFollowupForm)
		f.Post("/health/{profile_id}/followup/{id}/edit", routes.UpdateFollowup)
		f.Post("/health/{profile_id}/followup/{id}/delete", routes.DeleteFollowup)
		f.Post("/health/{profile_id}/followup/{id}/ai-summary", routes.GenerateAISummary)
		f.Post("/health/{profile_id}/followup/{followup_id}/result", routes.AddLabResult)
		f.Get("/health/{profile_id}/followup/{followup_id}/result/{id}/edit", routes.EditLabResultForm)
		f.Post("/health/{profile_id}/followup/{followup_id}/result/{id}/edit", routes.UpdateLabResult)
		f.Post("/health/{profile_id}/followup/{followup_id}/result/{id}/delete", routes.DeleteLabResult)

		// WhatsApp routes
		f.Get("/whatsapp", routes.WhatsAppPairing)
		f.Post("/whatsapp/connect", routes.WhatsAppConnect)
		f.Post("/whatsapp/disconnect", routes.WhatsAppDisconnect)
		f.Get("/whatsapp/status", routes.WhatsAppStatusAPI)
	}, routes.RequireAuth)

	port := cmd.String("port")

	log.Printf("Starting web server on port %s\n", port)
	srv := &http.Server{
		Addr:         fmt.Sprintf("0.0.0.0:%s", port),
		Handler:      f,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute, // Extended for SSE streaming (AI summary)
	}

	log.Fatal(srv.ListenAndServe())

	return nil
}

// handleWhatsAppMessage is called when a WhatsApp message is sent or received.
// It updates the last_auto_contact timestamp for matching contacts.
func handleWhatsAppMessage(jid string, timestamp time.Time, isOutgoing bool) {
	ctx := context.Background()

	// Extract phone number from JID
	phone := whatsapp.JIDToPhone(jid)

	// Find contact by phone number
	contactID, err := db.FindContactByPhone(ctx, phone)
	if err != nil {
		log.Printf("Error finding contact by phone %s: %v", phone, err)
		return
	}

	if contactID == nil {
		// No matching contact found, ignore
		return
	}

	// Update the contact's auto-contact timestamp
	err = db.UpdateContactAutoTimestamp(ctx, *contactID, timestamp)
	if err != nil {
		log.Printf("Error updating auto contact timestamp for %s: %v", *contactID, err)
		return
	}

	direction := "received"
	if isOutgoing {
		direction = "sent"
	}
	log.Printf("Updated last_auto_contact for contact %s (WhatsApp message %s)", *contactID, direction)
}
