package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// StatusBar renders the bottom status bar.
// It shows profile, region, mode (read or UNSAFE), and the last error if any.
type StatusBar struct {
	Profile  string
	Region   string
	Unsafe   bool
	LastErr  string
	Width    int
	Theme    StatusTheme
}

// StatusTheme is the subset of theme styles the status bar needs.
// Defined locally so this package has no import cycle on internal/ui.
type StatusTheme struct {
	Bar       lipgloss.Style
	Key       lipgloss.Style
	Val       lipgloss.Style
	UnsafeTag lipgloss.Style
	SafeTag   lipgloss.Style
	ErrorTag  lipgloss.Style
}

// View renders the status bar to a single line of width sb.Width.
func (sb StatusBar) View() string {
	mode := sb.Theme.SafeTag.Render("READ")
	if sb.Unsafe {
		mode = sb.Theme.UnsafeTag.Render("UNSAFE")
	}
	left := strings.Join([]string{
		mode,
		sb.Theme.Key.Render("profile:") + " " + sb.Theme.Val.Render(orDash(sb.Profile)),
		sb.Theme.Key.Render("region:") + " " + sb.Theme.Val.Render(orDash(sb.Region)),
	}, "  ")
	right := ""
	if sb.LastErr != "" {
		right = sb.Theme.ErrorTag.Render("err: " + truncate(sb.LastErr, 60))
	}
	gap := sb.Width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return sb.Theme.Bar.Width(sb.Width).Render(left + strings.Repeat(" ", gap) + right)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
