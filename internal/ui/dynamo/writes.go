package dynamo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	awsx "github.com/nkane/awsctl/internal/aws"
	"github.com/nkane/awsctl/internal/ui/core"
)

// writeDoneMsg is the single result message for every Dynamo mutation. The
// action field mirrors the audit label (e.g. "dynamo.putItem") so the screen
// that emitted the mutation — and any parent list/scan/describe screen that
// should refresh — can react to the broadcast without a per-mutation type.
type writeDoneMsg struct {
	action string // "dynamo.putItem", "dynamo.deleteItem", …
	target string // table name (or statement, for partiql)
	n      int    // affected count, for batch writes
	err    error
}

// Audit action labels (also used to discriminate writeDoneMsg).
const (
	actPutItem      = "dynamo.putItem"
	actDeleteItem   = "dynamo.deleteItem"
	actBatchWrite   = "dynamo.batchWrite"
	actCreateTable  = "dynamo.createTable"
	actDeleteTable  = "dynamo.deleteTable"
	actPartiQLWrite = "dynamo.partiqlWrite"
)

const writeTimeout = 30 * time.Second

// ---- mutation commands (each is the Run passed to core.ConfirmRequest) ----

func putItemCmd(client *awsx.DynamoClient, table string, item map[string]ddbtypes.AttributeValue) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return writeDoneMsg{action: actPutItem, target: table, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		defer cancel()
		return writeDoneMsg{action: actPutItem, target: table, err: client.PutItem(ctx, table, item)}
	}
}

func deleteItemCmd(client *awsx.DynamoClient, table string, key map[string]ddbtypes.AttributeValue) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return writeDoneMsg{action: actDeleteItem, target: table, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		defer cancel()
		return writeDoneMsg{action: actDeleteItem, target: table, err: client.DeleteItem(ctx, table, key)}
	}
}

func batchWriteCmd(client *awsx.DynamoClient, table string, puts, deletes []map[string]ddbtypes.AttributeValue) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return writeDoneMsg{action: actBatchWrite, target: table, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*writeTimeout)
		defer cancel()
		err := client.BatchWriteItems(ctx, table, puts, deletes)
		return writeDoneMsg{action: actBatchWrite, target: table, n: len(puts) + len(deletes), err: err}
	}
}

func createTableCmd(client *awsx.DynamoClient, in *dynamodb.CreateTableInput, name string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return writeDoneMsg{action: actCreateTable, target: name, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if err := client.CreateTable(ctx, in); err != nil {
			return writeDoneMsg{action: actCreateTable, target: name, err: err}
		}
		return writeDoneMsg{action: actCreateTable, target: name, err: client.WaitTableActive(ctx, name)}
	}
}

func deleteTableCmd(client *awsx.DynamoClient, name string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return writeDoneMsg{action: actDeleteTable, target: name, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		defer cancel()
		return writeDoneMsg{action: actDeleteTable, target: name, err: client.DeleteTable(ctx, name)}
	}
}

func partiqlWriteCmd(client *awsx.DynamoClient, stmt string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return writeDoneMsg{action: actPartiQLWrite, target: stmt, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		defer cancel()
		_, err := client.PartiQL(ctx, stmt)
		return writeDoneMsg{action: actPartiQLWrite, target: stmt, err: err}
	}
}

// ---- JSON <-> AttributeValue (plain JSON, no DynamoDB wire-format) ----

// jsonToItem parses a plain JSON object into an AttributeValue item map.
// Numbers are preserved exactly (json.Number); sets and binary are not
// expressible in plain JSON in v1 — use PartiQL for those.
func jsonToItem(s string) (map[string]ddbtypes.AttributeValue, error) {
	raw, err := decodeJSONObject(s)
	if err != nil {
		return nil, err
	}
	return mapToAV(raw)
}

func decodeJSONObject(s string) (map[string]interface{}, error) {
	dec := json.NewDecoder(strings.NewReader(strings.TrimSpace(s)))
	dec.UseNumber()
	var raw map[string]interface{}
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("invalid JSON object: %w", err)
	}
	return raw, nil
}

func mapToAV(raw map[string]interface{}) (map[string]ddbtypes.AttributeValue, error) {
	out := make(map[string]ddbtypes.AttributeValue, len(raw))
	for k, v := range raw {
		av, err := jsonToAV(v)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", k, err)
		}
		out[k] = av
	}
	return out, nil
}

func jsonToAV(v interface{}) (ddbtypes.AttributeValue, error) {
	switch x := v.(type) {
	case nil:
		return &ddbtypes.AttributeValueMemberNULL{Value: true}, nil
	case bool:
		return &ddbtypes.AttributeValueMemberBOOL{Value: x}, nil
	case string:
		return &ddbtypes.AttributeValueMemberS{Value: x}, nil
	case json.Number:
		return &ddbtypes.AttributeValueMemberN{Value: x.String()}, nil
	case []interface{}:
		l := make([]ddbtypes.AttributeValue, len(x))
		for i, e := range x {
			av, err := jsonToAV(e)
			if err != nil {
				return nil, err
			}
			l[i] = av
		}
		return &ddbtypes.AttributeValueMemberL{Value: l}, nil
	case map[string]interface{}:
		m, err := mapToAV(x)
		if err != nil {
			return nil, err
		}
		return &ddbtypes.AttributeValueMemberM{Value: m}, nil
	}
	return nil, fmt.Errorf("unsupported JSON value of type %T", v)
}

// ---- reusable textarea editor screen ----

// submitFn validates the editor text and returns the confirm metadata plus the
// Run command to gate behind core.ConfirmRequest. An error short-circuits the
// submit and is shown in the editor without opening the modal.
type submitFn func(text string) (confirmTitle, confirmBody, target string, run tea.Cmd, err error)

// editorScreen is a generic textarea-backed write editor (edit item, batch
// write, PartiQL write). It captures input, validates on ctrl+s, then emits a
// gated ConfirmRequest; on the broadcast writeDoneMsg it shows success/error.
type editorScreen struct {
	header string // header line, e.g. "edit item: orders"
	crumb  string // breadcrumb Title()
	hint   string // footer key hints
	action string // audit action (also matches writeDoneMsg.action)

	ta     textarea.Model
	submit submitFn

	status string
	errMsg string
	done   bool

	width, height int
}

func newEditorScreen(header, crumb, hint, action, placeholder, seed string, sf submitFn) *editorScreen {
	ta := textarea.New()
	ta.Placeholder = placeholder
	ta.ShowLineNumbers = true
	ta.CharLimit = 0
	if seed != "" {
		ta.SetValue(seed)
	}
	ta.Focus()
	return &editorScreen{header: header, crumb: crumb, hint: hint, action: action, ta: ta, submit: sf}
}

func (s *editorScreen) Init() tea.Cmd { return textarea.Blink }

func (s *editorScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case writeDoneMsg:
		if msg.action != s.action {
			return s, nil
		}
		if msg.err != nil {
			s.errMsg = msg.err.Error()
			return s, nil
		}
		s.errMsg = ""
		s.done = true
		s.status = "✓ applied — esc to go back"
		return s, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+s" {
			title, body, target, run, err := s.submit(s.ta.Value())
			if err != nil {
				s.errMsg = err.Error()
				return s, nil
			}
			s.errMsg = ""
			s.status = ""
			return s, core.ConfirmRequest(title, body, s.action, target, run)
		}
	}
	var cmd tea.Cmd
	s.ta, cmd = s.ta.Update(msg)
	return s, cmd
}

func (s *editorScreen) View() string {
	title := lipgloss.NewStyle().Bold(true).Render(s.header)
	status := ""
	if s.errMsg != "" {
		status = errStyle.Render("error: " + s.errMsg)
	} else if s.status != "" {
		status = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(s.status)
	}
	footer := faint(s.hint)
	parts := []string{title, s.ta.View()}
	if status != "" {
		parts = append(parts, status)
	}
	parts = append(parts, footer)
	return strings.Join(parts, "\n")
}

func (s *editorScreen) SetSize(w, h int) {
	s.width, s.height = w, h
	body := h - 3
	if body < 4 {
		body = 4
	}
	s.ta.SetWidth(w)
	s.ta.SetHeight(body)
}

func (s *editorScreen) Title() string { return s.crumb }
func (s *editorScreen) KeyHints() []key.Binding {
	return []key.Binding{core.Hint("ctrl+s", "submit"), core.Hint("esc", "cancel")}
}

// CapturesInput keeps the textarea owning every key (so "1"/"2"/"q" are literal).
func (s *editorScreen) CapturesInput() bool { return true }

// ---- editor constructors for each input-requiring mutation ----

// newEditItemScreen builds the put/update editor for one table. seed is the
// current item rendered as JSON when editing, or a "{}" template for a new put.
func newEditItemScreen(client *awsx.DynamoClient, table, seed string) *editorScreen {
	return newEditorScreen(
		"put item: "+table,
		"put item",
		"ctrl+s submit · esc cancel",
		actPutItem,
		`{ "pk": "…", "sk": "…" }`,
		seed,
		func(text string) (string, string, string, tea.Cmd, error) {
			item, err := jsonToItem(text)
			if err != nil {
				return "", "", "", nil, err
			}
			if len(item) == 0 {
				return "", "", "", nil, fmt.Errorf("item is empty")
			}
			body := fmt.Sprintf("Put %d-attribute item into %s? Existing item with the same key is overwritten.", len(item), table)
			return "Put item", body, table, putItemCmd(client, table, item), nil
		},
	)
}

// batchPayload is the JSON shape the batch-write editor accepts.
type batchPayload struct {
	Puts    []map[string]interface{} `json:"puts"`
	Deletes []map[string]interface{} `json:"deletes"`
}

const batchSeed = `{
  "puts": [
    { "pk": "…", "sk": "…" }
  ],
  "deletes": [
    { "pk": "…", "sk": "…" }
  ]
}`

// newBatchScreen builds the batch put/delete editor for one table.
func newBatchScreen(client *awsx.DynamoClient, table string) *editorScreen {
	return newEditorScreen(
		"batch write: "+table,
		"batch write",
		"ctrl+s submit · esc cancel",
		actBatchWrite,
		batchSeed,
		batchSeed,
		func(text string) (string, string, string, tea.Cmd, error) {
			dec := json.NewDecoder(strings.NewReader(strings.TrimSpace(text)))
			dec.UseNumber()
			var p batchPayload
			if err := dec.Decode(&p); err != nil {
				return "", "", "", nil, fmt.Errorf("invalid batch JSON: %w", err)
			}
			puts, err := convertMaps(p.Puts)
			if err != nil {
				return "", "", "", nil, fmt.Errorf("puts: %w", err)
			}
			dels, err := convertMaps(p.Deletes)
			if err != nil {
				return "", "", "", nil, fmt.Errorf("deletes: %w", err)
			}
			if len(puts)+len(dels) == 0 {
				return "", "", "", nil, fmt.Errorf("nothing to write: both puts and deletes are empty")
			}
			body := fmt.Sprintf("Batch-write %d put(s) and %d delete(s) to %s?", len(puts), len(dels), table)
			return "Batch write", body, table, batchWriteCmd(client, table, puts, dels), nil
		},
	)
}

func convertMaps(in []map[string]interface{}) ([]map[string]ddbtypes.AttributeValue, error) {
	out := make([]map[string]ddbtypes.AttributeValue, 0, len(in))
	for i, m := range in {
		av, err := mapToAV(m)
		if err != nil {
			return nil, fmt.Errorf("[%d]: %w", i, err)
		}
		out = append(out, av)
	}
	return out, nil
}

// newPartiqlScreen builds the PartiQL write-statement editor. seed prefills an
// INSERT template scoped to the table.
func newPartiqlScreen(client *awsx.DynamoClient, table string) *editorScreen {
	seed := fmt.Sprintf("INSERT INTO \"%s\" VALUE { 'pk': '…', 'sk': '…' }", table)
	return newEditorScreen(
		"partiql write: "+table,
		"partiql write",
		"ctrl+s submit · esc cancel",
		actPartiQLWrite,
		"INSERT INTO … / UPDATE … / DELETE …",
		seed,
		func(text string) (string, string, string, tea.Cmd, error) {
			stmt := strings.TrimSpace(text)
			if stmt == "" {
				return "", "", "", nil, fmt.Errorf("statement is empty")
			}
			disp := stmt
			if len(disp) > 80 {
				disp = disp[:77] + "…"
			}
			return "PartiQL write", "Execute statement?\n\n" + disp, stmt, partiqlWriteCmd(client, stmt), nil
		},
	)
}
