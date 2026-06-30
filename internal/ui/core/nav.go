package core

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

// ConfirmRequestMsg asks the App to gate a mutation behind the confirm modal.
// A screen emits this (via ConfirmRequest) when the user presses a write key;
// the App is the single owner of the modal and the --unsafe gate, so screens
// never reach across to open it themselves. In read-only mode the App refuses.
// Title/Body are shown in the modal; Action/Target label the audit record; Run
// is the command executed only if the user confirms.
type ConfirmRequestMsg struct {
	Title  string
	Body   string
	Action string
	Target string
	Run    tea.Cmd
}

// ConfirmRequest returns a command that emits a ConfirmRequestMsg.
func ConfirmRequest(title, body, action, target string, run tea.Cmd) tea.Cmd {
	return func() tea.Msg {
		return ConfirmRequestMsg{Title: title, Body: body, Action: action, Target: target, Run: run}
	}
}
