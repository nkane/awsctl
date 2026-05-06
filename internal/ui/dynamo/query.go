package dynamo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	awsx "github.com/nkane/awsctl/internal/aws"
)

// queryDescribeMsg carries the DescribeTable result used to seed key schema.
type queryDescribeMsg struct {
	table string
	desc  *ddbtypes.TableDescription
	err   error
}

// queryPageMsg carries one page of query results.
type queryPageMsg struct {
	table string
	res   *awsx.QueryResult
	err   error
}

// indexChoice is one selectable index (base table or a GSI/LSI).
type indexChoice struct {
	name string // "" = base table
	pk   string
	pkT  string
	sk   string
	skT  string
}

func (c indexChoice) label() string {
	if c.name == "" {
		return "<base table>"
	}
	return c.name
}

// QueryModel is the key-condition query screen.
type QueryModel struct {
	client   *awsx.DynamoClient
	table    string
	desc     *ddbtypes.TableDescription
	indexes  []indexChoice
	idxSel   int
	pkInput  textinput.Model
	skOp     int // 0 none, 1 =, 2 BEGINS_WITH, 3 BETWEEN
	skA      textinput.Model
	skB      textinput.Model
	focus    int // 0 idx, 1 pk, 2 sk-op, 3 skA, 4 skB, 5 run
	vp       viewport.Model
	spinner  spinner.Model
	loading  bool
	loadDesc bool
	err      string

	page     int
	totItems int
	lek      map[string]ddbtypes.AttributeValue
	buf      strings.Builder

	width  int
	height int
}

// NewQuery constructs the query screen for a table. Init triggers DescribeTable
// to seed key schema; user fills inputs and presses 'enter' on Run to query.
func NewQuery(client *awsx.DynamoClient, table string) QueryModel {
	pk := textinput.New()
	pk.Placeholder = "partition key value"
	pk.Prompt = "  "
	pk.CharLimit = 256

	skA := textinput.New()
	skA.Placeholder = "sort key value (or low for BETWEEN)"
	skA.Prompt = "  "
	skA.CharLimit = 256

	skB := textinput.New()
	skB.Placeholder = "high (BETWEEN only)"
	skB.Prompt = "  "
	skB.CharLimit = 256

	vp := viewport.New(0, 0)
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return QueryModel{
		client:   client,
		table:    table,
		pkInput:  pk,
		skA:      skA,
		skB:      skB,
		vp:       vp,
		spinner:  sp,
		loadDesc: true,
		focus:    1,
	}
}

// Init kicks off a DescribeTable to seed key schema.
func (m QueryModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, queryDescribeCmd(m.client, m.table), textinput.Blink)
}

// Name returns the table name.
func (m QueryModel) Name() string { return m.table }

// SetSize sizes the viewport (form takes ~10 lines, results take rest).
func (m *QueryModel) SetSize(w, h int) {
	m.width, m.height = w, h
	body := h - 12
	if body < 4 {
		body = 4
	}
	m.vp.Width = w
	m.vp.Height = body
	m.pkInput.Width = w - 6
	m.skA.Width = w - 6
	m.skB.Width = w - 6
}

// Update handles describe load, query results, and form input.
func (m QueryModel) Update(msg tea.Msg) (QueryModel, tea.Cmd) {
	switch msg := msg.(type) {
	case queryDescribeMsg:
		if msg.table != m.table {
			return m, nil
		}
		m.loadDesc = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err = ""
		m.desc = msg.desc
		m.indexes = buildIndexChoices(msg.desc)
		m.applyFocus()
		return m, nil

	case queryPageMsg:
		if msg.table != m.table {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err = ""
		m.page++
		m.totItems += len(msg.res.Items)
		m.lek = msg.res.LastEvaluatedKey
		m.buf.WriteString(renderPage(m.page, msg.res.Items))
		m.vp.SetContent(m.buf.String())
		m.vp.GotoBottom()
		return m, nil

	case spinner.TickMsg:
		if !m.loading && !m.loadDesc {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.advanceFocus(1)
			return m, nil
		case "shift+tab":
			m.advanceFocus(-1)
			return m, nil
		case "left", "right":
			if m.focus == 0 && len(m.indexes) > 0 {
				if msg.String() == "left" {
					m.idxSel = (m.idxSel - 1 + len(m.indexes)) % len(m.indexes)
				} else {
					m.idxSel = (m.idxSel + 1) % len(m.indexes)
				}
				return m, nil
			}
			if m.focus == 2 {
				if msg.String() == "left" {
					m.skOp = (m.skOp - 1 + 4) % 4
				} else {
					m.skOp = (m.skOp + 1) % 4
				}
				return m, nil
			}
		case "enter":
			if m.focus == 5 {
				return m, m.runQuery(false)
			}
			m.advanceFocus(1)
			return m, nil
		case "n":
			if m.focus == 1 || m.focus == 3 || m.focus == 4 {
				break
			}
			if m.lek != nil && !m.loading {
				return m, m.runQuery(true)
			}
			return m, nil
		case "r":
			if m.focus == 1 || m.focus == 3 || m.focus == 4 {
				break
			}
			m.resetResults()
			return m, nil
		}
	}

	// Forward to focused input.
	var cmd tea.Cmd
	switch m.focus {
	case 1:
		m.pkInput, cmd = m.pkInput.Update(msg)
	case 3:
		m.skA, cmd = m.skA.Update(msg)
	case 4:
		m.skB, cmd = m.skB.Update(msg)
	default:
		m.vp, cmd = m.vp.Update(msg)
	}
	return m, cmd
}

// runQuery builds expression values and dispatches Query. paging=true uses
// the existing LEK; otherwise resets results first.
func (m *QueryModel) runQuery(paging bool) tea.Cmd {
	if len(m.indexes) == 0 {
		m.err = "table has no key schema"
		return nil
	}
	idx := m.indexes[m.idxSel]
	pkVal := strings.TrimSpace(m.pkInput.Value())
	if pkVal == "" {
		m.err = "partition key value required"
		return nil
	}
	expr := "#pk = :pk"
	exprNames := map[string]string{"#pk": idx.pk}
	exprVals := map[string]ddbtypes.AttributeValue{":pk": typedAV(idx.pkT, pkVal)}

	if idx.sk != "" && m.skOp != 0 {
		a := strings.TrimSpace(m.skA.Value())
		b := strings.TrimSpace(m.skB.Value())
		exprNames["#sk"] = idx.sk
		switch m.skOp {
		case 1: // =
			if a == "" {
				m.err = "sort key value required"
				return nil
			}
			expr += " AND #sk = :ska"
			exprVals[":ska"] = typedAV(idx.skT, a)
		case 2: // BEGINS_WITH
			if a == "" {
				m.err = "sort key prefix required"
				return nil
			}
			expr += " AND begins_with(#sk, :ska)"
			exprVals[":ska"] = typedAV(idx.skT, a)
		case 3: // BETWEEN
			if a == "" || b == "" {
				m.err = "BETWEEN needs low and high values"
				return nil
			}
			expr += " AND #sk BETWEEN :ska AND :skb"
			exprVals[":ska"] = typedAV(idx.skT, a)
			exprVals[":skb"] = typedAV(idx.skT, b)
		}
	}

	if !paging {
		m.resetResults()
	}
	m.loading = true
	m.err = ""
	in := awsx.QueryInput{
		Table:                  m.table,
		IndexName:              idx.name,
		KeyConditionExpression: expr,
		ExpressionValues:       exprVals,
		Limit:                  50,
	}
	if paging {
		in.ExclusiveStartKey = m.lek
	}
	// ExpressionAttributeNames must be set on the SDK call; QueryInput
	// in awsx does not expose names — query module passes via raw Query.
	return tea.Batch(m.spinner.Tick, queryPageCmdNamed(m.client, in, exprNames))
}

func (m *QueryModel) resetResults() {
	m.page = 0
	m.totItems = 0
	m.lek = nil
	m.buf.Reset()
	m.vp.SetContent("")
}

// advanceFocus moves focus by delta, skipping unavailable inputs.
func (m *QueryModel) advanceFocus(delta int) {
	max := 6
	for i := 0; i < max; i++ {
		m.focus = (m.focus + delta + max) % max
		if m.focusValid() {
			break
		}
	}
	m.applyFocus()
}

// focusValid reports whether current focus index points at a usable field.
func (m QueryModel) focusValid() bool {
	if len(m.indexes) == 0 {
		return m.focus == 5 // only run is reachable
	}
	idx := m.indexes[m.idxSel]
	switch m.focus {
	case 0, 1, 2, 5:
		return true
	case 3:
		return idx.sk != "" && m.skOp != 0
	case 4:
		return idx.sk != "" && m.skOp == 3 // BETWEEN only
	}
	return false
}

// applyFocus sets the textinput Focus state to match m.focus.
func (m *QueryModel) applyFocus() {
	m.pkInput.Blur()
	m.skA.Blur()
	m.skB.Blur()
	switch m.focus {
	case 1:
		m.pkInput.Focus()
	case 3:
		m.skA.Focus()
	case 4:
		m.skB.Focus()
	}
}

// View renders the form + results.
func (m QueryModel) View() string {
	titleSty := lipgloss.NewStyle().Bold(true)
	title := titleSty.Render("query: " + m.table)
	if m.loadDesc {
		return title + "\n" + m.spinner.View() + " loading key schema…"
	}
	if len(m.indexes) == 0 {
		return title + "\n" + errStyle.Render("error: "+m.err)
	}

	idx := m.indexes[m.idxSel]
	skOps := []string{"(none)", "=", "BEGINS_WITH", "BETWEEN"}

	hl := func(active bool, s string) string {
		if active {
			return lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true).Render("▸ " + s)
		}
		return "  " + s
	}

	var b strings.Builder
	b.WriteString(title + "\n")
	b.WriteString(hl(m.focus == 0, fmt.Sprintf("index: ◀ %s ▶  (pk=%s%s)", idx.label(), idx.pk, skSuffix(idx))))
	b.WriteString("\n")
	b.WriteString(hl(m.focus == 1, fmt.Sprintf("pk (%s):", idx.pkT)))
	b.WriteString("\n  " + m.pkInput.View() + "\n")
	if idx.sk != "" {
		b.WriteString(hl(m.focus == 2, fmt.Sprintf("sk op: ◀ %s ▶", skOps[m.skOp])))
		b.WriteString("\n")
		if m.skOp != 0 {
			b.WriteString(hl(m.focus == 3, fmt.Sprintf("sk (%s):", idx.skT)))
			b.WriteString("\n  " + m.skA.View() + "\n")
		}
		if m.skOp == 3 {
			b.WriteString(hl(m.focus == 4, "sk high:"))
			b.WriteString("\n  " + m.skB.View() + "\n")
		}
	}
	b.WriteString(hl(m.focus == 5, "[ run query ]"))
	b.WriteString("\n")

	results := m.vp.View()
	if m.err != "" {
		results = errStyle.Render("error: " + m.err)
	} else if m.loading && m.totItems == 0 {
		results = m.spinner.View() + " querying…"
	} else if m.page == 0 {
		results = faint("(no query run yet — fill fields, tab to [run query], press enter)")
	}

	more := "no more pages"
	if m.lek != nil {
		more = "more available — press n"
	}
	loadHint := ""
	if m.loading && m.totItems > 0 {
		loadHint = " · " + m.spinner.View() + " loading"
	}
	footer := faint(fmt.Sprintf("page %d · items %d · %s%s    tab focus · ←/→ choose · enter run/next-field · n next-page · r reset · esc back",
		m.page, m.totItems, more, loadHint))

	return b.String() + "\n" + results + "\n" + footer
}

// skSuffix renders ", sk=NAME" if the index has a sort key.
func skSuffix(c indexChoice) string {
	if c.sk == "" {
		return ""
	}
	return ", sk=" + c.sk
}

// buildIndexChoices flattens base table + GSIs + LSIs into a selectable list.
func buildIndexChoices(d *ddbtypes.TableDescription) []indexChoice {
	if d == nil {
		return nil
	}
	out := []indexChoice{}
	base := indexChoice{name: ""}
	base.pk, base.sk = pkSk(d.KeySchema)
	base.pkT = attrType(d.AttributeDefinitions, base.pk)
	if base.sk != "" {
		base.skT = attrType(d.AttributeDefinitions, base.sk)
	}
	out = append(out, base)
	for _, g := range d.GlobalSecondaryIndexes {
		c := indexChoice{name: strDeref(g.IndexName)}
		c.pk, c.sk = pkSk(g.KeySchema)
		c.pkT = attrType(d.AttributeDefinitions, c.pk)
		if c.sk != "" {
			c.skT = attrType(d.AttributeDefinitions, c.sk)
		}
		out = append(out, c)
	}
	for _, l := range d.LocalSecondaryIndexes {
		c := indexChoice{name: strDeref(l.IndexName)}
		c.pk, c.sk = pkSk(l.KeySchema)
		c.pkT = attrType(d.AttributeDefinitions, c.pk)
		if c.sk != "" {
			c.skT = attrType(d.AttributeDefinitions, c.sk)
		}
		out = append(out, c)
	}
	return out
}

// pkSk extracts (HASH, RANGE) attribute names from a KeySchema.
func pkSk(ks []ddbtypes.KeySchemaElement) (string, string) {
	var pk, sk string
	for _, k := range ks {
		switch k.KeyType {
		case ddbtypes.KeyTypeHash:
			pk = strDeref(k.AttributeName)
		case ddbtypes.KeyTypeRange:
			sk = strDeref(k.AttributeName)
		}
	}
	return pk, sk
}

// typedAV constructs an AttributeValue of the appropriate scalar type.
// B (binary) is treated as string for v1; users needing raw bytes use PartiQL.
func typedAV(t, v string) ddbtypes.AttributeValue {
	switch t {
	case "N":
		return &ddbtypes.AttributeValueMemberN{Value: v}
	case "B":
		return &ddbtypes.AttributeValueMemberB{Value: []byte(v)}
	default:
		return &ddbtypes.AttributeValueMemberS{Value: v}
	}
}

// queryDescribeCmd loads the table description.
func queryDescribeCmd(client *awsx.DynamoClient, table string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return queryDescribeMsg{table: table, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		d, err := client.DescribeTable(ctx, table)
		return queryDescribeMsg{table: table, desc: d, err: err}
	}
}

// queryPageCmdNamed runs a Query with ExpressionAttributeNames support.
// We cannot call awsx.DynamoClient.Query directly because the v1 wrapper
// doesn't expose names; use QueryRaw via a helper added to the awsx package.
func queryPageCmdNamed(client *awsx.DynamoClient, in awsx.QueryInput, names map[string]string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return queryPageMsg{table: in.Table, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res, err := client.QueryWithNames(ctx, in, names)
		return queryPageMsg{table: in.Table, res: res, err: err}
	}
}
