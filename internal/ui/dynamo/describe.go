package dynamo

import (
	"context"
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

// describeLoadedMsg carries the result of DescribeTable.
type describeLoadedMsg struct {
	name string
	desc *ddbtypes.TableDescription
	err  error
}

// LoadDescribeCmd fetches table description in the background.
func LoadDescribeCmd(client *awsx.DynamoClient, name string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return describeLoadedMsg{name: name, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		d, err := client.DescribeTable(ctx, name)
		return describeLoadedMsg{name: name, desc: d, err: err}
	}
}

// DescribeModel renders one table's full description.
type DescribeModel struct {
	client  *awsx.DynamoClient
	name    string
	desc    *ddbtypes.TableDescription
	vp      viewport.Model
	spinner spinner.Model
	loading bool
	err     string
	width   int
	height  int
}

// NewDescribe constructs the describe screen and triggers initial load via Init.
func NewDescribe(client *awsx.DynamoClient, name string) DescribeModel {
	vp := viewport.New(0, 0)
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return DescribeModel{
		client:  client,
		name:    name,
		vp:      vp,
		spinner: sp,
		loading: true,
	}
}

// Init kicks off the first describe call.
func (m DescribeModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, LoadDescribeCmd(m.client, m.name))
}

// Name returns the table name (used by app for status bar).
func (m DescribeModel) Name() string { return m.name }

// Keys returns the primary key attribute names ([pk] or [pk, sk]) once the
// describe has loaded; nil otherwise.
func (m DescribeModel) Keys() []string {
	if m.desc == nil {
		return nil
	}
	out := make([]string, 0, len(m.desc.KeySchema))
	for _, k := range m.desc.KeySchema {
		out = append(out, strDeref(k.AttributeName))
	}
	return out
}

// SetSize sizes the viewport (1-line title + 1-line footer).
func (m *DescribeModel) SetSize(w, h int) {
	m.width, m.height = w, h
	body := h - 2
	if body < 4 {
		body = 4
	}
	m.vp.Width = w
	m.vp.Height = body
	if m.desc != nil {
		m.vp.SetContent(renderDescription(m.desc))
	}
}

// Update handles describe results + key input.
func (m DescribeModel) Update(msg tea.Msg) (DescribeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case describeLoadedMsg:
		if msg.name != m.name {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err = ""
		m.desc = msg.desc
		m.vp.SetContent(renderDescription(msg.desc))
		m.vp.GotoTop()
		return m, nil

	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		if msg.String() == "r" {
			m.loading = true
			m.err = ""
			return m, tea.Batch(m.spinner.Tick, LoadDescribeCmd(m.client, m.name))
		}
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// View renders the describe screen.
func (m DescribeModel) View() string {
	titleSty := lipgloss.NewStyle().Bold(true)
	title := titleSty.Render("table: " + m.name)
	body := m.vp.View()
	if m.loading {
		body = fmt.Sprintf("%s describing %s…", m.spinner.View(), m.name)
	} else if m.err != "" {
		body = errStyle.Render("error: "+m.err) + "\n\n" + faint("press r to retry")
	}
	footer := faint("r refresh · esc back")
	return title + "\n" + body + "\n" + footer
}

// renderDescription formats a TableDescription as a multi-section string for
// the viewport. Sections gracefully omit empty content.
func renderDescription(d *ddbtypes.TableDescription) string {
	if d == nil {
		return faint("(no description)")
	}
	var b strings.Builder
	hSty := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	kSty := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	row := func(k, v string) {
		if v == "" {
			v = "—"
		}
		b.WriteString("  ")
		b.WriteString(kSty.Render(k))
		b.WriteString("  ")
		b.WriteString(v)
		b.WriteString("\n")
	}
	hdr := func(s string) {
		b.WriteString("\n")
		b.WriteString(hSty.Render(s))
		b.WriteString("\n")
	}

	// Overview
	hdr("Overview")
	row("status", string(d.TableStatus))
	if d.CreationDateTime != nil {
		row("created", d.CreationDateTime.Format(time.RFC3339))
	}
	if d.ItemCount != nil {
		row("items", fmt.Sprintf("%d", *d.ItemCount))
	}
	if d.TableSizeBytes != nil {
		row("size", humanBytes(*d.TableSizeBytes))
	}
	if d.TableArn != nil {
		row("arn", *d.TableArn)
	}
	if d.TableId != nil {
		row("id", *d.TableId)
	}
	if d.DeletionProtectionEnabled != nil {
		row("deletion-protection", fmt.Sprintf("%t", *d.DeletionProtectionEnabled))
	}

	// Billing
	hdr("Billing")
	if d.BillingModeSummary != nil {
		row("mode", string(d.BillingModeSummary.BillingMode))
		if d.BillingModeSummary.LastUpdateToPayPerRequestDateTime != nil {
			row("last-pay-per-req", d.BillingModeSummary.LastUpdateToPayPerRequestDateTime.Format(time.RFC3339))
		}
	} else {
		row("mode", "PROVISIONED (default)")
	}
	if d.ProvisionedThroughput != nil {
		pt := d.ProvisionedThroughput
		if pt.ReadCapacityUnits != nil {
			row("read-units", fmt.Sprintf("%d", *pt.ReadCapacityUnits))
		}
		if pt.WriteCapacityUnits != nil {
			row("write-units", fmt.Sprintf("%d", *pt.WriteCapacityUnits))
		}
	}

	// Keys
	hdr("Primary Key")
	for _, k := range d.KeySchema {
		row(string(k.KeyType), strDeref(k.AttributeName)+"  ("+attrType(d.AttributeDefinitions, strDeref(k.AttributeName))+")")
	}

	// GSIs
	if len(d.GlobalSecondaryIndexes) > 0 {
		hdr(fmt.Sprintf("Global Secondary Indexes (%d)", len(d.GlobalSecondaryIndexes)))
		for _, gsi := range d.GlobalSecondaryIndexes {
			b.WriteString("  • " + strDeref(gsi.IndexName))
			if gsi.IndexStatus != "" {
				b.WriteString(" [" + string(gsi.IndexStatus) + "]")
			}
			b.WriteString("\n")
			for _, k := range gsi.KeySchema {
				row("  "+string(k.KeyType), strDeref(k.AttributeName))
			}
			if gsi.Projection != nil {
				row("  projection", string(gsi.Projection.ProjectionType))
				if len(gsi.Projection.NonKeyAttributes) > 0 {
					row("  proj-attrs", strings.Join(gsi.Projection.NonKeyAttributes, ", "))
				}
			}
			if gsi.ItemCount != nil {
				row("  items", fmt.Sprintf("%d", *gsi.ItemCount))
			}
			if gsi.IndexSizeBytes != nil {
				row("  size", humanBytes(*gsi.IndexSizeBytes))
			}
		}
	}

	// LSIs
	if len(d.LocalSecondaryIndexes) > 0 {
		hdr(fmt.Sprintf("Local Secondary Indexes (%d)", len(d.LocalSecondaryIndexes)))
		for _, lsi := range d.LocalSecondaryIndexes {
			b.WriteString("  • " + strDeref(lsi.IndexName) + "\n")
			for _, k := range lsi.KeySchema {
				row("  "+string(k.KeyType), strDeref(k.AttributeName))
			}
			if lsi.Projection != nil {
				row("  projection", string(lsi.Projection.ProjectionType))
			}
		}
	}

	// Streams
	hdr("Streams")
	if d.StreamSpecification != nil && d.StreamSpecification.StreamEnabled != nil && *d.StreamSpecification.StreamEnabled {
		row("enabled", "true")
		row("view-type", string(d.StreamSpecification.StreamViewType))
		if d.LatestStreamArn != nil {
			row("stream-arn", *d.LatestStreamArn)
		}
	} else {
		row("enabled", "false")
	}

	// SSE
	hdr("Encryption")
	if d.SSEDescription != nil {
		row("status", string(d.SSEDescription.Status))
		row("type", string(d.SSEDescription.SSEType))
		if d.SSEDescription.KMSMasterKeyArn != nil {
			row("kms-key", *d.SSEDescription.KMSMasterKeyArn)
		}
	} else {
		row("status", "DEFAULT (AWS-owned key)")
	}

	// All attribute definitions (sorted for stable rendering)
	if len(d.AttributeDefinitions) > 0 {
		hdr(fmt.Sprintf("Attribute Definitions (%d)", len(d.AttributeDefinitions)))
		defs := append([]ddbtypes.AttributeDefinition(nil), d.AttributeDefinitions...)
		sort.Slice(defs, func(i, j int) bool { return strDeref(defs[i].AttributeName) < strDeref(defs[j].AttributeName) })
		for _, a := range defs {
			row(strDeref(a.AttributeName), string(a.AttributeType))
		}
	}

	return b.String()
}

func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// attrType returns the AttributeType (S/N/B) for a named attribute, or "?".
func attrType(defs []ddbtypes.AttributeDefinition, name string) string {
	for _, a := range defs {
		if strDeref(a.AttributeName) == name {
			return string(a.AttributeType)
		}
	}
	return "?"
}

// humanBytes formats bytes as KB / MB / GB.
func humanBytes(n int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case n >= GB:
		return fmt.Sprintf("%.2f GB", float64(n)/float64(GB))
	case n >= MB:
		return fmt.Sprintf("%.2f MB", float64(n)/float64(MB))
	case n >= KB:
		return fmt.Sprintf("%.2f KB", float64(n)/float64(KB))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
