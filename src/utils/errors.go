/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package utils

import "errors"

var (
	errMissingRequiredADIFFields = errors.New("missing required fields (CALL or QSO_DATE)")
	errInvalidDateTimeFormat     = errors.New("invalid date/time format")
	errNoIDPropertyFound         = errors.New("no ID property found in content")
	errInvalidUUIDFormat         = errors.New("invalid UUID format")
	errUUIDLengthOutOfBounds     = errors.New("UUID length out of bounds")
)
