package core

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Screen is a single push/pop-able view in the navigation stack.
//
// It mirrors tea.Model but (a) returns the Screen interface from Update so the
// stack can hold heterogeneous screens, and (b) adds Title/KeyHints/SetSize so
// the chrome (breadcrumbs, menu hints, layout) can be driven generically — the
// k9s pattern, adapted to Bubble Tea's immutable update loop.
//
// Screens never mutate the stack directly. To drill in or back out they emit a
// PushMsg / PopMsg command (see nav.go); the App owns the stack and applies it.
type Screen interface {
	// Init returns the screen's initial command (first fetch, spinner tick…).
	Init() tea.Cmd
	// Update handles a message and returns the (possibly new) screen value.
	Update(tea.Msg) (Screen, tea.Cmd)
	// View renders the screen body (between header and status bar).
	View() string
	// Title is the breadcrumb label for this screen (e.g. "services").
	Title() string
	// KeyHints are the screen-local bindings shown in the menu / help overlay.
	KeyHints() []key.Binding
	// SetSize informs the screen of the available body dimensions.
	SetSize(w, h int)
}

// InputCapturer is an optional Screen capability. When CapturesInput reports
// true, the App forwards every key (except ctrl+c) straight to the screen and
// suppresses global shortcuts — e.g. a text editor or an active filter, where
// keys like "1"/"2"/"q" are literal input, not navigation.
type InputCapturer interface {
	CapturesInput() bool
}

// EscHandler is an optional Screen capability. When WantsEsc reports true, the
// App forwards `esc` to the screen (to cancel an in-screen mode such as an
// active filter) instead of popping the stack.
type EscHandler interface {
	WantsEsc() bool
}
