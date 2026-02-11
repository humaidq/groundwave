/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import "errors"

var (
	errMissingDate               = errors.New("missing date")
	errInvalidDate               = errors.New("invalid date")
	errYearOutOfRange            = errors.New("year out of range")
	errMonthOutOfRange           = errors.New("month out of range")
	errDayOutOfRange             = errors.New("day out of range")
	errHourOutOfRange            = errors.New("hour out of range")
	errMinuteOutOfRange          = errors.New("minute out of range")
	errSessionUserMissing        = errors.New("session user missing")
	errWebAuthnRPIDRequired      = errors.New("WEBAUTHN_RP_ID is required")
	errWebAuthnRPOriginsRequired = errors.New("WEBAUTHN_RP_ORIGINS is required")
	errSetupUserMissing          = errors.New("setup user missing")
	errInvalidSetupUser          = errors.New("invalid setup user")
	errDisplayNameMissing        = errors.New("display name missing")
	errRegistrationUserMissing   = errors.New("registration user missing")
)
