/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package whatsapp

import "errors"

var (
	errNoExistingSessionToReconnect = errors.New("no existing session to reconnect")
	errNoDeviceStoreLoader          = errors.New("device store loader is not configured")
	errNoDeviceStoreContainer       = errors.New("whatsapp SQL store container is unavailable")
)
