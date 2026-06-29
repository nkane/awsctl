package ecs

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// diffOp classifies a line in a two-way diff.
type diffOp int

const (
	diffEqual diffOp = iota
	diffDel
	diffAdd
)

// diffLine is one rendered line of a diff with its operation.
type diffLine struct {
	op   diffOp
	text string
}

// lineDiff computes a line-level diff of a→b via a longest-common-subsequence
// walk. Returns the ordered ops (equal/del/add). Deterministic and
// dependency-free, suitable for diffing two pretty-printed task-def JSONs.
func lineDiff(a, b []string) []diffLine {
	n, m := len(a), len(b)

	// lcs[i][j] = length of the LCS of a[i:] and b[j:].
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	out := make([]diffLine, 0, n+m)
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			out = append(out, diffLine{diffEqual, a[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			out = append(out, diffLine{diffDel, a[i]})
			i++
		default:
			out = append(out, diffLine{diffAdd, b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, diffLine{diffDel, a[i]})
	}
	for ; j < m; j++ {
		out = append(out, diffLine{diffAdd, b[j]})
	}
	return out
}

var (
	diffAddStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	diffDelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

// renderDiff formats diff lines with +/- gutters and add/del coloring.
func renderDiff(lines []diffLine) string {
	var b strings.Builder
	for _, ln := range lines {
		switch ln.op {
		case diffAdd:
			b.WriteString(diffAddStyle.Render("+ "+ln.text) + "\n")
		case diffDel:
			b.WriteString(diffDelStyle.Render("- "+ln.text) + "\n")
		default:
			b.WriteString("  " + ln.text + "\n")
		}
	}
	return b.String()
}

// diffStat counts added and removed lines, for a one-line summary.
func diffStat(lines []diffLine) (added, removed int) {
	for _, ln := range lines {
		switch ln.op {
		case diffAdd:
			added++
		case diffDel:
			removed++
		}
	}
	return added, removed
}
