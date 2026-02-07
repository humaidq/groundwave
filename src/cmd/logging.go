/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package cmd

import "github.com/humaidq/groundwave/logging"

var appLogger = logging.Logger(logging.SourceApp)
var whatsappLogger = logging.Logger(logging.SourceWhatsApp)
var requestLogger = logging.Logger(logging.SourceWebRequest)
var requestStdLogger = logging.StdLogger(logging.SourceWebRequest)
