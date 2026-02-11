// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package logging

import "testing"

func TestLoggerInitializers(t *testing.T) {
	t.Parallel()

	Init()

	if l := Logger(SourceApp); l == nil {
		t.Fatal("Logger returned nil")
	}

	if l := StdLogger(SourceWeb); l == nil {
		t.Fatal("StdLogger returned nil")
	}
}
