/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package whatsapp

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// Status represents the WhatsApp connection status
type Status string

const (
	StatusDisconnected Status = "disconnected"
	StatusConnecting   Status = "connecting"
	StatusConnected    Status = "connected"
	StatusPairing      Status = "pairing"
)

// MessageHandler is called when a WhatsApp message is sent or received
type MessageHandler func(jid string, timestamp time.Time, isOutgoing bool, message string)

// Client manages the WhatsApp connection via whatsmeow
type Client struct {
	client        *whatsmeow.Client
	container     *sqlstore.Container
	deviceStore   *store.Device
	status        Status
	qrCode        string // Base64 encoded PNG
	mu            sync.RWMutex
	onMessage     MessageHandler
	stopReconnect chan struct{}
}

var (
	instance *Client
	once     sync.Once
)

// GetClient returns the singleton WhatsApp client instance
func GetClient() *Client {
	return instance
}

// Initialize sets up the WhatsApp client with PostgreSQL storage
func Initialize(ctx context.Context, databaseURL string, onMessage MessageHandler) error {
	var initErr error
	once.Do(func() {
		// Set device name to "Groundwave" for WhatsApp linked devices
		store.SetOSInfo("Groundwave", [3]uint32{1, 0, 0})

		// Create a silent logger for whatsmeow
		logger := waLog.Noop

		// Initialize the SQL store with PostgreSQL
		container, err := sqlstore.New(ctx, "pgx", databaseURL, logger)
		if err != nil {
			initErr = fmt.Errorf("failed to create sqlstore: %w", err)
			return
		}

		// Get or create device store
		deviceStore, err := container.GetFirstDevice(ctx)
		if err != nil {
			initErr = fmt.Errorf("failed to get device: %w", err)
			return
		}

		instance = &Client{
			container:     container,
			deviceStore:   deviceStore,
			status:        StatusDisconnected,
			onMessage:     onMessage,
			stopReconnect: make(chan struct{}),
		}

		// If we have an existing device, try to reconnect
		if deviceStore.ID != nil {
			go instance.Reconnect(context.Background())
		}
	})

	return initErr
}

// GetStatus returns the current connection status
func (c *Client) GetStatus() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// GetQRCode returns the current QR code as a base64 PNG string
func (c *Client) GetQRCode() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.qrCode
}

// setStatus updates the connection status thread-safely
func (c *Client) setStatus(status Status) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = status
}

// setQRCode updates the QR code thread-safely
func (c *Client) setQRCode(qrCode string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.qrCode = qrCode
}

// Connect initiates the WhatsApp connection
func (c *Client) Connect(ctx context.Context) error {
	c.setStatus(StatusConnecting)

	// Create new client with structured logger
	c.client = whatsmeow.NewClient(c.deviceStore, newWALogger("whatsmeow"))
	c.client.AddEventHandler(c.handleEvent)

	// Enable auto-reconnect
	c.client.EnableAutoReconnect = true
	c.client.AutoTrustIdentity = true

	// If already logged in, just connect
	if c.client.Store.ID != nil {
		err := c.client.Connect()
		if err != nil {
			c.setStatus(StatusDisconnected)
			return fmt.Errorf("failed to connect: %w", err)
		}
		c.setStatus(StatusConnected)
		return nil
	}

	// Need to pair - get QR code channel BEFORE connecting
	c.setStatus(StatusPairing)
	qrChan, _ := c.client.GetQRChannel(ctx)

	// Connect to WhatsApp
	err := c.client.Connect()
	if err != nil {
		c.setStatus(StatusDisconnected)
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Handle QR code events in goroutine
	go func() {
		for evt := range qrChan {
			logger.Info("WhatsApp QR event", "event", evt.Event)
			if evt.Event == "code" {
				// Generate QR code image
				png, err := qrcode.Encode(evt.Code, qrcode.Medium, 256)
				if err != nil {
					logger.Error("Failed to generate QR code", "error", err)
					continue
				}
				c.setQRCode(base64.StdEncoding.EncodeToString(png))
				logger.Info("WhatsApp QR code generated")
			} else if evt.Event == "success" {
				c.setQRCode("")
				c.setStatus(StatusConnected)
				logger.Info("WhatsApp pairing successful")
			} else if evt.Event == "timeout" {
				c.setQRCode("")
				c.setStatus(StatusDisconnected)
				logger.Warn("WhatsApp QR code timeout")
			} else if evt.Event == "error" {
				c.setQRCode("")
				c.setStatus(StatusDisconnected)
				logger.Error("WhatsApp pairing error", "error", evt.Error)
			}
		}
	}()

	return nil
}

// Reconnect attempts to reconnect with existing credentials
func (c *Client) Reconnect(ctx context.Context) error {
	if c.deviceStore.ID == nil {
		return fmt.Errorf("no existing session to reconnect")
	}

	c.setStatus(StatusConnecting)

	c.client = whatsmeow.NewClient(c.deviceStore, newWALogger("whatsmeow"))
	c.client.AddEventHandler(c.handleEvent)
	c.client.EnableAutoReconnect = true
	c.client.AutoTrustIdentity = true

	err := c.client.Connect()
	if err != nil {
		c.setStatus(StatusDisconnected)
		return fmt.Errorf("failed to reconnect: %w", err)
	}

	c.setStatus(StatusConnected)
	logger.Info("WhatsApp reconnected successfully")
	return nil
}

// Disconnect cleanly disconnects the WhatsApp client
func (c *Client) Disconnect() {
	if c.client != nil {
		c.client.Disconnect()
	}
	c.setStatus(StatusDisconnected)
	c.setQRCode("")
}

// Logout disconnects and removes the device credentials
func (c *Client) Logout() error {
	if c.client == nil {
		return nil
	}

	ctx := context.Background()
	err := c.client.Logout(ctx)
	if err != nil {
		return fmt.Errorf("failed to logout: %w", err)
	}

	c.setStatus(StatusDisconnected)
	c.setQRCode("")

	// Get a fresh device store
	deviceStore, err := c.container.GetFirstDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to get new device: %w", err)
	}
	c.deviceStore = deviceStore

	return nil
}

// handleEvent processes WhatsApp events
func (c *Client) handleEvent(evt interface{}) {
	switch v := evt.(type) {
	case *events.Connected:
		c.setStatus(StatusConnected)
		logger.Info("WhatsApp connected")

	case *events.Disconnected:
		c.setStatus(StatusDisconnected)
		logger.Info("WhatsApp disconnected")

	case *events.LoggedOut:
		c.setStatus(StatusDisconnected)
		logger.Info("WhatsApp logged out")

	case *events.Message:
		c.handleMessage(v)
	}
}

// handleMessage processes incoming/outgoing messages
func (c *Client) handleMessage(evt *events.Message) {
	// Skip group messages - only track direct chats
	if evt.Info.IsGroup {
		return
	}

	messageText := extractMessageText(evt)
	if messageText == "" {
		return
	}

	otherParty := resolveOtherPartyJID(evt.Info)

	logger.Info(
		"WhatsApp message info",
		"from_me", evt.Info.IsFromMe,
		"chat", evt.Info.Chat,
		"sender", evt.Info.Sender,
		"recipient_alt", evt.Info.RecipientAlt,
		"device_sent_meta", evt.Info.DeviceSentMeta,
		"message_id", evt.Info.ID,
		"message_type", evt.Info.Type,
	)

	// Determine if this is an outgoing message
	isOutgoing := evt.Info.IsFromMe

	// Call the message handler
	if c.onMessage != nil {
		c.onMessage(otherParty.User, evt.Info.Timestamp, isOutgoing, messageText)
	}
}

func resolveOtherPartyJID(info types.MessageInfo) types.JID {
	if info.IsFromMe {
		// For outgoing messages, use the recipient
		if !info.RecipientAlt.IsEmpty() {
			return info.RecipientAlt.ToNonAD()
		}
		return info.Chat.ToNonAD()
	}

	// For incoming messages, use the sender directly
	if !info.Sender.IsEmpty() {
		return info.Sender.ToNonAD()
	}
	if !info.SenderAlt.IsEmpty() {
		return info.SenderAlt.ToNonAD()
	}
	return info.Chat.ToNonAD()
}

func extractMessageText(evt *events.Message) string {
	if evt == nil {
		return ""
	}

	messageEvent := evt.UnwrapRaw()
	if messageEvent == nil || messageEvent.Message == nil {
		return ""
	}

	message := messageEvent.Message
	if text := strings.TrimSpace(message.GetConversation()); text != "" {
		return text
	}

	if extended := message.GetExtendedTextMessage(); extended != nil {
		if text := strings.TrimSpace(extended.GetText()); text != "" {
			return text
		}
	}

	if image := message.GetImageMessage(); image != nil {
		if caption := strings.TrimSpace(image.GetCaption()); caption != "" {
			return "<image> " + caption
		}
	}

	return ""
}

// IsConnected returns true if WhatsApp is connected
func (c *Client) IsConnected() bool {
	return c.GetStatus() == StatusConnected
}
