package ui

// Stack is a LIFO of Screens — the navigation history for one mode.
//
// Each top-level mode (Lambda, Dynamo, ECS) owns its own Stack so that switching
// tabs preserves each mode's drill-down position. The bottom of the stack is the
// mode's root list screen; drilling in pushes, `esc` pops.
type Stack struct {
	screens []Screen
}

// NewStack returns a stack seeded with a root screen.
func NewStack(root Screen) *Stack {
	return &Stack{screens: []Screen{root}}
}

// Push adds a screen to the top.
func (s *Stack) Push(scr Screen) {
	s.screens = append(s.screens, scr)
}

// Pop removes and returns the top screen. It never pops the root: popping the
// last screen is a no-op and returns nil, so a mode always has a visible view.
func (s *Stack) Pop() Screen {
	if len(s.screens) <= 1 {
		return nil
	}
	top := s.screens[len(s.screens)-1]
	s.screens = s.screens[:len(s.screens)-1]
	return top
}

// Top returns the current screen (never nil — there is always a root).
func (s *Stack) Top() Screen {
	return s.screens[len(s.screens)-1]
}

// SetTop replaces the current screen in place (used when Update returns a new
// screen value, since screens are values, not pointers).
func (s *Stack) SetTop(scr Screen) {
	s.screens[len(s.screens)-1] = scr
}

// Depth is the number of screens in the stack.
func (s *Stack) Depth() int { return len(s.screens) }

// AtRoot reports whether only the root screen remains.
func (s *Stack) AtRoot() bool { return len(s.screens) == 1 }

// Crumbs returns the breadcrumb trail, root first.
func (s *Stack) Crumbs() []string {
	out := make([]string, len(s.screens))
	for i, scr := range s.screens {
		out[i] = scr.Title()
	}
	return out
}

// SetSize forwards a resize to every screen so background (non-top) screens stay
// laid out correctly when revealed by a pop.
func (s *Stack) SetSize(w, h int) {
	for _, scr := range s.screens {
		scr.SetSize(w, h)
	}
}
