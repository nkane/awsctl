// Package ui contains the Bubble Tea models that compose awsctl.
package ui

import (
	"context"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsx "github.com/nkane/awsctl/internal/aws"
	"github.com/nkane/awsctl/internal/ui/components"
	"github.com/nkane/awsctl/internal/ui/core"
	dynamoui "github.com/nkane/awsctl/internal/ui/dynamo"
	ecsui "github.com/nkane/awsctl/internal/ui/ecs"
	lambdaui "github.com/nkane/awsctl/internal/ui/lambda"
	"github.com/nkane/awsctl/internal/ui/profile"
)

// Mode is the top-level screen.
type Mode int

const (
	ModeLambda Mode = iota
	ModeDynamo
	ModeEcs
	ModeProfile
)

// Options configures the root model from main.
type Options struct {
	Profile string
	Region  string
	Unsafe  bool
	Logger  *slog.Logger
}

// App is the root Bubble Tea model. Each non-profile mode owns a navigation
// Stack whose bottom is that mode's root list screen; drilling in pushes a
// child screen, `esc` pops. The App handles global navigation and delegates
// everything else to the active stack's top screen.
type App struct {
	opts     Options
	cfg      *awsx.Config
	mode     Mode
	prevMode Mode
	picker   profile.Model

	lstack *core.Stack       // Lambda navigation stack
	dstack *core.Stack       // Dynamo navigation stack
	estack *core.Stack       // ECS navigation stack
	lroot  lambdaui.RootList // == lstack bottom; kept for client seeding
	droot  dynamoui.RootList // == dstack bottom; kept for client seeding
	eroot  ecsui.RootList    // == estack bottom; kept for client seeding

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
	lroot := lambdaui.NewListScreen()
	droot := dynamoui.NewListScreen()
	eroot := ecsui.NewListScreen()
	return App{
		opts:   opts,
		mode:   ModeLambda,
		theme:  theme,
		keys:   DefaultKeys(),
		lroot:  lroot,
		droot:  droot,
		eroot:  eroot,
		lstack: core.NewStack(lroot),
		dstack: core.NewStack(droot),
		estack: core.NewStack(eroot),
		tabs: components.Tabs{
			Items:    []string{"[1] Lambda", "[2] DynamoDB", "[3] ECS"},
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

// active returns the navigation stack for the current mode.
func (a App) active() *core.Stack { return a.stackFor(a.mode) }

// stackFor returns the navigation stack for the given mode.
func (a App) stackFor(m Mode) *core.Stack {
	switch m {
	case ModeDynamo:
		return a.dstack
	case ModeEcs:
		return a.estack
	default:
		return a.lstack
	}
}

// push pushes a screen onto the active stack, sizes it, and returns its Init
// command. A nil screen (e.g. a builder with no selection) is a no-op.
func (a App) push(s core.Screen) tea.Cmd {
	if s == nil {
		return nil
	}
	st := a.active()
	st.Push(s)
	w, h := a.contentSize()
	s.SetSize(w, h)
	return s.Init()
}

// Update implements tea.Model.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.status.Width = msg.Width
		w, h := a.contentSize()
		a.lstack.SetSize(w, h)
		a.dstack.SetSize(w, h)
		a.estack.SetSize(w, h)
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
		// Reset both stacks to fresh roots wired with clients for the new config,
		// dropping any drill-down that belonged to the previous profile/region.
		a.lroot = lambdaui.NewListScreen()
		a.droot = dynamoui.NewListScreen()
		a.eroot = ecsui.NewListScreen()
		a.lstack = core.NewStack(a.lroot)
		a.dstack = core.NewStack(a.droot)
		a.estack = core.NewStack(a.eroot)
		w, h := a.contentSize()
		a.lstack.SetSize(w, h)
		a.dstack.SetSize(w, h)
		a.estack.SetSize(w, h)
		a.lroot.SetClient(awsx.NewLambdaClient(msg.cfg))
		a.droot.SetClient(awsx.NewDynamoClient(msg.cfg))
		a.eroot.SetClient(awsx.NewEcsClient(msg.cfg))
		return a, tea.Batch(a.lroot.Refresh(), a.droot.Refresh(), a.eroot.Refresh())

	case profile.Selected:
		a.mode = a.prevMode
		return a, loadAWSConfigCmd(msg.Profile, msg.Region)

	case profile.Cancelled:
		a.mode = a.prevMode
		return a, nil

	case core.PushMsg:
		return a, a.push(msg.Screen)

	case core.PopMsg:
		a.active().Pop()
		return a, nil

	case tea.KeyMsg:
		// Picker owns input while open.
		if a.mode == ModeProfile {
			var cmd tea.Cmd
			a.picker, cmd = a.picker.Update(msg)
			return a, cmd
		}
		return a.handleKey(msg)
	}

	// Non-key async messages (spinner ticks, data-loaded) go to every screen in
	// both stacks so a background fetch completes regardless of what is on top.
	return a, tea.Batch(a.lstack.Broadcast(msg), a.dstack.Broadcast(msg), a.estack.Broadcast(msg))
}

// handleKey routes a key press: ctrl+c quits, screen-specific drill keys push a
// child, input-capturing screens swallow the rest, `esc` pops, and the global
// shortcuts switch mode / open the profile picker.
func (a App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	st := a.active()
	top := st.Top()

	// ctrl+c always quits, even inside an editor.
	if msg.String() == "ctrl+c" {
		a.quitting = true
		return a, tea.Quit
	}

	// Drill-in keys (screen-specific). Checked first so a screen can both edit
	// text and expose a drill key when it is not currently focused.
	if cmd, handled := a.drill(top, msg); handled {
		return a, cmd
	}

	// esc pops the stack, unless the screen wants esc to cancel an in-screen mode
	// (an active filter, say). At the root, Pop is a no-op.
	if msg.String() == "esc" {
		if e, ok := top.(core.EscHandler); ok && e.WantsEsc() {
			nt, cmd := top.Update(msg)
			st.SetTop(nt)
			return a, cmd
		}
		st.Pop()
		return a, nil
	}

	// Input-capturing screens (payload editor, active filter…) own every other
	// key so navigation shortcuts don't steal literal input.
	if c, ok := top.(core.InputCapturer); ok && c.CapturesInput() {
		nt, cmd := top.Update(msg)
		st.SetTop(nt)
		return a, cmd
	}

	// Global navigation.
	switch {
	case keyMatches(msg, a.keys.Quit):
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
	case keyMatches(msg, a.keys.Ecs):
		a.mode = ModeEcs
		a.tabs.Active = 2
		return a, nil
	case keyMatches(msg, a.keys.Profile):
		a.prevMode = a.mode
		a.mode = ModeProfile
		a.picker = profile.New(a.status.Profile, a.status.Region)
		var cmd tea.Cmd
		a.picker, cmd = a.picker.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
		return a, tea.Batch(a.picker.Init(), cmd)
	}

	// Anything else goes to the top screen.
	nt, cmd := top.Update(msg)
	st.SetTop(nt)
	return a, cmd
}

// drill maps a key to a screen-specific drill-in. It returns (cmd, true) when
// the key is a drill key for the current top screen — even if the resulting
// builder yields nil (no selection), so the key is consumed rather than falling
// through to global navigation.
func (a App) drill(top core.Screen, msg tea.KeyMsg) (tea.Cmd, bool) {
	switch t := top.(type) {
	case lambdaui.RootList:
		if t.IsFiltering() {
			return nil, false
		}
		switch msg.String() {
		case "enter":
			return a.push(t.OpenDetail(a.cfg)), true
		case "i":
			return a.push(t.OpenInvoke(a.cfg)), true
		case "l":
			return a.push(t.OpenLogs(a.cfg)), true
		}
	case lambdaui.Detailer:
		switch msg.String() {
		case "i":
			return a.push(t.OpenInvoke(a.cfg)), true
		case "l":
			return a.push(t.OpenLogs(a.cfg)), true
		}
	case dynamoui.RootList:
		if t.IsFiltering() {
			return nil, false
		}
		if msg.String() == "enter" {
			return a.push(t.OpenDescribe(a.cfg)), true
		}
	case dynamoui.Describer:
		switch msg.String() {
		case "s":
			return a.push(t.OpenScan(a.cfg)), true
		case "Q":
			return a.push(t.OpenQuery(a.cfg)), true
		}
	case dynamoui.Scanner:
		if msg.String() == "enter" {
			return a.push(t.OpenItem(a.cfg)), true
		}
	case dynamoui.Querier:
		if msg.String() == "o" && !t.InputFocused() {
			return a.push(t.OpenResult(a.cfg)), true
		}
	case ecsui.RootList:
		if t.IsFiltering() {
			return nil, false
		}
		if msg.String() == "enter" {
			return a.push(t.OpenServices(a.cfg)), true
		}
	case ecsui.ServiceList:
		if t.IsFiltering() {
			return nil, false
		}
		switch msg.String() {
		case "enter":
			return a.push(t.OpenTasks(a.cfg)), true
		case "d":
			return a.push(t.OpenDescribe(a.cfg)), true
		}
	case ecsui.TaskList:
		if t.IsFiltering() {
			return nil, false
		}
		if msg.String() == "enter" {
			return a.push(t.OpenContainers(a.cfg)), true
		}
	}
	return nil, false
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
	if crumbs := a.crumbView(); crumbs != "" {
		header += "  " + crumbs
	}

	body := a.active().Top().View()
	w, h := a.contentSize()
	mid := lipgloss.NewStyle().Width(w).Height(h).Render(body)
	return header + "\n" + mid + "\n" + a.statusView()
}

// crumbView renders the active stack's breadcrumb trail, or "" at the root.
func (a App) crumbView() string {
	c := a.active().Crumbs()
	if len(c) <= 1 {
		return ""
	}
	return a.theme.Subtle.Render(strings.Join(c, " › "))
}

func (a App) statusView() string {
	a.status.LastErr = a.lastErr
	return a.status.View()
}

func centered(w, h int, s string) string {
	return lipgloss.Place(w, h-2, lipgloss.Center, lipgloss.Center, s)
}
