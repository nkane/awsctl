package dynamo

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	awsx "github.com/nkane/awsctl/internal/aws"
	"github.com/nkane/awsctl/internal/ui/core"
)

// attrTypes are the selectable scalar key types (S=string, N=number, B=binary).
var attrTypes = []ddbtypes.ScalarAttributeType{
	ddbtypes.ScalarAttributeTypeS,
	ddbtypes.ScalarAttributeTypeN,
	ddbtypes.ScalarAttributeTypeB,
}

// createTableScreen is the PAY_PER_REQUEST create-table form: table name,
// partition key (name + type) and an optional sort key (name + type). It mirrors
// the query screen's focus/selector idiom. Gated via core.ConfirmRequest.
type createTableScreen struct {
	client *awsx.DynamoClient

	nameIn textinput.Model
	pkIn   textinput.Model
	pkType int // index into attrTypes
	skIn   textinput.Model
	skType int
	focus  int // 0 name,1 pk,2 pkType,3 sk,4 skType,5 create

	status string
	errMsg string
	done   bool

	width, height int
}

const ctFocusMax = 6

// newCreateTableScreen builds the create-table form.
func newCreateTableScreen(client *awsx.DynamoClient) *createTableScreen {
	mk := func(ph string) textinput.Model {
		t := textinput.New()
		t.Placeholder = ph
		t.Prompt = "  "
		t.CharLimit = 255
		return t
	}
	name := mk("table name")
	name.Focus()
	return &createTableScreen{
		client: client,
		nameIn: name,
		pkIn:   mk("partition key attribute name"),
		skIn:   mk("sort key attribute name (optional)"),
		focus:  0,
	}
}

func (s *createTableScreen) Init() tea.Cmd { return textinput.Blink }

// inputFocused reports whether a text field (not a selector/button) owns input.
func (s *createTableScreen) inputFocused() bool {
	return s.focus == 0 || s.focus == 1 || s.focus == 3
}

func (s *createTableScreen) Update(msg tea.Msg) (core.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case writeDoneMsg:
		if msg.action != actCreateTable {
			return s, nil
		}
		if msg.err != nil {
			s.errMsg = msg.err.Error()
			return s, nil
		}
		s.errMsg = ""
		s.done = true
		s.status = "✓ table created — esc to go back"
		return s, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			s.advance(1)
			return s, nil
		case "shift+tab":
			s.advance(-1)
			return s, nil
		case "left", "right":
			if s.focus == 2 {
				s.pkType = cycle(s.pkType, msg.String() == "right", len(attrTypes))
				return s, nil
			}
			if s.focus == 4 {
				s.skType = cycle(s.skType, msg.String() == "right", len(attrTypes))
				return s, nil
			}
		case "enter":
			if s.focus == 5 {
				return s, s.submit()
			}
			s.advance(1)
			return s, nil
		}
	}

	var cmd tea.Cmd
	switch s.focus {
	case 0:
		s.nameIn, cmd = s.nameIn.Update(msg)
	case 1:
		s.pkIn, cmd = s.pkIn.Update(msg)
	case 3:
		s.skIn, cmd = s.skIn.Update(msg)
	}
	return s, cmd
}

// submit validates the form and emits a gated create-table ConfirmRequest.
func (s *createTableScreen) submit() tea.Cmd {
	name := strings.TrimSpace(s.nameIn.Value())
	pk := strings.TrimSpace(s.pkIn.Value())
	sk := strings.TrimSpace(s.skIn.Value())
	if name == "" {
		s.errMsg = "table name required"
		return nil
	}
	if pk == "" {
		s.errMsg = "partition key name required"
		return nil
	}
	s.errMsg = ""

	in := &dynamodb.CreateTableInput{
		TableName:   &name,
		BillingMode: ddbtypes.BillingModePayPerRequest,
		AttributeDefinitions: []ddbtypes.AttributeDefinition{
			{AttributeName: &pk, AttributeType: attrTypes[s.pkType]},
		},
		KeySchema: []ddbtypes.KeySchemaElement{
			{AttributeName: &pk, KeyType: ddbtypes.KeyTypeHash},
		},
	}
	keyDesc := fmt.Sprintf("pk=%s (%s)", pk, attrTypes[s.pkType])
	if sk != "" {
		in.AttributeDefinitions = append(in.AttributeDefinitions,
			ddbtypes.AttributeDefinition{AttributeName: &sk, AttributeType: attrTypes[s.skType]})
		in.KeySchema = append(in.KeySchema,
			ddbtypes.KeySchemaElement{AttributeName: &sk, KeyType: ddbtypes.KeyTypeRange})
		keyDesc += fmt.Sprintf(", sk=%s (%s)", sk, attrTypes[s.skType])
	}
	body := fmt.Sprintf("Create PAY_PER_REQUEST table %q with %s?", name, keyDesc)
	return core.ConfirmRequest("Create table", body, actCreateTable, name, createTableCmd(s.client, in, name))
}

func (s *createTableScreen) advance(delta int) {
	s.focus = (s.focus + delta + ctFocusMax) % ctFocusMax
	// Skip sort-key type selector when no sort key name is set? Keep reachable —
	// the type only applies when a name is entered, validated at submit.
	s.applyFocus()
}

func (s *createTableScreen) applyFocus() {
	s.nameIn.Blur()
	s.pkIn.Blur()
	s.skIn.Blur()
	switch s.focus {
	case 0:
		s.nameIn.Focus()
	case 1:
		s.pkIn.Focus()
	case 3:
		s.skIn.Focus()
	}
}

func (s *createTableScreen) View() string {
	title := lipgloss.NewStyle().Bold(true).Render("create table")
	hl := func(active bool, str string) string {
		if active {
			return lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true).Render("▸ " + str)
		}
		return "  " + str
	}
	var b strings.Builder
	b.WriteString(title + "\n")
	b.WriteString(hl(s.focus == 0, "table name:") + "\n  " + s.nameIn.View() + "\n")
	b.WriteString(hl(s.focus == 1, "partition key:") + "\n  " + s.pkIn.View() + "\n")
	b.WriteString(hl(s.focus == 2, fmt.Sprintf("pk type: ◀ %s ▶", attrTypes[s.pkType])) + "\n")
	b.WriteString(hl(s.focus == 3, "sort key (optional):") + "\n  " + s.skIn.View() + "\n")
	b.WriteString(hl(s.focus == 4, fmt.Sprintf("sk type: ◀ %s ▶", attrTypes[s.skType])) + "\n")
	b.WriteString(hl(s.focus == 5, "[ create table ]") + "\n")

	if s.errMsg != "" {
		b.WriteString(errStyle.Render("error: "+s.errMsg) + "\n")
	} else if s.status != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(s.status) + "\n")
	}
	b.WriteString(faint("tab focus · ←/→ choose type · enter next/create · esc cancel"))
	return b.String()
}

func (s *createTableScreen) SetSize(w, h int) {
	s.width, s.height = w, h
	s.nameIn.Width = w - 6
	s.pkIn.Width = w - 6
	s.skIn.Width = w - 6
}

func (s *createTableScreen) Title() string { return "create table" }
func (s *createTableScreen) KeyHints() []key.Binding {
	return []key.Binding{core.Hint("tab", "focus"), core.Hint("enter", "create"), core.Hint("esc", "cancel")}
}

// CapturesInput keeps the form owning every key while open.
func (s *createTableScreen) CapturesInput() bool { return true }

// cycle moves idx forward/backward within [0,n).
func cycle(idx int, forward bool, n int) int {
	if forward {
		return (idx + 1) % n
	}
	return (idx - 1 + n) % n
}
