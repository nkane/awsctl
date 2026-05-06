package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Tabs renders a horizontal tab strip.
type Tabs struct {
	Items    []string
	Active   int
	Active1  lipgloss.Style
	Inactive lipgloss.Style
}

// View renders the tabs.
func (t Tabs) View() string {
	parts := make([]string, len(t.Items))
	for i, name := range t.Items {
		if i == t.Active {
			parts[i] = t.Active1.Render(name)
		} else {
			parts[i] = t.Inactive.Render(name)
		}
	}
	return strings.Join(parts, " ")
}
