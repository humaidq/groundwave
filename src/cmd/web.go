/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/flamego/csrf"
	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"
	"github.com/urfave/cli/v3"

	"github.com/humaidq/groundwave/db"
	"github.com/humaidq/groundwave/routes"
	"github.com/humaidq/groundwave/static"
	"github.com/humaidq/groundwave/templates"
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

	// Sync CardDAV contacts
	log.Println("Syncing contacts from CardDAV...")
	if err := db.SyncAllCardDAVContacts(ctx); err != nil {
		log.Printf("Warning: CardDAV sync failed: %v", err)
		// Don't fail startup, just log the error
	}

	// Create maps directory if it doesn't exist
	if err := os.MkdirAll("maps", 0755); err != nil {
		return fmt.Errorf("failed to create maps directory: %w", err)
	}

	f := flamego.Classic()

	// Setup flamego
	fs, err := template.EmbedFS(templates.Templates, ".", []string{".html"})
	if err != nil {
		panic(err)
	}
	f.Use(session.Sessioner())
	f.Use(csrf.Csrfer())
	f.Use(template.Templater(template.Options{
		FileSystem: fs,
	}))
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

	// Protected routes (require authentication)
	f.Group("", func() {
		f.Get("/", routes.Welcome)
		f.Get("/logout", routes.Logout)
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
		f.Post("/contact/{id}/phone/{phone_id}/delete", routes.DeletePhone)
		f.Post("/contact/{id}/url/{url_id}/delete", routes.DeleteURL)
		f.Post("/contact/{id}/log", routes.AddLog)
		f.Post("/contact/{id}/log/{log_id}/delete", routes.DeleteLog)
		f.Post("/contact/{id}/note", routes.AddNote)
		f.Post("/contact/{id}/note/{note_id}/delete", routes.DeleteNote)
		f.Post("/contact/{id}/carddav/link", routes.LinkCardDAV)
		f.Post("/contact/{id}/carddav/unlink", routes.UnlinkCardDAV)
		f.Get("/carddav/contacts", routes.ListCardDAVContacts)
		f.Get("/carddav/picker", routes.CardDAVPicker)
		f.Post("/contact/{id}/delete", routes.DeleteContact)
		f.Post("/contact/{id}/tag", routes.AddTag)
		f.Post("/contact/{id}/tag/{tag_id}/delete", routes.RemoveTag)

		// Tag management routes
		f.Get("/tags", routes.ListTags)
		f.Get("/tags/{id}", routes.ViewTagContacts)
		f.Get("/tags/{id}/edit", routes.EditTagForm)
		f.Post("/tags/{id}/edit", routes.UpdateTag)

		// Zettelkasten routes
		f.Get("/zk", routes.ZettelkastenIndex)
		f.Get("/zk/{id}", routes.ViewZKNote)
		f.Post("/zk/{id}/comment", routes.AddZettelComment)
		f.Post("/zk/{id}/comment/{comment_id}/delete", routes.DeleteZettelComment)
		f.Get("/zettel-inbox", routes.ZettelCommentsInbox)

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
	}, routes.RequireAuth)

	port := cmd.String("port")

	log.Printf("Starting web server on port %s\n", port)
	srv := &http.Server{
		Addr:         fmt.Sprintf("0.0.0.0:%s", port),
		Handler:      f,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())

	return nil
}
