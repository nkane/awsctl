//go:build integration

// Integration smoke test for the navigation-stack refactor (#63). Drives the
// real App through Update against a seeded LocalStack — no PTY — asserting the
// load -> list -> drill(push) -> esc(pop) flow and breadcrumb rendering.
//
//	docker run ... localstack/localstack:3.8   (see docker-compose.localstack.yml)
//	./scripts/seed-localstack.sh   (or awslocal seeding)
//	AWSCTL_ENDPOINT_URL=http://localhost:4566 \
//	AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_REGION=us-east-1 \
//	  go test -tags=integration ./internal/ui/...
package ui

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// drain runs a tea.Cmd (and any batched sub-commands) to completion, feeding
// every resulting message back through the model. Synchronous: good enough for
// a deterministic smoke test against local fixtures.
func drain(t *testing.T, m tea.Model, cmd tea.Cmd) tea.Model {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		if time.Now().After(deadline) {
			t.Fatal("drain timed out")
		}
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg := c()
		queue = append(queue, flatten(msg)...)
		var next tea.Cmd
		m, next = m.Update(msg)
		if next != nil {
			queue = append(queue, next)
		}
	}
	return m
}

// flatten unwraps tea.BatchMsg into individual messages' commands.
func flatten(msg tea.Msg) []tea.Cmd {
	if batch, ok := msg.(tea.BatchMsg); ok {
		return []tea.Cmd(batch)
	}
	return nil
}

func mkKey(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func TestSmokeNavStack(t *testing.T) {
	if os.Getenv("AWSCTL_ENDPOINT_URL") == "" {
		t.Skip("set AWSCTL_ENDPOINT_URL to run the LocalStack smoke test")
	}

	app := NewApp(Options{
		Region: "us-east-1",
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	var m tea.Model = app

	// Lay out, then load config + first fetches.
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = drain(t, m, m.(App).Init())

	// Default mode is Lambda — the function list should have rendered.
	if v := m.(App).View(); !strings.Contains(v, "demo-hello") {
		t.Fatalf("lambda list did not render seeded function; view:\n%s", v)
	}

	// Switch to Dynamo mode ("2"): table list should render.
	m, _ = m.Update(mkKey("2"))
	m = drain(t, m, nil)
	if v := m.(App).View(); !strings.Contains(v, "demo-orders") {
		t.Fatalf("dynamo list did not render seeded table; view:\n%s", v)
	}
	if d := m.(App).dstack.Depth(); d != 1 {
		t.Fatalf("dynamo stack should be at root, depth=%d", d)
	}

	// Drill into the selected table (enter -> describe). Stack should grow and a
	// breadcrumb trail should appear.
	m2, cmd := m.Update(mkKey("enter"))
	m = drain(t, m2, cmd)
	if d := m.(App).dstack.Depth(); d != 2 {
		t.Fatalf("after enter, dynamo stack depth=%d, want 2", d)
	}
	if v := m.(App).View(); !strings.Contains(v, "tables") || !strings.Contains(v, "describe") {
		t.Fatalf("breadcrumb trail missing 'tables > describe'; view:\n%s", v)
	}

	// esc pops back to the table list.
	m, _ = m.Update(mkKey("esc"))
	if d := m.(App).dstack.Depth(); d != 1 {
		t.Fatalf("after esc, dynamo stack depth=%d, want 1 (root)", d)
	}

	// esc at root is a no-op (stays on the list).
	m, _ = m.Update(mkKey("esc"))
	if d := m.(App).dstack.Depth(); d != 1 {
		t.Fatalf("esc at root should be a no-op, depth=%d", d)
	}
}
