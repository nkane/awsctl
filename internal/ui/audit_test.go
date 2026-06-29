package ui

import (
	"context"
	"io"
	"log/slog"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// captureHandler records every slog.Record it handles for later inspection.
type captureHandler struct {
	records []slog.Record
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}
func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(string) slog.Handler      { return h }

// auditGroup returns the attrs nested inside the "audit" group of r, or nil if
// there is no such group.
func auditGroup(r slog.Record) []slog.Attr {
	var group []slog.Attr
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "audit" && a.Value.Kind() == slog.KindGroup {
			group = a.Value.Group()
			return false
		}
		return true
	})
	return group
}

// groupStr looks up a string attr by key inside a slice of attrs.
func groupStr(attrs []slog.Attr, key string) (string, bool) {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value.String(), true
		}
	}
	return "", false
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestAuditLogGroup asserts auditLog emits one INFO record whose attrs contain a
// group named "audit" with action/target/result inside it (not at top level),
// including any appended extra attrs.
func TestAuditLogGroup(t *testing.T) {
	h := &captureHandler{}
	logger := slog.New(h)

	auditLog(logger, "delete", "func:foo", "confirmed", slog.String("region", "us-east-1"))

	if len(h.records) != 1 {
		t.Fatalf("records = %d, want 1", len(h.records))
	}
	r := h.records[0]
	if r.Level != slog.LevelInfo {
		t.Fatalf("level = %v, want INFO", r.Level)
	}
	if r.Message != "audit" {
		t.Fatalf("message = %q, want %q", r.Message, "audit")
	}

	// action/target/result must NOT appear at the top level.
	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "action", "target", "result", "region":
			t.Fatalf("attr %q leaked to top level; want inside audit group", a.Key)
		}
		return true
	})

	group := auditGroup(r)
	if group == nil {
		t.Fatal("no audit group found")
	}
	for k, want := range map[string]string{
		"action": "delete",
		"target": "func:foo",
		"result": "confirmed",
		"region": "us-east-1",
	} {
		got, ok := groupStr(group, k)
		if !ok {
			t.Fatalf("audit group missing %q", k)
		}
		if got != want {
			t.Fatalf("audit group %q = %q, want %q", k, got, want)
		}
	}
}

// TestAuditLogNilLogger asserts a nil logger is a no-op (no panic).
func TestAuditLogNilLogger(t *testing.T) {
	auditLog(nil, "delete", "func:foo", "confirmed")
}

// TestRequestConfirmReadOnly asserts that without --unsafe the gate refuses:
// the modal stays closed and lastErr is set.
func TestRequestConfirmReadOnly(t *testing.T) {
	app := NewApp(Options{Unsafe: false, Logger: discardLogger()})
	ran := false
	app.requestConfirm("Delete?", "delete foo", "delete", "func:foo", func() tea.Msg {
		ran = true
		return nil
	})

	if app.confirm.Active() {
		t.Fatal("confirm modal opened in read-only mode")
	}
	if app.lastErr == "" {
		t.Fatal("lastErr not set in read-only refusal")
	}
	if ran {
		t.Fatal("pending command ran in read-only mode")
	}
	if app.pendingCmd != nil {
		t.Fatal("pending command stored in read-only mode")
	}
}

// TestRequestConfirmUnsafeConfirm asserts that with --unsafe the gate opens the
// modal, and a 'y' key yields the pending cmd plus a "confirmed" audit record.
func TestRequestConfirmUnsafeConfirm(t *testing.T) {
	h := &captureHandler{}
	app := NewApp(Options{Unsafe: true, Logger: slog.New(h)})
	app.requestConfirm("Delete?", "delete foo", "delete", "func:foo", func() tea.Msg { return nil })

	if !app.confirm.Active() {
		t.Fatal("confirm modal not open after requestConfirm in unsafe mode")
	}

	m, cmd := app.Update(runes("y"))
	if cmd == nil {
		t.Fatal("ConfirmYes did not return the pending command")
	}
	if m.(App).confirm.Active() {
		t.Fatal("modal still active after confirm")
	}
	if m.(App).pendingCmd != nil {
		t.Fatal("pending command not cleared after confirm")
	}

	if len(h.records) != 1 {
		t.Fatalf("records = %d, want 1", len(h.records))
	}
	group := auditGroup(h.records[0])
	if res, _ := groupStr(group, "result"); res != "confirmed" {
		t.Fatalf("audit result = %q, want %q", res, "confirmed")
	}
	if act, _ := groupStr(group, "action"); act != "delete" {
		t.Fatalf("audit action = %q, want %q", act, "delete")
	}
}

// TestRequestConfirmUnsafeCancel asserts that 'n' and 'esc' both cancel the
// modal and emit a "cancelled" audit record while clearing the pending cmd.
func TestRequestConfirmUnsafeCancel(t *testing.T) {
	for _, key := range []string{"n", "esc"} {
		h := &captureHandler{}
		app := NewApp(Options{Unsafe: true, Logger: slog.New(h)})
		app.requestConfirm("Delete?", "delete foo", "delete", "func:foo", func() tea.Msg { return nil })

		var msg tea.KeyMsg
		if key == "esc" {
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		} else {
			msg = runes(key)
		}
		m, cmd := app.Update(msg)
		if cmd != nil {
			t.Fatalf("%q: expected nil cmd on cancel", key)
		}
		if m.(App).confirm.Active() {
			t.Fatalf("%q: modal still active after cancel", key)
		}
		if m.(App).pendingCmd != nil {
			t.Fatalf("%q: pending command not cleared after cancel", key)
		}
		if len(h.records) != 1 {
			t.Fatalf("%q: records = %d, want 1", key, len(h.records))
		}
		if res, _ := groupStr(auditGroup(h.records[0]), "result"); res != "cancelled" {
			t.Fatalf("%q: audit result = %q, want %q", key, res, "cancelled")
		}
	}
}
