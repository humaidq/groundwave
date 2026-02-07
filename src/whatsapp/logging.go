/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package whatsapp

import (
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/humaidq/groundwave/logging"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var logger = logging.Logger(logging.SourceWhatsApp)

type waLogger struct {
	base   *log.Logger
	module string
}

func newWALogger(module string) waLog.Logger {
	return &waLogger{base: logger, module: module}
}

func (w *waLogger) withModule() *log.Logger {
	if w.module == "" {
		return w.base
	}
	return w.base.With("module", w.module)
}

func (w *waLogger) Errorf(msg string, args ...interface{}) {
	w.withModule().Error(fmt.Sprintf(msg, args...))
}
func (w *waLogger) Warnf(msg string, args ...interface{}) {
	w.withModule().Warn(fmt.Sprintf(msg, args...))
}
func (w *waLogger) Infof(msg string, args ...interface{}) {
	w.withModule().Info(fmt.Sprintf(msg, args...))
}
func (w *waLogger) Debugf(msg string, args ...interface{}) {
	w.withModule().Debug(fmt.Sprintf(msg, args...))
}

func (w *waLogger) Sub(module string) waLog.Logger {
	if w.module == "" {
		return &waLogger{base: w.base, module: module}
	}
	return &waLogger{base: w.base, module: w.module + "/" + module}
}
