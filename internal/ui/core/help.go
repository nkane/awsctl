package core

import "github.com/charmbracelet/bubbles/key"

// Hint builds a key.Binding for the help/menu display. keyLabel is what the
// user presses (also used as the help key column); desc is the action label.
func Hint(keyLabel, desc string) key.Binding {
	return key.NewBinding(key.WithKeys(keyLabel), key.WithHelp(keyLabel, desc))
}
