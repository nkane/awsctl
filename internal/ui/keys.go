package ui

import "github.com/charmbracelet/bubbles/key"

// KeyMap is the global keymap. Screen-local maps may shadow these.
type KeyMap struct {
	Lambda  key.Binding
	Dynamo  key.Binding
	Profile key.Binding
	Help    key.Binding
	Quit    key.Binding
	Back    key.Binding
	Up      key.Binding
	Down    key.Binding
	Top     key.Binding
	Bottom  key.Binding
	Filter  key.Binding
	Enter   key.Binding
	Tab     key.Binding
	More    key.Binding
}

// DefaultKeys returns the vim-ish global keymap.
func DefaultKeys() KeyMap {
	return KeyMap{
		Lambda:  key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "lambda")),
		Dynamo:  key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "dynamo")),
		Profile: key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "profile")),
		Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Back:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Up:      key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k", "up")),
		Down:    key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j", "down")),
		Top:     key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
		Bottom:  key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
		Filter:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Enter:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Tab:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next tab")),
		More:    key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "load more")),
	}
}
