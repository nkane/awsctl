package lambda

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

	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	awsx "github.com/nkane/awsctl/internal/aws"
)

// detailLoadedMsg carries the result of GetFunctionDetail.
type detailLoadedMsg struct {
	name string
	d    *awsx.FunctionDetail
	err  error
}

// LoadDetailCmd fetches everything for one function in the background.
func LoadDetailCmd(client *awsx.LambdaClient, name string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return detailLoadedMsg{name: name, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		d, err := client.GetFunctionDetail(ctx, name)
		return detailLoadedMsg{name: name, d: d, err: err}
	}
}

// DetailTab identifies the active sub-tab in the detail view.
type DetailTab int

const (
	TabConfig DetailTab = iota
	TabCode
	TabEnv
	TabTriggers
	TabPermissions
	TabConcurrency
	TabAliases
	TabVersions
	TabLayers
	TabVPC
	TabURLs
	TabTags
)

var detailTabNames = []string{
	"Configuration", "Code", "Environment", "Triggers",
	"Permissions", "Concurrency", "Aliases", "Versions",
	"Layers", "VPC", "URL", "Tags",
}

// DetailModel is the per-function detail screen with sub-tabs.
type DetailModel struct {
	client    *awsx.LambdaClient
	name      string
	tab       DetailTab
	detail    *awsx.FunctionDetail
	loading   bool
	err       string
	width     int
	height    int
	spinner   spinner.Model
	vp        viewport.Model
	maskEnv   bool // start masked; toggle with 'e'
}

// NewDetail constructs a detail screen for the given function name.
func NewDetail(client *awsx.LambdaClient, name string) DetailModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	vp := viewport.New(0, 0)
	return DetailModel{
		client:  client,
		name:    name,
		spinner: sp,
		vp:      vp,
		loading: true,
		maskEnv: true,
	}
}

// Init starts the background fetch + spinner ticker.
func (m DetailModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, LoadDetailCmd(m.client, m.name))
}

// SetSize lays out the inner viewport. Reserves 2 lines for header + tab strip.
func (m *DetailModel) SetSize(w, h int) {
	m.width, m.height = w, h
	innerH := h - 4
	if innerH < 3 {
		innerH = 3
	}
	m.vp.Width = w
	m.vp.Height = innerH
	if m.detail != nil {
		m.vp.SetContent(m.renderTabBody())
	}
}

// Name returns the function name being shown.
func (m DetailModel) Name() string { return m.name }

// Update handles messages routed to the detail screen.
func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case detailLoadedMsg:
		if msg.name != m.name {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.detail = msg.d
		m.vp.SetContent(m.renderTabBody())
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
		switch msg.String() {
		case "tab", "right", "l":
			m.tab = DetailTab((int(m.tab) + 1) % len(detailTabNames))
			m.vp.SetContent(m.renderTabBody())
			m.vp.GotoTop()
			return m, nil
		case "shift+tab", "left", "h":
			m.tab = DetailTab((int(m.tab) - 1 + len(detailTabNames)) % len(detailTabNames))
			m.vp.SetContent(m.renderTabBody())
			m.vp.GotoTop()
			return m, nil
		case "e":
			// Toggle env-var masking.
			m.maskEnv = !m.maskEnv
			if m.tab == TabEnv {
				m.vp.SetContent(m.renderTabBody())
			}
			return m, nil
		case "r":
			m.loading = true
			m.err = ""
			return m, tea.Batch(m.spinner.Tick, LoadDetailCmd(m.client, m.name))
		}
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// View renders the entire detail screen.
func (m DetailModel) View() string {
	header := titleStyle.Render("Lambda · "+m.name) +
		"  " + faintSty.Render("(esc back · tab/h/l next pane · e mask · r refresh · j/k scroll)")

	if m.loading {
		return header + "\n\n  " + m.spinner.View() + " loading details…"
	}
	if m.err != "" {
		return header + "\n\n  " + errStyle.Render("error: "+m.err)
	}
	if m.detail == nil {
		return header + "\n\n  " + faint("no data")
	}
	return header + "\n" + m.tabsStrip() + "\n" + m.vp.View()
}

func (m DetailModel) tabsStrip() string {
	parts := make([]string, len(detailTabNames))
	for i, n := range detailTabNames {
		if DetailTab(i) == m.tab {
			parts[i] = activeSubTab.Render(n)
		} else {
			parts[i] = inactiveSubTab.Render(n)
		}
	}
	return strings.Join(parts, " ")
}

// renderTabBody dispatches to the per-tab renderer.
func (m DetailModel) renderTabBody() string {
	d := m.detail
	switch m.tab {
	case TabConfig:
		return m.renderConfig(d)
	case TabCode:
		return m.renderCode(d)
	case TabEnv:
		return m.renderEnv(d)
	case TabTriggers:
		return m.renderTriggers(d)
	case TabPermissions:
		return m.renderPermissions(d)
	case TabConcurrency:
		return m.renderConcurrency(d)
	case TabAliases:
		return m.renderAliases(d)
	case TabVersions:
		return m.renderVersions(d)
	case TabLayers:
		return m.renderLayers(d)
	case TabVPC:
		return m.renderVPC(d)
	case TabURLs:
		return m.renderURLs(d)
	case TabTags:
		return m.renderTags(d)
	}
	return ""
}

// ---- per-tab renderers ------------------------------------------------------

func (m DetailModel) renderConfig(d *awsx.FunctionDetail) string {
	if d.Function == nil || d.Function.Configuration == nil {
		return faint("(no configuration)")
	}
	c := d.Function.Configuration
	rows := [][2]string{
		{"Name", deref(c.FunctionName)},
		{"ARN", deref(c.FunctionArn)},
		{"Description", deref(c.Description)},
		{"Runtime", string(c.Runtime)},
		{"Handler", deref(c.Handler)},
		{"Architectures", joinArchs(c.Architectures)},
		{"Memory (MB)", intStr(c.MemorySize)},
		{"Timeout (s)", intStr(c.Timeout)},
		{"Ephemeral storage (MB)", ephemeralStr(c)},		{"Package type", string(c.PackageType)},
		{"Code size", bytesStr(c.CodeSize)},
		{"Code SHA256", deref(c.CodeSha256)},
		{"Version", deref(c.Version)},
		{"Revision ID", deref(c.RevisionId)},
		{"Last modified", deref(c.LastModified)},
		{"Last update status", string(c.LastUpdateStatus)},
		{"Last update reason", deref(c.LastUpdateStatusReason)},
		{"State", string(c.State)},
		{"State reason", deref(c.StateReason)},
		{"Role", deref(c.Role)},
		{"KMS Key ARN", deref(c.KMSKeyArn)},
		{"Logging format", loggingFormat(c)},
	}
	return kvTable(rows)
}

func (m DetailModel) renderCode(d *awsx.FunctionDetail) string {
	if d.Function == nil {
		return faint("(no code info)")
	}
	rows := [][2]string{}
	if loc := d.Function.Code; loc != nil {
		rows = append(rows,
			[2]string{"Repository type", deref(loc.RepositoryType)},
			[2]string{"Image URI", deref(loc.ImageUri)},
			[2]string{"Resolved image URI", deref(loc.ResolvedImageUri)},
			[2]string{"Location", truncURL(deref(loc.Location), 100)},
		)
	}
	if c := d.Function.Configuration; c != nil {
		if c.ImageConfigResponse != nil && c.ImageConfigResponse.ImageConfig != nil {
			ic := c.ImageConfigResponse.ImageConfig
			rows = append(rows,
				[2]string{"Image entry point", strings.Join(ic.EntryPoint, " ")},
				[2]string{"Image command", strings.Join(ic.Command, " ")},
				[2]string{"Image working dir", deref(ic.WorkingDirectory)},
			)
		}
		if c.SnapStart != nil {
			rows = append(rows, [2]string{"SnapStart apply on", string(c.SnapStart.ApplyOn)})
		}
	}
	if len(rows) == 0 {
		return faint("(no code info)")
	}
	return kvTable(rows)
}

func (m DetailModel) renderEnv(d *awsx.FunctionDetail) string {
	if d.Function == nil || d.Function.Configuration == nil || d.Function.Configuration.Environment == nil {
		return faint("(no environment variables)")
	}
	env := d.Function.Configuration.Environment
	if env.Error != nil && env.Error.Message != nil {
		return errStyle.Render("env error: " + *env.Error.Message)
	}
	if len(env.Variables) == 0 {
		return faint("(no environment variables)")
	}
	keys := make([]string, 0, len(env.Variables))
	for k := range env.Variables {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rows := make([][2]string, 0, len(keys))
	for _, k := range keys {
		v := env.Variables[k]
		if m.maskEnv {
			v = mask(v)
		}
		rows = append(rows, [2]string{k, v})
	}
	hint := faintSty.Render(fmt.Sprintf("(values %s — press 'e' to toggle)", maskState(m.maskEnv)))
	return hint + "\n\n" + kvTable(rows)
}

func (m DetailModel) renderTriggers(d *awsx.FunctionDetail) string {
	if len(d.EventSources) == 0 {
		if e, ok := d.Errors["event_sources"]; ok {
			return errStyle.Render("event sources error: " + e)
		}
		return faint("(no event source mappings)")
	}
	out := []string{}
	for _, es := range d.EventSources {
		rows := [][2]string{
			{"UUID", deref(es.UUID)},
			{"Source ARN", deref(es.EventSourceArn)},
			{"State", deref(es.State)},
			{"Last processing result", deref(es.LastProcessingResult)},
			{"Batch size", intStr(es.BatchSize)},
			{"Maximum batching window (s)", intStr(es.MaximumBatchingWindowInSeconds)},
			{"Starting position", string(es.StartingPosition)},
			{"Enabled", boolStr(es.State)},
		}
		out = append(out, kvTable(rows))
	}
	return strings.Join(out, "\n\n"+strings.Repeat("─", 40)+"\n\n")
}

func (m DetailModel) renderPermissions(d *awsx.FunctionDetail) string {
	roleARN := ""
	if d.Function != nil && d.Function.Configuration != nil {
		roleARN = deref(d.Function.Configuration.Role)
	}
	out := kvTable([][2]string{{"Execution role", roleARN}})
	if d.Policy == "" {
		out += "\n\n" + faint("(no resource-based policy attached)")
		return out
	}
	// Pretty-print the JSON policy.
	var pretty bytes
	if b, err := jsonPretty([]byte(d.Policy)); err == nil {
		pretty = b
	} else {
		pretty = []byte(d.Policy)
	}
	out += "\n\n" + sectionTitle.Render("Resource-based policy") + "\n" + string(pretty)
	return out
}

func (m DetailModel) renderConcurrency(d *awsx.FunctionDetail) string {
	rows := [][2]string{}
	if d.Concurrency != nil && d.Concurrency.ReservedConcurrentExecutions != nil {
		rows = append(rows, [2]string{"Reserved concurrent executions", intStr(d.Concurrency.ReservedConcurrentExecutions)})
	} else {
		rows = append(rows, [2]string{"Reserved concurrent executions", "(unreserved — uses account pool)"})
	}
	if e, ok := d.Errors["concurrency"]; ok {
		rows = append(rows, [2]string{"error", e})
	}
	return kvTable(rows)
}

func (m DetailModel) renderAliases(d *awsx.FunctionDetail) string {
	if len(d.Aliases) == 0 {
		return faint("(no aliases)")
	}
	out := []string{}
	for _, a := range d.Aliases {
		rows := [][2]string{
			{"Name", deref(a.Name)},
			{"Function version", deref(a.FunctionVersion)},
			{"ARN", deref(a.AliasArn)},
			{"Description", deref(a.Description)},
			{"Revision ID", deref(a.RevisionId)},
		}
		if a.RoutingConfig != nil && len(a.RoutingConfig.AdditionalVersionWeights) > 0 {
			parts := []string{}
			for v, w := range a.RoutingConfig.AdditionalVersionWeights {
				parts = append(parts, fmt.Sprintf("%s=%.2f", v, w))
			}
			rows = append(rows, [2]string{"Additional weights", strings.Join(parts, ", ")})
		}
		out = append(out, kvTable(rows))
	}
	return strings.Join(out, "\n\n"+strings.Repeat("─", 40)+"\n\n")
}

func (m DetailModel) renderVersions(d *awsx.FunctionDetail) string {
	if len(d.Versions) == 0 {
		return faint("(no versions)")
	}
	rows := [][2]string{}
	for _, v := range d.Versions {
		label := deref(v.Version)
		val := fmt.Sprintf("%s · %s · %dMB · %ds",
			deref(v.LastModified), string(v.Runtime), derefInt(v.MemorySize), derefInt(v.Timeout))
		rows = append(rows, [2]string{label, val})
	}
	return kvTable(rows)
}

func (m DetailModel) renderLayers(d *awsx.FunctionDetail) string {
	if d.Function == nil || d.Function.Configuration == nil || len(d.Function.Configuration.Layers) == 0 {
		return faint("(no layers attached)")
	}
	rows := [][2]string{}
	for _, l := range d.Function.Configuration.Layers {
		rows = append(rows, [2]string{deref(l.Arn), fmt.Sprintf("size=%s signing=%s",
			bytesStr(l.CodeSize), deref(l.SigningJobArn))})
	}
	return kvTable(rows)
}

func (m DetailModel) renderVPC(d *awsx.FunctionDetail) string {
	if d.Function == nil || d.Function.Configuration == nil || d.Function.Configuration.VpcConfig == nil {
		return faint("(no VPC configuration)")
	}
	vc := d.Function.Configuration.VpcConfig
	if len(vc.SubnetIds) == 0 && len(vc.SecurityGroupIds) == 0 {
		return faint("(no VPC configuration)")
	}
	return kvTable([][2]string{
		{"VPC ID", deref(vc.VpcId)},
		{"Subnets", strings.Join(vc.SubnetIds, ", ")},
		{"Security groups", strings.Join(vc.SecurityGroupIds, ", ")},
		{"IPv6 allowed for dual stack", boolPtrStr(vc.Ipv6AllowedForDualStack)},
	})
}

func (m DetailModel) renderURLs(d *awsx.FunctionDetail) string {
	if len(d.URLConfigs) == 0 {
		return faint("(no function URLs configured)")
	}
	out := []string{}
	for _, u := range d.URLConfigs {
		rows := [][2]string{
			{"URL", deref(u.FunctionUrl)},
			{"Auth type", string(u.AuthType)},
			{"Invoke mode", string(u.InvokeMode)},
			{"Created", deref(u.CreationTime)},
			{"Last modified", deref(u.LastModifiedTime)},
		}
		if u.Cors != nil {
			rows = append(rows,
				[2]string{"CORS allow origins", strings.Join(u.Cors.AllowOrigins, ", ")},
				[2]string{"CORS allow methods", strings.Join(u.Cors.AllowMethods, ", ")},
				[2]string{"CORS allow headers", strings.Join(u.Cors.AllowHeaders, ", ")},
			)
		}
		out = append(out, kvTable(rows))
	}
	return strings.Join(out, "\n\n"+strings.Repeat("─", 40)+"\n\n")
}

func (m DetailModel) renderTags(d *awsx.FunctionDetail) string {
	if len(d.Tags) == 0 {
		return faint("(no tags)")
	}
	keys := make([]string, 0, len(d.Tags))
	for k := range d.Tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rows := make([][2]string, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, [2]string{k, d.Tags[k]})
	}
	return kvTable(rows)
}

// ---- helpers ----------------------------------------------------------------

type bytes = []byte

func jsonPretty(in []byte) ([]byte, error) {
	var v any
	if err := json.Unmarshal(in, &v); err != nil {
		return nil, err
	}
	return json.MarshalIndent(v, "", "  ")
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefInt(p *int32) int32 {
	if p == nil {
		return 0
	}
	return *p
}

func intStr(p *int32) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *p)
}

func boolStr(state *string) string {
	if state == nil {
		return "-"
	}
	switch strings.ToLower(*state) {
	case "enabled":
		return "true"
	case "disabled":
		return "false"
	}
	return *state
}

func boolPtrStr(p *bool) string {
	if p == nil {
		return "-"
	}
	if *p {
		return "true"
	}
	return "false"
}

func bytesStr(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.2f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.2f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func mask(v string) string {
	if v == "" {
		return ""
	}
	// Show first/last 2 chars when long enough so the user can still
	// distinguish values during a screencast.
	if len(v) <= 6 {
		return strings.Repeat("•", len(v))
	}
	return v[:2] + strings.Repeat("•", len(v)-4) + v[len(v)-2:]
}

func maskState(masked bool) string {
	if masked {
		return "masked"
	}
	return "visible"
}

func truncURL(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// kvTable renders a 2-column key/value table aligned on the longest key.
func kvTable(rows [][2]string) string {
	maxK := 0
	for _, r := range rows {
		if len(r[0]) > maxK {
			maxK = len(r[0])
		}
	}
	var b strings.Builder
	for _, r := range rows {
		k := r[0]
		v := r[1]
		if v == "" {
			v = "-"
		}
		fmt.Fprintf(&b, "  %s%s  %s\n",
			keyStyle.Render(k),
			strings.Repeat(" ", maxK-len(k)),
			valStyle.Render(v),
		)
	}
	return b.String()
}

// Helpers that pull from optional nested structs in the SDK.
func ephemeralStr(c *lambdatypes.FunctionConfiguration) string {
	if c == nil || c.EphemeralStorage == nil || c.EphemeralStorage.Size == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *c.EphemeralStorage.Size)
}

func joinArchs(a []lambdatypes.Architecture) string {
	out := make([]string, 0, len(a))
	for _, x := range a {
		out = append(out, string(x))
	}
	if len(out) == 0 {
		return "-"
	}
	return strings.Join(out, ", ")
}

// styles (kept local; could move to theme later)
var (
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	keyStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	valStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	sectionTitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("219"))
	activeSubTab   = lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Background(lipgloss.Color("63")).Padding(0, 1)
	inactiveSubTab = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 1)
)

// loggingFormat extracts the optional LoggingConfig.LogFormat string.
func loggingFormat(c *lambdatypes.FunctionConfiguration) string {
	if c == nil || c.LoggingConfig == nil {
		return "-"
	}
	return string(c.LoggingConfig.LogFormat)
}
