// Package ui contains the Bubble Tea models that compose awsctl.
package ui

import (
	"context"
	"fmt"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsx "github.com/nkane/awsctl/internal/aws"
	"github.com/nkane/awsctl/internal/ui/components"
	dynamoui "github.com/nkane/awsctl/internal/ui/dynamo"
	lambdaui "github.com/nkane/awsctl/internal/ui/lambda"
	"github.com/nkane/awsctl/internal/ui/profile"
)

// Mode is the top-level screen.
type Mode int

const (
	ModeLambda Mode = iota
	ModeDynamo
	ModeProfile
)

// Options configures the root model from main.
type Options struct {
	Profile string
	Region  string
	Unsafe  bool
	Logger  *slog.Logger
}

// App is the root Bubble Tea model.
type App struct {
	opts         Options
	cfg          *awsx.Config
	mode         Mode
	prevMode     Mode
	picker       profile.Model
	lambdas      lambdaui.ListModel
	lambdaDetail *lambdaui.DetailModel // nil unless a function is being viewed
	lambdaInvoke *lambdaui.InvokeModel // nil unless invoke screen is open
	lambdaLogs   *lambdaui.LogsModel   // nil unless log tail is open

	tables       dynamoui.ListModel
	tableDescribe *dynamoui.DescribeModel // nil unless describe is open
	tableScan    *dynamoui.ScanModel      // nil unless scan is open
	tableQuery   *dynamoui.QueryModel     // nil unless query is open

	width  int
	height int

	theme  Theme
	keys   KeyMap
	tabs   components.Tabs
	status components.StatusBar

	lastErr  string
	quitting bool
}

// awsConfigLoadedMsg is delivered after Load completes.
type awsConfigLoadedMsg struct {
	cfg *awsx.Config
	err error
}

// loadAWSConfigCmd asynchronously loads an aws.Config.
func loadAWSConfigCmd(profile, region string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := awsx.Load(context.Background(), profile, region)
		return awsConfigLoadedMsg{cfg: cfg, err: err}
	}
}

// NewApp constructs the root model.
func NewApp(opts Options) App {
	theme := NewTheme()
	return App{
		opts:    opts,
		mode:    ModeLambda,
		theme:   theme,
		keys:    DefaultKeys(),
		lambdas: lambdaui.NewList(),
		tables:  dynamoui.NewList(),
		tabs: components.Tabs{
			Items:    []string{"[1] Lambda", "[2] DynamoDB"},
			Active:   0,
			Active1:  theme.TabActive,
			Inactive: theme.TabInactiv,
		},
		status: components.StatusBar{
			Profile: opts.Profile,
			Region:  opts.Region,
			Unsafe:  opts.Unsafe,
			Theme: components.StatusTheme{
				Bar:       theme.StatusBar,
				Key:       theme.StatusKey,
				Val:       theme.StatusVal,
				UnsafeTag: theme.UnsafeTag,
				SafeTag:   theme.SafeTag,
				ErrorTag:  theme.ErrorTag,
			},
		},
	}
}

// Init implements tea.Model.
func (a App) Init() tea.Cmd {
	return tea.Batch(
		tea.SetWindowTitle("awsctl"),
		loadAWSConfigCmd(a.opts.Profile, a.opts.Region),
	)
}

// contentSize returns the width/height available to a body screen between
// the header (1 line) and status bar (1 line).
func (a App) contentSize() (int, int) {
	h := a.height - 3
	if h < 3 {
		h = 3
	}
	return a.width, h
}

// Update implements tea.Model.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.status.Width = msg.Width
		w, h := a.contentSize()
		a.lambdas.SetSize(w, h)
		if a.lambdaDetail != nil {
			a.lambdaDetail.SetSize(w, h)
		}
		if a.lambdaInvoke != nil {
			a.lambdaInvoke.SetSize(w, h)
		}
		if a.lambdaLogs != nil {
			a.lambdaLogs.SetSize(w, h)
		}
		a.tables.SetSize(w, h)
		if a.tableDescribe != nil {
			a.tableDescribe.SetSize(w, h)
		}
		if a.tableScan != nil {
			a.tableScan.SetSize(w, h)
		}
		if a.tableQuery != nil {
			a.tableQuery.SetSize(w, h)
		}
		// Only forward to picker if it has been constructed (mode==Profile).
		if a.mode == ModeProfile {
			var cmd tea.Cmd
			a.picker, cmd = a.picker.Update(msg)
			return a, cmd
		}
		return a, nil

	case awsConfigLoadedMsg:
		if msg.err != nil {
			a.lastErr = msg.err.Error()
			a.opts.Logger.Error("aws config load failed", "err", msg.err)
			return a, nil
		}
		a.cfg = msg.cfg
		a.status.Profile = msg.cfg.Profile
		a.status.Region = msg.cfg.Region
		a.lastErr = ""
		a.opts.Logger.Info("aws config loaded", "profile", msg.cfg.Profile, "region", msg.cfg.Region)
		// Hand a fresh Lambda client to the list screen and trigger first fetch.
		a.lambdas.SetClient(awsx.NewLambdaClient(msg.cfg))
		a.tables.SetClient(awsx.NewDynamoClient(msg.cfg))
		return a, tea.Batch(a.lambdas.Refresh(), a.tables.Refresh())

	case profile.Selected:
		a.mode = a.prevMode
		return a, loadAWSConfigCmd(msg.Profile, msg.Region)

	case profile.Cancelled:
		a.mode = a.prevMode
		return a, nil

	case tea.KeyMsg:
		// Picker owns input while open.
		if a.mode == ModeProfile {
			var cmd tea.Cmd
			a.picker, cmd = a.picker.Update(msg)
			return a, cmd
		}
		// Lambda logs owns input when open.
		if a.mode == ModeLambda && a.lambdaLogs != nil {
			switch msg.String() {
			case "ctrl+c":
				a.quitting = true
				return a, tea.Quit
			case "esc":
				// If filter input is active, let logs model handle esc.
				if !a.lambdaLogs.FilterFocused() {
					a.lambdaLogs = nil
					return a, nil
				}
			}
			lg, cmd := a.lambdaLogs.Update(msg)
			a.lambdaLogs = &lg
			return a, cmd
		}
		// Lambda invoke owns input when open.
		if a.mode == ModeLambda && a.lambdaInvoke != nil {
			switch {
			case keyMatches(msg, a.keys.Quit) && msg.String() != "q":
				// only ctrl+c quits while editing; 'q' is a valid char in payload.
				a.quitting = true
				return a, tea.Quit
			case msg.String() == "esc":
				a.lambdaInvoke = nil
				return a, nil
			}
			inv, cmd := a.lambdaInvoke.Update(msg)
			a.lambdaInvoke = &inv
			return a, cmd
		}
		// Lambda detail owns input when open (except global tab/profile/quit).
		if a.mode == ModeLambda && a.lambdaDetail != nil {
			switch {
			case keyMatches(msg, a.keys.Quit):
				a.quitting = true
				return a, tea.Quit
			case keyMatches(msg, a.keys.Profile):
				a.prevMode = a.mode
				a.mode = ModeProfile
				a.picker = profile.New(a.status.Profile, a.status.Region)
				var cmd tea.Cmd
				a.picker, cmd = a.picker.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
				return a, tea.Batch(a.picker.Init(), cmd)
			case msg.String() == "esc":
				a.lambdaDetail = nil
				return a, nil
			case msg.String() == "i":
				name := a.lambdaDetail.Name()
				if name != "" && a.cfg != nil {
					inv := lambdaui.NewInvoke(awsx.NewLambdaClient(a.cfg), name)
					w, h := a.contentSize()
					inv.SetSize(w, h)
					a.lambdaInvoke = &inv
				}
				return a, nil
			case msg.String() == "l":
				name := a.lambdaDetail.Name()
				if name != "" && a.cfg != nil {
					lg := lambdaui.NewLogs(awsx.NewLogsClient(a.cfg), name)
					w, h := a.contentSize()
					lg.SetSize(w, h)
					a.lambdaLogs = &lg
					return a, lg.Init()
				}
				return a, nil
			case keyMatches(msg, a.keys.Dynamo):
				a.mode = ModeDynamo
				a.tabs.Active = 1
				return a, nil
			}
			d, cmd := a.lambdaDetail.Update(msg)
			a.lambdaDetail = &d
			return a, cmd
		}
		// Dynamo query owns input when open (textinputs need most keys).
		if a.mode == ModeDynamo && a.tableQuery != nil {
			switch msg.String() {
			case "ctrl+c":
				a.quitting = true
				return a, tea.Quit
			case "esc":
				a.tableQuery = nil
				return a, nil
			}
			qm, cmd := a.tableQuery.Update(msg)
			a.tableQuery = &qm
			return a, cmd
		}
		// Dynamo scan owns input when open.
		if a.mode == ModeDynamo && a.tableScan != nil {
			switch {
			case keyMatches(msg, a.keys.Quit):
				a.quitting = true
				return a, tea.Quit
			case keyMatches(msg, a.keys.Profile):
				a.prevMode = a.mode
				a.mode = ModeProfile
				a.picker = profile.New(a.status.Profile, a.status.Region)
				var cmd tea.Cmd
				a.picker, cmd = a.picker.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
				return a, tea.Batch(a.picker.Init(), cmd)
			case msg.String() == "esc":
				a.tableScan = nil
				return a, nil
			case keyMatches(msg, a.keys.Lambda):
				a.mode = ModeLambda
				a.tabs.Active = 0
				return a, nil
			}
			s, cmd := a.tableScan.Update(msg)
			a.tableScan = &s
			return a, cmd
		}
		// Dynamo describe owns input when open.
		if a.mode == ModeDynamo && a.tableDescribe != nil {
			switch {
			case keyMatches(msg, a.keys.Quit):
				a.quitting = true
				return a, tea.Quit
			case keyMatches(msg, a.keys.Profile):
				a.prevMode = a.mode
				a.mode = ModeProfile
				a.picker = profile.New(a.status.Profile, a.status.Region)
				var cmd tea.Cmd
				a.picker, cmd = a.picker.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
				return a, tea.Batch(a.picker.Init(), cmd)
			case msg.String() == "esc":
				a.tableDescribe = nil
				return a, nil
			case msg.String() == "s":
				name := a.tableDescribe.Name()
				if name != "" && a.cfg != nil {
					s := dynamoui.NewScan(awsx.NewDynamoClient(a.cfg), name)
					w, h := a.contentSize()
					s.SetSize(w, h)
					a.tableScan = &s
					return a, s.Init()
				}
				return a, nil
			case msg.String() == "Q":
				name := a.tableDescribe.Name()
				if name != "" && a.cfg != nil {
					qm := dynamoui.NewQuery(awsx.NewDynamoClient(a.cfg), name)
					w, h := a.contentSize()
					qm.SetSize(w, h)
					a.tableQuery = &qm
					return a, qm.Init()
				}
				return a, nil
			case keyMatches(msg, a.keys.Lambda):
				a.mode = ModeLambda
				a.tabs.Active = 0
				return a, nil
			}
			d, cmd := a.tableDescribe.Update(msg)
			a.tableDescribe = &d
			return a, cmd
		}
		switch {
		case keyMatches(msg, a.keys.Quit):
			// Don't quit if a list is currently filtering.
			if a.mode == ModeLambda && a.lambdas.IsFiltering() {
				break
			}
			if a.mode == ModeDynamo && a.tables.IsFiltering() {
				break
			}
			a.quitting = true
			return a, tea.Quit
		case keyMatches(msg, a.keys.Lambda):
			a.mode = ModeLambda
			a.tabs.Active = 0
			return a, nil
		case keyMatches(msg, a.keys.Dynamo):
			a.mode = ModeDynamo
			a.tabs.Active = 1
			return a, nil
		case keyMatches(msg, a.keys.Profile):
			a.prevMode = a.mode
			a.mode = ModeProfile
			a.picker = profile.New(a.status.Profile, a.status.Region)
			// Seed picker with current size so its list renders.
			var cmd tea.Cmd
			a.picker, cmd = a.picker.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
			return a, tea.Batch(a.picker.Init(), cmd)
		case msg.String() == "enter" && a.mode == ModeLambda && !a.lambdas.IsFiltering():
			sel := a.lambdas.Selected()
			if sel.Name == "" || a.cfg == nil {
				return a, nil
			}
			d := lambdaui.NewDetail(awsx.NewLambdaClient(a.cfg), sel.Name)
			w, h := a.contentSize()
			d.SetSize(w, h)
			a.lambdaDetail = &d
			return a, d.Init()
		case msg.String() == "i" && a.mode == ModeLambda && !a.lambdas.IsFiltering():
			sel := a.lambdas.Selected()
			if sel.Name == "" || a.cfg == nil {
				return a, nil
			}
			inv := lambdaui.NewInvoke(awsx.NewLambdaClient(a.cfg), sel.Name)
			w, h := a.contentSize()
			inv.SetSize(w, h)
			a.lambdaInvoke = &inv
			return a, nil
		case msg.String() == "l" && a.mode == ModeLambda && !a.lambdas.IsFiltering():
			sel := a.lambdas.Selected()
			if sel.Name == "" || a.cfg == nil {
				return a, nil
			}
			lg := lambdaui.NewLogs(awsx.NewLogsClient(a.cfg), sel.Name)
			w, h := a.contentSize()
			lg.SetSize(w, h)
			a.lambdaLogs = &lg
			return a, lg.Init()
		case msg.String() == "enter" && a.mode == ModeDynamo && !a.tables.IsFiltering():
			name := a.tables.Selected()
			if name == "" || a.cfg == nil {
				return a, nil
			}
			d := dynamoui.NewDescribe(awsx.NewDynamoClient(a.cfg), name)
			w, h := a.contentSize()
			d.SetSize(w, h)
			a.tableDescribe = &d
			return a, d.Init()
		case msg.String() == "s" && a.mode == ModeDynamo && !a.tables.IsFiltering():
			name := a.tables.Selected()
			if name == "" || a.cfg == nil {
				return a, nil
			}
			s := dynamoui.NewScan(awsx.NewDynamoClient(a.cfg), name)
			w, h := a.contentSize()
			s.SetSize(w, h)
			a.tableScan = &s
			return a, s.Init()
		case msg.String() == "Q" && a.mode == ModeDynamo && !a.tables.IsFiltering():
			name := a.tables.Selected()
			if name == "" || a.cfg == nil {
				return a, nil
			}
			qm := dynamoui.NewQuery(awsx.NewDynamoClient(a.cfg), name)
			w, h := a.contentSize()
			qm.SetSize(w, h)
			a.tableQuery = &qm
			return a, qm.Init()
		}
		// Forward unhandled keys to the active screen.
		if a.mode == ModeLambda {
			var cmd tea.Cmd
			a.lambdas, cmd = a.lambdas.Update(msg)
			return a, cmd
		}
		if a.mode == ModeDynamo {
			var cmd tea.Cmd
			a.tables, cmd = a.tables.Update(msg)
			return a, cmd
		}
	}

	// Forward non-key messages (spinner ticks, loadedMsg, detailLoadedMsg) to
	// both the list and detail screens so background loads complete regardless
	// of which is currently visible.
	var cmd1, cmd2, cmd3, cmd4, cmd5, cmd6, cmd7, cmd8 tea.Cmd
	a.lambdas, cmd1 = a.lambdas.Update(msg)
	if a.lambdaDetail != nil {
		d, c := a.lambdaDetail.Update(msg)
		a.lambdaDetail = &d
		cmd2 = c
	}
	if a.lambdaInvoke != nil {
		inv, c := a.lambdaInvoke.Update(msg)
		a.lambdaInvoke = &inv
		cmd3 = c
	}
	if a.lambdaLogs != nil {
		lg, c := a.lambdaLogs.Update(msg)
		a.lambdaLogs = &lg
		cmd4 = c
	}
	a.tables, cmd5 = a.tables.Update(msg)
	if a.tableDescribe != nil {
		d, c := a.tableDescribe.Update(msg)
		a.tableDescribe = &d
		cmd6 = c
	}
	if a.tableScan != nil {
		s, c := a.tableScan.Update(msg)
		a.tableScan = &s
		cmd7 = c
	}
	if a.tableQuery != nil {
		qm, c := a.tableQuery.Update(msg)
		a.tableQuery = &qm
		cmd8 = c
	}
	return a, tea.Batch(cmd1, cmd2, cmd3, cmd4, cmd5, cmd6, cmd7, cmd8)
}

// View implements tea.Model.
func (a App) View() string {
	if a.quitting {
		return ""
	}
	if a.mode == ModeProfile {
		return centered(a.width, a.height, a.picker.View()) + "\n" + a.statusView()
	}

	header := a.theme.Title.Render("awsctl") + "  " + a.tabs.View()
	body := ""
	switch a.mode {
	case ModeLambda:
		if a.lambdaLogs != nil {
			body = a.lambdaLogs.View()
		} else if a.lambdaInvoke != nil {
			body = a.lambdaInvoke.View()
		} else if a.lambdaDetail != nil {
			body = a.lambdaDetail.View()
		} else {
			body = a.lambdas.View()
		}
	case ModeDynamo:
		if a.tableQuery != nil {
			body = a.tableQuery.View()
		} else if a.tableScan != nil {
			body = a.tableScan.View()
		} else if a.tableDescribe != nil {
			body = a.tableDescribe.View()
		} else {
			body = a.tables.View()
		}
	}

	w, h := a.contentSize()
	mid := lipgloss.NewStyle().Width(w).Height(h).Render(body)
	return header + "\n" + mid + "\n" + a.statusView()
}

func (a App) statusView() string {
	a.status.LastErr = a.lastErr
	return a.status.View()
}

func placeholder(title, sub string) string {
	t := lipgloss.NewStyle().Bold(true).Render(title)
	s := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(sub)
	return fmt.Sprintf("\n  %s\n  %s\n", t, s)
}

func centered(w, h int, s string) string {
	return lipgloss.Place(w, h-2, lipgloss.Center, lipgloss.Center, s)
}
