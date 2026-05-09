package dynamo

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	awsx "github.com/nkane/awsctl/internal/aws"
)

// itemRefetchedMsg is returned after a re-fetch via GetItem.
type itemRefetchedMsg struct {
	item map[string]ddbtypes.AttributeValue
	err  error
}

// ItemModel renders one Dynamo item as a navigable tree with type badges.
type ItemModel struct {
	client *awsx.DynamoClient
	table  string
	item   map[string]ddbtypes.AttributeValue
	key    map[string]ddbtypes.AttributeValue // for r=refetch
	keySet bool

	collapsed map[string]bool // path -> collapsed
	vp        viewport.Model
	spinner   spinner.Model
	loading   bool
	flash     string // transient status (copy ok / refetched)
	err       string

	width, height int
}

// NewItem builds an item view for an already-loaded item. If key is non-nil,
// the model can re-fetch via 'r'.
func NewItem(client *awsx.DynamoClient, table string, item map[string]ddbtypes.AttributeValue, key map[string]ddbtypes.AttributeValue) ItemModel {
	vp := viewport.New(0, 0)
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	m := ItemModel{
		client:    client,
		table:     table,
		item:      item,
		key:       key,
		keySet:    key != nil,
		collapsed: map[string]bool{},
		vp:        vp,
		spinner:   sp,
	}
	return m
}

// Init returns nil — content set on first SetSize.
func (m ItemModel) Init() tea.Cmd { return nil }

// Name returns table name for status.
func (m ItemModel) Name() string { return m.table }

// SetSize sizes the viewport.
func (m *ItemModel) SetSize(w, h int) {
	m.width, m.height = w, h
	body := h - 2
	if body < 4 {
		body = 4
	}
	m.vp.Width = w
	m.vp.Height = body
	m.refresh()
}

// refresh re-renders the tree into the viewport.
func (m *ItemModel) refresh() {
	if m.item == nil {
		m.vp.SetContent(faint("(item not found)"))
		return
	}
	m.vp.SetContent(renderTree(m.item, m.collapsed))
}

// Update handles input + refetch results.
func (m ItemModel) Update(msg tea.Msg) (ItemModel, tea.Cmd) {
	switch msg := msg.(type) {
	case itemRefetchedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err = ""
		m.item = msg.item
		m.flash = "refetched"
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
		case "y":
			if m.item == nil {
				return m, nil
			}
			data, err := json.MarshalIndent(plainOf(m.item), "", "  ")
			if err != nil {
				m.err = "marshal: " + err.Error()
				return m, nil
			}
			if err := clipboard.WriteAll(string(data)); err != nil {
				m.err = "clipboard: " + err.Error()
				return m, nil
			}
			m.flash = "copied JSON to clipboard"
			m.err = ""
			return m, nil
		case "r":
			if !m.keySet || m.client == nil {
				m.err = "cannot refetch: no key in context"
				return m, nil
			}
			m.loading = true
			m.err = ""
			m.flash = ""
			return m, tea.Batch(m.spinner.Tick, getItemCmd(m.client, m.table, m.key))
		case " ":
			// space toggles collapse on the cursor line — viewport doesn't track
			// a cursor, so v1 toggles the top-level by typed key prefix is heavy.
			// For v1 simplicity: 'c' collapses all, 'e' expands all.
			return m, nil
		case "c":
			collapseAll(m.item, "", m.collapsed, true)
			m.refresh()
			return m, nil
		case "e":
			m.collapsed = map[string]bool{}
			m.refresh()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// View renders the item view.
func (m ItemModel) View() string {
	titleSty := lipgloss.NewStyle().Bold(true)
	title := titleSty.Render("item: " + m.table)
	body := m.vp.View()
	if m.loading {
		body = m.spinner.View() + " refetching…"
	}
	status := ""
	if m.err != "" {
		status = errStyle.Render("error: " + m.err)
	} else if m.flash != "" {
		status = faint(m.flash)
	}
	footer := faint("y copy JSON · r refetch · c collapse all · e expand all · g/G top/bottom · esc back")
	if status != "" {
		footer = status + "    " + footer
	}
	return title + "\n" + body + "\n" + footer
}

// getItemCmd dispatches a single GetItem call.
func getItemCmd(client *awsx.DynamoClient, table string, key map[string]ddbtypes.AttributeValue) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		it, err := client.GetItem(ctx, table, key)
		return itemRefetchedMsg{item: it, err: err}
	}
}

// renderTree produces a multi-line string with type badges + indentation.
// path identifies each node so collapse state survives re-renders.
func renderTree(item map[string]ddbtypes.AttributeValue, collapsed map[string]bool) string {
	var b strings.Builder
	keys := sortedKeys(item)
	for _, k := range keys {
		renderNode(&b, k, item[k], "", k, 0, collapsed)
	}
	return b.String()
}

// renderNode renders one attribute. label is the displayed key; path is the
// collapse-state identifier; depth controls indent.
func renderNode(b *strings.Builder, label string, av ddbtypes.AttributeValue, parentPath, path string, depth int, collapsed map[string]bool) {
	indent := strings.Repeat("  ", depth)
	badge := badgeOf(av)
	keySty := lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	bSty := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Faint(true)

	switch x := av.(type) {
	case *ddbtypes.AttributeValueMemberM:
		marker := "▾"
		if collapsed[path] {
			marker = "▸"
		}
		fmt.Fprintf(b, "%s%s %s %s  %s\n", indent, marker, keySty.Render(label), bSty.Render("[M]"), bSty.Render(fmt.Sprintf("%d keys", len(x.Value))))
		if !collapsed[path] {
			subKeys := make([]string, 0, len(x.Value))
			for k := range x.Value {
				subKeys = append(subKeys, k)
			}
			sort.Strings(subKeys)
			for _, k := range subKeys {
				renderNode(b, k, x.Value[k], path, path+"."+k, depth+1, collapsed)
			}
		}
	case *ddbtypes.AttributeValueMemberL:
		marker := "▾"
		if collapsed[path] {
			marker = "▸"
		}
		fmt.Fprintf(b, "%s%s %s %s  %s\n", indent, marker, keySty.Render(label), bSty.Render("[L]"), bSty.Render(fmt.Sprintf("%d items", len(x.Value))))
		if !collapsed[path] {
			for i, e := range x.Value {
				renderNode(b, fmt.Sprintf("[%d]", i), e, path, fmt.Sprintf("%s[%d]", path, i), depth+1, collapsed)
			}
		}
	case *ddbtypes.AttributeValueMemberSS:
		fmt.Fprintf(b, "%s  %s %s  %s\n", indent, keySty.Render(label), bSty.Render(badge), formatSet(x.Value))
	case *ddbtypes.AttributeValueMemberNS:
		fmt.Fprintf(b, "%s  %s %s  %s\n", indent, keySty.Render(label), bSty.Render(badge), formatSet(x.Value))
	case *ddbtypes.AttributeValueMemberBS:
		out := make([]string, len(x.Value))
		for i, bb := range x.Value {
			out[i] = fmt.Sprintf("<%dB>", len(bb))
		}
		fmt.Fprintf(b, "%s  %s %s  %s\n", indent, keySty.Render(label), bSty.Render(badge), formatSet(out))
	default:
		val := scalarString(av)
		fmt.Fprintf(b, "%s  %s %s  %s\n", indent, keySty.Render(label), bSty.Render(badge), val)
	}
}

// scalarString renders a non-collection AttributeValue as a string.
func scalarString(av ddbtypes.AttributeValue) string {
	switch x := av.(type) {
	case *ddbtypes.AttributeValueMemberS:
		return strconvQuote(x.Value)
	case *ddbtypes.AttributeValueMemberN:
		return x.Value
	case *ddbtypes.AttributeValueMemberBOOL:
		return fmt.Sprintf("%t", x.Value)
	case *ddbtypes.AttributeValueMemberNULL:
		return "null"
	case *ddbtypes.AttributeValueMemberB:
		return fmt.Sprintf("<%dB>", len(x.Value))
	}
	return "?"
}

// badgeOf returns the short type tag for an AttributeValue.
func badgeOf(av ddbtypes.AttributeValue) string {
	switch av.(type) {
	case *ddbtypes.AttributeValueMemberS:
		return "[S]"
	case *ddbtypes.AttributeValueMemberN:
		return "[N]"
	case *ddbtypes.AttributeValueMemberB:
		return "[B]"
	case *ddbtypes.AttributeValueMemberBOOL:
		return "[BOOL]"
	case *ddbtypes.AttributeValueMemberNULL:
		return "[NULL]"
	case *ddbtypes.AttributeValueMemberSS:
		return "[SS]"
	case *ddbtypes.AttributeValueMemberNS:
		return "[NS]"
	case *ddbtypes.AttributeValueMemberBS:
		return "[BS]"
	case *ddbtypes.AttributeValueMemberL:
		return "[L]"
	case *ddbtypes.AttributeValueMemberM:
		return "[M]"
	}
	return "[?]"
}

// formatSet renders set-member lists compactly with a head sample if very long.
func formatSet(vs []string) string {
	if len(vs) <= 8 {
		return "{" + strings.Join(vs, ", ") + "}"
	}
	head := strings.Join(vs[:8], ", ")
	return fmt.Sprintf("{%s, …+%d}", head, len(vs)-8)
}

// strconvQuote double-quotes a string for display, escaping minimally.
func strconvQuote(s string) string {
	// Use json.Marshal for correct escape handling without importing strconv.
	b, _ := json.Marshal(s)
	return string(b)
}

// sortedKeys returns the keys of an item sorted alphabetically.
func sortedKeys(item map[string]ddbtypes.AttributeValue) []string {
	out := make([]string, 0, len(item))
	for k := range item {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// collapseAll walks the tree and sets collapsed[path]=v for every M/L node.
func collapseAll(item map[string]ddbtypes.AttributeValue, parentPath string, collapsed map[string]bool, v bool) {
	for k, av := range item {
		path := k
		if parentPath != "" {
			path = parentPath + "." + k
		}
		walkCollapse(av, path, collapsed, v)
	}
}

func walkCollapse(av ddbtypes.AttributeValue, path string, collapsed map[string]bool, v bool) {
	switch x := av.(type) {
	case *ddbtypes.AttributeValueMemberM:
		collapsed[path] = v
		for k, e := range x.Value {
			walkCollapse(e, path+"."+k, collapsed, v)
		}
	case *ddbtypes.AttributeValueMemberL:
		collapsed[path] = v
		for i, e := range x.Value {
			walkCollapse(e, fmt.Sprintf("%s[%d]", path, i), collapsed, v)
		}
	}
}

// plainOf converts an AV map to JSON-friendly Go values (uses simplify from scan.go).
func plainOf(item map[string]ddbtypes.AttributeValue) map[string]interface{} {
	out := make(map[string]interface{}, len(item))
	for k, v := range item {
		out[k] = simplify(v)
	}
	return out
}
