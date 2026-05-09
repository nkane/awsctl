package dynamo

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	awsx "github.com/nkane/awsctl/internal/aws"
)

// scanPageMsg carries one page of scan results.
type scanPageMsg struct {
	table string
	res   *awsx.ScanResult
	err   error
}

// scanPageCmd runs Scan with optional ExclusiveStartKey.
func scanPageCmd(client *awsx.DynamoClient, table string, lek map[string]ddbtypes.AttributeValue) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return scanPageMsg{table: table, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res, err := client.Scan(ctx, awsx.ScanInput{
			Table:             table,
			Limit:             50,
			ExclusiveStartKey: lek,
		})
		return scanPageMsg{table: table, res: res, err: err}
	}
}

// ScanModel is the paginated, row-selectable scan viewer for one table.
type ScanModel struct {
	client  *awsx.DynamoClient
	table   string
	keys    []string // primary key attribute names (pk[, sk])
	items   []map[string]ddbtypes.AttributeValue
	cursor  int
	vp      viewport.Model
	spinner spinner.Model
	loading bool
	err     string

	page     int
	totItems int
	lek      map[string]ddbtypes.AttributeValue

	width  int
	height int
}

// NewScan constructs the scan screen for a table. keys is the table's primary
// key attribute names (used to extract a key from a selected item for opening
// item view); pass nil if unknown — selection still works, item view loses
// refetch capability.
func NewScan(client *awsx.DynamoClient, table string, keys []string) ScanModel {
	vp := viewport.New(0, 0)
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return ScanModel{
		client:  client,
		table:   table,
		keys:    keys,
		vp:      vp,
		spinner: sp,
		loading: true,
	}
}

// Init kicks off the first scan page.
func (m ScanModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, scanPageCmd(m.client, m.table, nil))
}

// Name returns the table name (for status bar).
func (m ScanModel) Name() string { return m.table }

// Selected returns the currently highlighted item, or nil if none.
func (m ScanModel) Selected() map[string]ddbtypes.AttributeValue {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return nil
	}
	return m.items[m.cursor]
}

// SelectedKey extracts the primary key from the selected item, or nil.
func (m ScanModel) SelectedKey() map[string]ddbtypes.AttributeValue {
	it := m.Selected()
	if it == nil || len(m.keys) == 0 {
		return nil
	}
	out := map[string]ddbtypes.AttributeValue{}
	for _, k := range m.keys {
		if v, ok := it[k]; ok {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// SetSize sizes the viewport (1-line title + 1-line footer).
func (m *ScanModel) SetSize(w, h int) {
	m.width, m.height = w, h
	body := h - 2
	if body < 4 {
		body = 4
	}
	m.vp.Width = w
	m.vp.Height = body
	m.refresh()
}

// Update handles scan page results + key input.
func (m ScanModel) Update(msg tea.Msg) (ScanModel, tea.Cmd) {
	switch msg := msg.(type) {
	case scanPageMsg:
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
		m.items = append(m.items, msg.res.Items...)
		m.refresh()
		return m, nil

	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.items)-1 {
				m.cursor++
				m.refresh()
				m.ensureCursorVisible()
			}
			return m, nil
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.refresh()
				m.ensureCursorVisible()
			}
			return m, nil
		case "n":
			if m.loading || m.lek == nil {
				return m, nil
			}
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, scanPageCmd(m.client, m.table, m.lek))
		case "r":
			m.loading = true
			m.err = ""
			m.page = 0
			m.totItems = 0
			m.cursor = 0
			m.items = nil
			m.lek = nil
			m.vp.SetContent("")
			return m, tea.Batch(m.spinner.Tick, scanPageCmd(m.client, m.table, nil))
		case "g":
			m.cursor = 0
			m.refresh()
			m.vp.GotoTop()
			return m, nil
		case "G":
			if len(m.items) > 0 {
				m.cursor = len(m.items) - 1
			}
			m.refresh()
			m.vp.GotoBottom()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// ensureCursorVisible scrolls so the cursor row stays in view.
func (m *ScanModel) ensureCursorVisible() {
	row := m.cursor // one row per item
	top := m.vp.YOffset
	bot := top + m.vp.Height - 1
	if row < top {
		m.vp.SetYOffset(row)
	} else if row > bot {
		m.vp.SetYOffset(row - m.vp.Height + 1)
	}
}

// refresh re-renders all items into the viewport with the cursor highlighted.
func (m *ScanModel) refresh() {
	if len(m.items) == 0 {
		m.vp.SetContent("")
		return
	}
	var b strings.Builder
	cursorSty := lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	for i, it := range m.items {
		marker := "  "
		line := summarizeItem(it)
		if i == m.cursor {
			marker = cursorSty.Render("▸ ")
			line = cursorSty.Render(line)
		}
		fmt.Fprintf(&b, "%s%s\n", marker, line)
	}
	m.vp.SetContent(b.String())
}

// View renders the scan screen.
func (m ScanModel) View() string {
	titleSty := lipgloss.NewStyle().Bold(true)
	title := titleSty.Render("scan: " + m.table)
	body := m.vp.View()
	if m.err != "" {
		body = errStyle.Render("error: "+m.err) + "\n\n" + faint("press r to retry")
	} else if m.loading && m.totItems == 0 {
		body = fmt.Sprintf("%s scanning %s…", m.spinner.View(), m.table)
	}

	more := "no more pages"
	if m.lek != nil {
		more = "more available — press n"
	}
	loadHint := ""
	if m.loading {
		loadHint = " · " + m.spinner.View() + " loading"
	}
	footer := faint(fmt.Sprintf("page %d · items %d · cursor %d · %s%s    j/k cursor · enter open · n next · r reset · esc back",
		m.page, m.totItems, m.cursor+1, more, loadHint))
	return title + "\n" + body + "\n" + footer
}

// summarizeItem renders one item as a single-line preview: key=val pairs joined.
func summarizeItem(item map[string]ddbtypes.AttributeValue) string {
	keys := make([]string, 0, len(item))
	for k := range item {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, oneLineAV(item[k])))
	}
	s := strings.Join(parts, "  ")
	if len(s) > 240 {
		s = s[:237] + "…"
	}
	return s
}

// oneLineAV renders an AttributeValue in compact single-line form.
func oneLineAV(v ddbtypes.AttributeValue) string {
	switch x := v.(type) {
	case *ddbtypes.AttributeValueMemberS:
		s := x.Value
		if len(s) > 60 {
			s = s[:57] + "…"
		}
		return strconvQuote(s)
	case *ddbtypes.AttributeValueMemberN:
		return x.Value
	case *ddbtypes.AttributeValueMemberBOOL:
		return fmt.Sprintf("%t", x.Value)
	case *ddbtypes.AttributeValueMemberNULL:
		return "null"
	case *ddbtypes.AttributeValueMemberB:
		return fmt.Sprintf("<%dB>", len(x.Value))
	case *ddbtypes.AttributeValueMemberSS:
		return fmt.Sprintf("{S×%d}", len(x.Value))
	case *ddbtypes.AttributeValueMemberNS:
		return fmt.Sprintf("{N×%d}", len(x.Value))
	case *ddbtypes.AttributeValueMemberBS:
		return fmt.Sprintf("{B×%d}", len(x.Value))
	case *ddbtypes.AttributeValueMemberL:
		return fmt.Sprintf("[%d]", len(x.Value))
	case *ddbtypes.AttributeValueMemberM:
		return fmt.Sprintf("{%d}", len(x.Value))
	}
	return "?"
}

// formatItem renders one item as indented JSON-ish text. Kept for legacy uses.
func formatItem(item map[string]ddbtypes.AttributeValue) string {
	plain := make(map[string]interface{}, len(item))
	keys := make([]string, 0, len(item))
	for k := range item {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		plain[k] = simplify(item[k])
	}
	out, err := json.MarshalIndent(plain, "  ", "  ")
	if err != nil {
		return "  " + faint("(unrenderable item)")
	}
	return "  " + string(out)
}

// simplify reduces an AttributeValue to a Go value for JSON rendering.
func simplify(v ddbtypes.AttributeValue) interface{} {
	switch x := v.(type) {
	case *ddbtypes.AttributeValueMemberS:
		return x.Value
	case *ddbtypes.AttributeValueMemberN:
		return x.Value
	case *ddbtypes.AttributeValueMemberBOOL:
		return x.Value
	case *ddbtypes.AttributeValueMemberNULL:
		return nil
	case *ddbtypes.AttributeValueMemberSS:
		return x.Value
	case *ddbtypes.AttributeValueMemberNS:
		return x.Value
	case *ddbtypes.AttributeValueMemberBS:
		out := make([]string, len(x.Value))
		for i, b := range x.Value {
			out[i] = fmt.Sprintf("<%dB>", len(b))
		}
		return out
	case *ddbtypes.AttributeValueMemberB:
		return fmt.Sprintf("<%dB>", len(x.Value))
	case *ddbtypes.AttributeValueMemberL:
		out := make([]interface{}, len(x.Value))
		for i, e := range x.Value {
			out[i] = simplify(e)
		}
		return out
	case *ddbtypes.AttributeValueMemberM:
		out := make(map[string]interface{}, len(x.Value))
		for k, e := range x.Value {
			out[k] = simplify(e)
		}
		return out
	}
	return "?"
}
