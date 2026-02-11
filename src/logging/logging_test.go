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
