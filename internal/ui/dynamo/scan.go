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

// ScanModel is the paginated scan viewer for one table.
type ScanModel struct {
	client  *awsx.DynamoClient
	table   string
	vp      viewport.Model
	spinner spinner.Model
	loading bool
	err     string

	page     int
	totItems int
	lek      map[string]ddbtypes.AttributeValue
	buf      strings.Builder

	width  int
	height int
}

// NewScan constructs the scan screen for a table. Init triggers the first page.
func NewScan(client *awsx.DynamoClient, table string) ScanModel {
	vp := viewport.New(0, 0)
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return ScanModel{
		client:  client,
		table:   table,
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

// SetSize sizes the viewport (1-line title + 1-line footer).
func (m *ScanModel) SetSize(w, h int) {
	m.width, m.height = w, h
	body := h - 2
	if body < 4 {
		body = 4
	}
	m.vp.Width = w
	m.vp.Height = body
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
		m.buf.WriteString(renderPage(m.page, msg.res.Items))
		m.vp.SetContent(m.buf.String())
		m.vp.GotoBottom()
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
			m.lek = nil
			m.buf.Reset()
			m.vp.SetContent("")
			return m, tea.Batch(m.spinner.Tick, scanPageCmd(m.client, m.table, nil))
		case "g":
			m.vp.GotoTop()
			return m, nil
		case "G":
			m.vp.GotoBottom()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
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
	footer := faint(fmt.Sprintf("page %d · items %d · %s%s    n next · r reset · g/G top/bottom · esc back",
		m.page, m.totItems, more, loadHint))
	return title + "\n" + body + "\n" + footer
}

// renderPage formats one page of items as numbered JSON blocks.
func renderPage(page int, items []map[string]ddbtypes.AttributeValue) string {
	if len(items) == 0 {
		return faint(fmt.Sprintf("\n— page %d: 0 items —\n", page))
	}
	var b strings.Builder
	hdrSty := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	b.WriteString("\n")
	b.WriteString(hdrSty.Render(fmt.Sprintf("— page %d: %d items —", page, len(items))))
	b.WriteString("\n\n")
	for i, it := range items {
		b.WriteString(faint(fmt.Sprintf("[%d]\n", i+1)))
		b.WriteString(formatItem(it))
		b.WriteString("\n\n")
	}
	return b.String()
}

// formatItem renders one item as indented JSON-ish text. Attribute values are
// reduced to their primary scalar payload (S/N/BOOL/NULL/L/M/SS/NS/BS/B).
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
