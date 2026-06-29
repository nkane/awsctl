package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConfirmResult is the outcome of a key handled by an active Confirm modal.
type ConfirmResult int

const (
	ConfirmNone ConfirmResult = iota // still open, no decision yet
	ConfirmYes                       // user confirmed
	ConfirmNo                        // user cancelled
)

// ConfirmTheme styles the modal. audit-wiring builds this from the app Theme.
type ConfirmTheme struct {
	Box    lipgloss.Style // the bordered box
	Title  lipgloss.Style // title line
	Body   lipgloss.Style // prompt body text
	Active lipgloss.Style // highlighted (selected) button
	Button lipgloss.Style // unselected button
}

// cursor positions within the button row.
const (
	confirmCursorCancel = 0
	confirmCursorYes    = 1
)

// Confirm is a yes/no modal overlay that gates destructive actions. Value type
// (like Tabs / StatusBar): every method returns an updated copy; no pointers.
type Confirm struct {
	theme  ConfirmTheme
	title  string
	body   string
	active bool
	cursor int
}

// NewConfirm builds an inactive modal with the given theme.
func NewConfirm(theme ConfirmTheme) Confirm {
	return Confirm{theme: theme}
}

// Show opens the modal with the given title and prompt body. Cursor defaults to
// the SAFE choice (Cancel), so a stray Enter does not confirm.
func (c Confirm) Show(title, body string) Confirm {
	c.title = title
	c.body = body
	c.active = true
	c.cursor = confirmCursorCancel
	return c
}

// Hide closes the modal (used on cancel-from-outside; Update auto-hides on a
// decision).
func (c Confirm) Hide() Confirm {
	c.active = false
	return c
}

// Active reports whether the modal is open. While true the App routes all keys
// to Update and suppresses global shortcuts.
func (c Confirm) Active() bool {
	return c.active
}

// Update handles one key while the modal is open. Returns the updated modal and
// the result. On ConfirmYes or ConfirmNo the returned modal is auto-hidden
// (Active()==false). Key map:
//
//	left/right/h/l/tab  move cursor between Cancel and Confirm
//	enter               selects the cursor's choice
//	y                   ConfirmYes        n / esc   ConfirmNo
//
// Any other key returns (c, ConfirmNone) unchanged.
func (c Confirm) Update(msg tea.KeyMsg) (Confirm, ConfirmResult) {
	switch msg.String() {
	case "left", "h":
		c.cursor = confirmCursorCancel
		return c, ConfirmNone
	case "right", "l":
		c.cursor = confirmCursorYes
		return c, ConfirmNone
	case "tab":
		if c.cursor == confirmCursorCancel {
			c.cursor = confirmCursorYes
		} else {
			c.cursor = confirmCursorCancel
		}
		return c, ConfirmNone
	case "enter":
		if c.cursor == confirmCursorYes {
			return c.Hide(), ConfirmYes
		}
		return c.Hide(), ConfirmNo
	case "y":
		return c.Hide(), ConfirmYes
	case "n", "esc":
		return c.Hide(), ConfirmNo
	default:
		return c, ConfirmNone
	}
}

// View renders the modal centered within w x h. Returns "" when not Active.
func (c Confirm) View(w, h int) string {
	if !c.active {
		return ""
	}

	cancel := "[ Cancel ]"
	confirm := "[ Confirm ]"
	if c.cursor == confirmCursorYes {
		cancel = c.theme.Button.Render(cancel)
		confirm = c.theme.Active.Render(confirm)
	} else {
		cancel = c.theme.Active.Render(cancel)
		confirm = c.theme.Button.Render(confirm)
	}
	buttons := lipgloss.JoinHorizontal(lipgloss.Top, cancel, " ", confirm)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		c.theme.Title.Render(c.title),
		c.theme.Body.Render(c.body),
		"",
		buttons,
	)

	box := c.theme.Box.Render(content)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}
