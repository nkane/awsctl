package ui

import tea "github.com/charmbracelet/bubbletea"

// Navigation messages. Screens request a stack change by emitting one of these
// as a command; the App is the single owner that applies it to the active
// Stack. This keeps Bubble Tea's update loop pure — no screen reaches across to
// mutate another's state.

// PushMsg asks the App to push Screen onto the active mode's stack.
// The pushed screen is sized and Init()'d by the App.
type PushMsg struct{ Screen Screen }

// PopMsg asks the App to pop the active mode's top screen (back / esc).
type PopMsg struct{}

// Push returns a command that emits a PushMsg.
func Push(scr Screen) tea.Cmd {
	return func() tea.Msg { return PushMsg{Screen: scr} }
}

// Pop returns a command that emits a PopMsg.
func Pop() tea.Cmd {
	return func() tea.Msg { return PopMsg{} }
}
