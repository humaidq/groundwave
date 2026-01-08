/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"encoding/gob"

	"github.com/flamego/session"
)

// FlashType represents the type of flash message
type FlashType string

const (
	FlashError   FlashType = "error"
	FlashSuccess FlashType = "success"
	FlashWarning FlashType = "warning"
	FlashInfo    FlashType = "info"
)

// FlashMessage represents a flash message to be displayed to the user
type FlashMessage struct {
	Type    FlashType
	Message string
}

func init() {
	// Register FlashMessage with gob for session serialization
	gob.Register(FlashMessage{})
}

// SetErrorFlash sets an error flash message in the session
func SetErrorFlash(s session.Session, message string) {
	s.SetFlash(FlashMessage{
		Type:    FlashError,
		Message: message,
	})
}

// SetSuccessFlash sets a success flash message in the session
func SetSuccessFlash(s session.Session, message string) {
	s.SetFlash(FlashMessage{
		Type:    FlashSuccess,
		Message: message,
	})
}

// SetWarningFlash sets a warning flash message in the session
func SetWarningFlash(s session.Session, message string) {
	s.SetFlash(FlashMessage{
		Type:    FlashWarning,
		Message: message,
	})
}

// SetInfoFlash sets an info flash message in the session
func SetInfoFlash(s session.Session, message string) {
	s.SetFlash(FlashMessage{
		Type:    FlashInfo,
		Message: message,
	})
}
