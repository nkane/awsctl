// Package profile renders a picker that lets the user choose an AWS profile
// and region. On confirm it emits a Selected message that the root model
// uses to reload the AWS config.
package profile

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsx "github.com/nkane/awsctl/internal/aws"
)

// Selected is emitted when the user confirms a profile + region.
type Selected struct {
	Profile string
	Region  string
}

// Cancelled is emitted when the user dismisses the picker.
type Cancelled struct{}

type focus int

const (
	focusProfile focus = iota
	focusRegion
)

// Model is the profile/region picker.
type Model struct {
	profiles list.Model
	region   textinput.Model
	focus    focus
	width    int
	height   int
}

type item string

func (i item) FilterValue() string { return string(i) }
func (i item) Title() string       { return string(i) }
func (i item) Description() string { return "" }

// New constructs a picker pre-populated with profiles from disk and a
// suggested region.
func New(currentProfile, currentRegion string) Model {
	profiles, _ := awsx.ListProfiles()
	items := make([]list.Item, 0, len(profiles))
	for _, p := range profiles {
		items = append(items, item(p))
	}
	d := list.NewDefaultDelegate()
	d.ShowDescription = false
	l := list.New(items, d, 30, 12)
	l.Title = "AWS Profile"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)

	// Pre-select current profile if present.
	for i, it := range items {
		if string(it.(item)) == currentProfile {
			l.Select(i)
			break
		}
	}

	ti := textinput.New()
	ti.Placeholder = "us-east-1"
	ti.SetValue(currentRegion)
	ti.CharLimit = 32
	ti.Width = 24

	return Model{profiles: l, region: ti, focus: focusProfile}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// Reserve some chrome.
		m.profiles.SetSize(min(40, msg.Width-4), max(8, msg.Height-10))
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return Cancelled{} }
		case "tab":
			if m.focus == focusProfile {
				m.focus = focusRegion
				m.region.Focus()
			} else {
				m.focus = focusProfile
				m.region.Blur()
			}
			return m, nil
		case "enter":
			prof := ""
			if it, ok := m.profiles.SelectedItem().(item); ok {
				prof = string(it)
			}
			region := m.region.Value()
			return m, func() tea.Msg { return Selected{Profile: prof, Region: region} }
		}
	}
	var cmd tea.Cmd
	if m.focus == focusProfile {
		m.profiles, cmd = m.profiles.Update(msg)
	} else {
		m.region, cmd = m.region.Update(msg)
	}
	return m, cmd
}

// View implements tea.Model.
func (m Model) View() string {
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("tab: switch field · enter: confirm · esc: cancel")
	regionLabel := lipgloss.NewStyle().Bold(true).Render("Region")
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
	body := lipgloss.JoinVertical(lipgloss.Left,
		m.profiles.View(),
		"",
		regionLabel,
		m.region.View(),
		"",
		hint,
	)
	return box.Render(body)
}

func min(a, b int) int { if a < b { return a }; return b }
func max(a, b int) int { if a > b { return a }; return b }
