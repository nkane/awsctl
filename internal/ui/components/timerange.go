package components

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// RangeOption is a selectable lookback window. Period is the metric bucket
// width screen-integration feeds into MetricQuery.Period.
type RangeOption struct {
	Label    string        // "1h", "3h", "12h", "24h", "7d"
	Duration time.Duration // lookback from now
	Period   time.Duration // bucket width
}

// TimeRange is a horizontal selector over a fixed set of RangeOptions.
type TimeRange struct {
	options []RangeOption
	cursor  int
}

// DefaultRangeOptions returns the canonical lookback windows. The Period for
// each window is owned here; screen-integration depends on these values.
func DefaultRangeOptions() []RangeOption {
	return []RangeOption{
		{Label: "1h", Duration: time.Hour, Period: time.Minute},
		{Label: "3h", Duration: 3 * time.Hour, Period: 5 * time.Minute},
		{Label: "12h", Duration: 12 * time.Hour, Period: 5 * time.Minute},
		{Label: "24h", Duration: 24 * time.Hour, Period: 15 * time.Minute},
		{Label: "7d", Duration: 7 * 24 * time.Hour, Period: time.Hour},
	}
}

// NewTimeRange returns a selector over the default options with the cursor on "3h".
func NewTimeRange() TimeRange {
	opts := DefaultRangeOptions()
	cursor := 0
	for i, o := range opts {
		if o.Label == "3h" {
			cursor = i
			break
		}
	}
	return TimeRange{options: opts, cursor: cursor}
}

var (
	trLeft  = key.NewBinding(key.WithKeys("left", "h"))
	trRight = key.NewBinding(key.WithKeys("right", "l"))
)

// Update moves the cursor on left/right (or h/l). The bool reports whether the
// selection actually changed (false at the boundaries or on any other message).
func (t TimeRange) Update(msg tea.Msg) (TimeRange, bool) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return t, false
	}
	switch {
	case key.Matches(km, trLeft):
		if t.cursor > 0 {
			t.cursor--
			return t, true
		}
	case key.Matches(km, trRight):
		if t.cursor < len(t.options)-1 {
			t.cursor++
			return t, true
		}
	}
	return t, false
}

// Selected returns the currently highlighted option.
func (t TimeRange) Selected() RangeOption {
	if len(t.options) == 0 {
		return RangeOption{}
	}
	if t.cursor < 0 || t.cursor >= len(t.options) {
		return t.options[0]
	}
	return t.options[t.cursor]
}

var (
	pillSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Padding(0, 1).
				Foreground(lipgloss.Color("0")).
				Background(lipgloss.Color("12"))
	pillStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(lipgloss.Color("7"))
)

// View renders the options as horizontal pills with the selected one highlighted.
func (t TimeRange) View() string {
	parts := make([]string, len(t.options))
	for i, o := range t.options {
		if i == t.cursor {
			parts[i] = pillSelectedStyle.Render(o.Label)
		} else {
			parts[i] = pillStyle.Render(o.Label)
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}
