package components

import (
	"github.com/NimbleMarkets/ntcharts/linechart/timeserieslinechart"
	"github.com/charmbracelet/lipgloss"

	awsx "github.com/nkane/awsctl/internal/aws"
)

// emptyChartMsg is shown centered when a series has no datapoints.
const emptyChartMsg = "no datapoints in range"

// Default chart dimensions used when SetSize has not been called.
const (
	defaultChartWidth  = 40
	defaultChartHeight = 8
)

var chartTitleStyle = lipgloss.NewStyle().Bold(true)

// Chart wraps an ntcharts timeserieslinechart and a title. It renders a single
// MetricSeries. The struct has value semantics: WithSeries returns an updated
// copy and the underlying ntcharts model is rebuilt on each View, so copies
// never share mutable chart state.
type Chart struct {
	title  string
	width  int
	height int
	series awsx.MetricSeries
}

// NewChart returns a Chart with the given title and no data.
func NewChart(title string) Chart {
	return Chart{title: title}
}

// SetSize sets the render dimensions (including the title line).
func (c *Chart) SetSize(w, h int) {
	c.width = w
	c.height = h
}

// WithSeries returns a copy of the chart bound to the given series.
func (c Chart) WithSeries(s awsx.MetricSeries) Chart {
	c.series = s
	return c
}

// View renders the title above the plotted series. When the series has no
// points it renders a centered "no datapoints in range" placeholder.
func (c Chart) View() string {
	return lipgloss.JoinVertical(lipgloss.Left, chartTitleStyle.Render(c.title), c.body())
}

// body renders the graph area (everything below the title line).
func (c Chart) body() string {
	w, h := c.width, c.height
	if w <= 0 {
		w = defaultChartWidth
	}
	if h <= 1 {
		h = defaultChartHeight
	}
	graphHeight := h - 1
	if graphHeight < 1 {
		graphHeight = 1
	}

	if len(c.series.Points) == 0 {
		pw := w
		if pw < len(emptyChartMsg) {
			pw = len(emptyChartMsg)
		}
		return lipgloss.Place(pw, graphHeight, lipgloss.Center, lipgloss.Center, emptyChartMsg)
	}

	lc := timeserieslinechart.New(w, graphHeight)
	for _, p := range c.series.Points {
		lc.Push(timeserieslinechart.TimePoint{Time: p.Timestamp, Value: p.Value})
	}
	lc.Draw()
	return lc.View()
}
