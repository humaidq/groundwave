/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"

	"github.com/humaidq/groundwave/whatsapp"
)

// WhatsAppPairing renders the WhatsApp pairing/status page
func WhatsAppPairing(_ flamego.Context, t template.Template, data template.Data) {
	client := whatsapp.GetClient()

	if client != nil {
		data["Status"] = string(client.GetStatus())
		data["QRCode"] = client.GetQRCode()
		data["IsConnected"] = client.IsConnected()
	} else {
		data["Status"] = "unavailable"
		data["QRCode"] = ""
		data["IsConnected"] = false
	}

	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "WhatsApp", URL: "", IsCurrent: true},
	}

	t.HTML(http.StatusOK, "whatsapp_pairing")
}

// WhatsAppConnect initiates the WhatsApp connection
func WhatsAppConnect(c flamego.Context, s session.Session) {
	client := whatsapp.GetClient()

	if client == nil {
		SetErrorFlash(s, "WhatsApp is not available")
		c.Redirect("/whatsapp", http.StatusSeeOther)

		return
	}

	// Use background context since the connection needs to persist beyond the HTTP request
	go func() {
		if err := client.Connect(context.Background()); err != nil {
			logger.Error("WhatsApp connect failed", "error", err)
		}
	}()

	c.Redirect("/whatsapp", http.StatusSeeOther)
}

// WhatsAppDisconnect disconnects the WhatsApp session
func WhatsAppDisconnect(c flamego.Context, s session.Session) {
	client := whatsapp.GetClient()

	if client == nil {
		SetErrorFlash(s, "WhatsApp is not available")
		c.Redirect("/whatsapp", http.StatusSeeOther)

		return
	}

	err := client.Logout()
	if err != nil {
		SetErrorFlash(s, "Failed to disconnect WhatsApp")
	} else {
		SetSuccessFlash(s, "WhatsApp disconnected")
	}

	c.Redirect("/whatsapp", http.StatusSeeOther)
}

// WhatsAppStatusAPI returns the current WhatsApp status as JSON
func WhatsAppStatusAPI(c flamego.Context) {
	client := whatsapp.GetClient()

	response := map[string]interface{}{
		"status":    "unavailable",
		"qrCode":    "",
		"connected": false,
	}

	if client != nil {
		response["status"] = string(client.GetStatus())
		response["qrCode"] = client.GetQRCode()
		response["connected"] = client.IsConnected()
	}

	c.ResponseWriter().Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(c.ResponseWriter()).Encode(response); err != nil {
		logger.Error("Error encoding WhatsApp status", "error", err)
	}
}
