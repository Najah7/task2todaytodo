package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/port/adapter"
	"github.com/Najah7/task2todaytodo/internal/settings"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

func TestClosePassesBoundedLiveContextAndJoinsErrors(t *testing.T) {
	originalErr := errors.New("server failed")
	shutdownErr := errors.New("worker shutdown failed")
	result := originalErr
	called := false

	Close(func(ctx context.Context) error {
		called = true
		if err := ctx.Err(); err != nil {
			t.Errorf("close context already done: %v", err)
		}
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("close context has no deadline")
		}
		if remaining := time.Until(deadline); remaining <= 0 || remaining > 10*time.Second {
			t.Errorf("close context deadline remaining = %s, want (0, 10s]", remaining)
		}
		return shutdownErr
	}, &result)

	if !called {
		t.Fatal("close callback was not called")
	}
	if !errors.Is(result, originalErr) || !errors.Is(result, shutdownErr) {
		t.Errorf("Close() result = %v, want both original and shutdown errors", result)
	}
}

func TestCloseSuccessPreservesOriginalError(t *testing.T) {
	originalErr := errors.New("server failed")
	result := originalErr
	Close(func(context.Context) error { return nil }, &result)
	if result != originalErr {
		t.Errorf("Close() result = %v, want original error %v", result, originalErr)
	}
}

func TestNewRejectsInvalidDatabaseSettingsWithoutInitializingApplication(t *testing.T) {
	app, closeApp, err := New(settings.DatabaseSettings{SSLMode: "invalid"}, adapter.NewIDIssuer(), telemetry.Nop())
	if err == nil {
		t.Fatal("New() error = nil, want invalid database settings error")
	}
	if app != nil {
		t.Errorf("New() application = %#v, want nil", app)
	}
	if closeApp != nil {
		t.Error("New() close callback is not nil")
	}
}
