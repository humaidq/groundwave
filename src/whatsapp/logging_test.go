package whatsapp

import "testing"

func TestNewWALogger(t *testing.T) {
	t.Parallel()

	base := newWALogger("pairing")
	wa, ok := base.(*waLogger)
	if !ok {
		t.Fatalf("unexpected logger type: %T", base)
	}
	if wa.module != "pairing" {
		t.Fatalf("unexpected module name: %q", wa.module)
	}
}

func TestWALoggerSub(t *testing.T) {
	t.Parallel()

	root := &waLogger{base: logger, module: ""}
	sub := root.Sub("transport")
	transport, ok := sub.(*waLogger)
	if !ok {
		t.Fatalf("unexpected sub logger type: %T", sub)
	}
	if transport.module != "transport" {
		t.Fatalf("unexpected module path for root sub logger: %q", transport.module)
	}

	nested := transport.Sub("events")
	nestedLogger, ok := nested.(*waLogger)
	if !ok {
		t.Fatalf("unexpected nested logger type: %T", nested)
	}
	if nestedLogger.module != "transport/events" {
		t.Fatalf("unexpected nested module path: %q", nestedLogger.module)
	}
}

func TestWALoggerMethods(t *testing.T) {
	t.Parallel()

	wa := &waLogger{base: logger, module: "test"}

	if got := wa.withModule(); got == nil {
		t.Fatal("withModule returned nil logger")
	}

	// Validate formatting and method plumbing execute without panic.
	wa.Errorf("error %d", 1)
	wa.Warnf("warn %d", 2)
	wa.Infof("info %d", 3)
	wa.Debugf("debug %d", 4)

	root := &waLogger{base: logger, module: ""}
	if got := root.withModule(); got == nil {
		t.Fatal("withModule returned nil logger for empty module")
	}
}
