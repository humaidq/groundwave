/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package logging

import (
	stdlog "log"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

// Log source tags used in structured logger contexts.
const (
	SourceApp        = "app"
	SourceWeb        = "web"
	SourceWebRequest = "web_request"
	SourceDB         = "db"
	SourceWhatsApp   = "whatsapp"
)

var (
	initOnce   sync.Once
	baseLogger *log.Logger
)

// Init configures the base logger and stdlib log output.
func Init() {
	initOnce.Do(func() {
		baseLogger = log.NewWithOptions(os.Stdout, log.Options{
			TimeFunction:    log.NowUTC,
			TimeFormat:      time.RFC3339Nano,
			Level:           log.DebugLevel,
			ReportTimestamp: true,
			Formatter:       log.LogfmtFormatter,
		})

		stdLogger := baseLogger.With("source", SourceApp).StandardLog(log.StandardLogOptions{ForceLevel: log.InfoLevel})

		stdlog.SetFlags(0)
		stdlog.SetOutput(stdLogger.Writer())
	})
}

// Logger returns a logfmt logger tagged with the provided source.
func Logger(source string) *log.Logger {
	Init()
	return baseLogger.With("source", source)
}

// StdLogger returns a stdlib logger that writes logfmt output with a source.
func StdLogger(source string) *stdlog.Logger {
	Init()
	return baseLogger.With("source", source).StandardLog(log.StandardLogOptions{ForceLevel: log.InfoLevel})
}
