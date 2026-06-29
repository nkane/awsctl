package lambda

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsx "github.com/nkane/awsctl/internal/aws"
	"github.com/nkane/awsctl/internal/ui/components"
)

// metricSpec is the static (namespace/metric/dims/stat) part of a query. The
// panel attaches the time window + period from its TimeRange at fetch time.
type metricSpec struct {
	Title      string
	Namespace  string
	MetricName string
	Dimensions map[string]string
	Stat       string
}

// metricsLoadedMsg carries the result of a GetMetrics fetch. gen guards against
// stale responses when the user changes the range while a fetch is in flight.
type metricsLoadedMsg struct {
	gen    int
	series []awsx.MetricSeries
	err    error
}

// metricsPanel is the reusable metrics sub-view: a TimeRange selector on top and
// one Chart per metric. It owns loading/error state and chart layout.
type metricsPanel struct {
	mc      *awsx.MetricClient
	specs   []metricSpec
	tr      components.TimeRange
	charts  []components.Chart
	spinner spinner.Model
	loaded  bool // first fetch has been triggered
	loading bool
	err     string
	gen     int
	width   int
	height  int
}

// newMetricsPanel builds a panel for the given metric specs. cfg may be nil
// (config not loaded yet); the panel then renders an error line instead of
// charts.
func newMetricsPanel(cfg *awsx.Config, specs []metricSpec) metricsPanel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	charts := make([]components.Chart, len(specs))
	for i, s := range specs {
		charts[i] = components.NewChart(s.Title)
	}
	var mc *awsx.MetricClient
	if cfg != nil {
		mc = awsx.NewMetricClient(cfg)
	}
	return metricsPanel{
		mc:      mc,
		specs:   specs,
		tr:      components.NewTimeRange(),
		charts:  charts,
		spinner: sp,
	}
}

// SetSize records the available area and re-lays the charts.
func (p *metricsPanel) SetSize(w, h int) {
	p.width, p.height = w, h
	p.layout()
}

// metricsCols returns the number of chart columns for the current width.
func (p *metricsPanel) metricsCols() int {
	if p.width >= 80 {
		return 2
	}
	return 1
}

// layout sizes each chart to fit a simple grid in the available area.
func (p *metricsPanel) layout() {
	n := len(p.charts)
	if n == 0 {
		return
	}
	cols := p.metricsCols()
	rows := (n + cols - 1) / cols
	if rows < 1 {
		rows = 1
	}
	const gap = 1
	chartW := (p.width - (cols-1)*gap) / cols
	if chartW < 12 {
		chartW = 12
	}
	// Reserve one line per row for inter-row spacing.
	chartH := (p.height - (rows - 1)) / rows
	if chartH < 4 {
		chartH = 4
	}
	for i := range p.charts {
		p.charts[i].SetSize(chartW, chartH)
	}
}

// startIfNeeded kicks off the first fetch the first time the tab is opened.
func (p *metricsPanel) startIfNeeded() tea.Cmd {
	if p.loaded {
		return nil
	}
	return p.start()
}

// start triggers a fetch for the current range and starts the spinner.
func (p *metricsPanel) start() tea.Cmd {
	p.loaded = true
	if p.mc == nil {
		p.err = "aws config not loaded"
		p.loading = false
		return nil
	}
	p.loading = true
	p.err = ""
	p.gen++
	gen := p.gen
	opt := p.tr.Selected()
	specs := p.specs
	mc := p.mc
	fetch := func() tea.Msg {
		end := time.Now()
		start := end.Add(-opt.Duration)
		qs := make([]awsx.MetricQuery, len(specs))
		for i, s := range specs {
			qs[i] = awsx.MetricQuery{
				Namespace:  s.Namespace,
				MetricName: s.MetricName,
				Dimensions: s.Dimensions,
				Stat:       s.Stat,
				Period:     opt.Period,
				Start:      start,
				End:        end,
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		series, err := mc.GetMetrics(ctx, qs)
		return metricsLoadedMsg{gen: gen, series: series, err: err}
	}
	return tea.Batch(p.spinner.Tick, fetch)
}

// Update routes panel-relevant messages: fetch results, spinner ticks, and the
// range keys (h/l/left/right) that change the selected window. Range changes and
// stale-fetch guarding are handled here; the returned cmd drives any refetch.
func (p *metricsPanel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case metricsLoadedMsg:
		if msg.gen != p.gen {
			return nil // stale; a newer fetch superseded it
		}
		p.loading = false
		if msg.err != nil {
			p.err = msg.err.Error() // keep prior charts visible
			return nil
		}
		p.err = ""
		for i := range p.charts {
			if i < len(msg.series) {
				p.charts[i] = p.charts[i].WithSeries(msg.series[i])
			}
		}
		return nil

	case spinner.TickMsg:
		if !p.loading {
			return nil
		}
		var cmd tea.Cmd
		p.spinner, cmd = p.spinner.Update(msg)
		return cmd

	case tea.KeyMsg:
		tr, changed := p.tr.Update(msg)
		p.tr = tr
		if changed {
			return p.start()
		}
		return nil
	}
	return nil
}

// View renders the selector, an optional status line, and the chart grid.
func (p metricsPanel) View() string {
	parts := []string{p.tr.View()}

	switch {
	case p.loading:
		parts = append(parts, p.spinner.View()+" loading metrics…")
	case p.err != "":
		parts = append(parts, errStyle.Render("metrics error: "+p.err))
	}

	cols := p.metricsCols()
	var rows []string
	var row []string
	for i := range p.charts {
		row = append(row, p.charts[i].View())
		if len(row) == cols {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, withGaps(row)...))
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, withGaps(row)...))
	}
	parts = append(parts, lipgloss.JoinVertical(lipgloss.Left, rows...))
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// withGaps interleaves a single-space column between charts in a row.
func withGaps(cells []string) []string {
	if len(cells) <= 1 {
		return cells
	}
	out := make([]string, 0, len(cells)*2-1)
	for i, c := range cells {
		if i > 0 {
			out = append(out, " ")
		}
		out = append(out, c)
	}
	return out
}

// lambdaMetricSpecs returns the CloudWatch metrics shown for a Lambda function.
func lambdaMetricSpecs(fn string) []metricSpec {
	dim := map[string]string{"FunctionName": fn}
	return []metricSpec{
		{Title: "Invocations", Namespace: "AWS/Lambda", MetricName: "Invocations", Dimensions: dim, Stat: "Sum"},
		{Title: "Errors", Namespace: "AWS/Lambda", MetricName: "Errors", Dimensions: dim, Stat: "Sum"},
		{Title: "Duration (avg ms)", Namespace: "AWS/Lambda", MetricName: "Duration", Dimensions: dim, Stat: "Average"},
		{Title: "Throttles", Namespace: "AWS/Lambda", MetricName: "Throttles", Dimensions: dim, Stat: "Sum"},
	}
}
