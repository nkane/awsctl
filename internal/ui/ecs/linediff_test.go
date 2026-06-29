package ecs

import (
	"strings"
	"testing"
)

func TestLineDiffBasic(t *testing.T) {
	a := []string{"one", "two", "three"}
	b := []string{"one", "TWO", "three", "four"}
	d := lineDiff(a, b)

	added, removed := diffStat(d)
	if added != 2 || removed != 1 {
		t.Fatalf("stat = +%d -%d, want +2 -1", added, removed)
	}

	// Equal lines preserved in order; "two" removed, "TWO" + "four" added.
	var eq, del, add []string
	for _, ln := range d {
		switch ln.op {
		case diffEqual:
			eq = append(eq, ln.text)
		case diffDel:
			del = append(del, ln.text)
		case diffAdd:
			add = append(add, ln.text)
		}
	}
	if strings.Join(eq, ",") != "one,three" {
		t.Fatalf("equal lines = %v, want [one three]", eq)
	}
	if strings.Join(del, ",") != "two" {
		t.Fatalf("deleted lines = %v, want [two]", del)
	}
	if strings.Join(add, ",") != "TWO,four" {
		t.Fatalf("added lines = %v, want [TWO four]", add)
	}
}

func TestLineDiffIdentical(t *testing.T) {
	a := []string{"x", "y", "z"}
	added, removed := diffStat(lineDiff(a, a))
	if added != 0 || removed != 0 {
		t.Fatalf("identical inputs produced +%d -%d, want 0 0", added, removed)
	}
}

func TestLineDiffEmptySides(t *testing.T) {
	added, removed := diffStat(lineDiff(nil, []string{"a", "b"}))
	if added != 2 || removed != 0 {
		t.Fatalf("nil→2 lines = +%d -%d, want +2 -0", added, removed)
	}
	added, removed = diffStat(lineDiff([]string{"a", "b"}, nil))
	if added != 0 || removed != 2 {
		t.Fatalf("2 lines→nil = +%d -%d, want +0 -2", added, removed)
	}
}

func TestRenderDiffGutters(t *testing.T) {
	out := renderDiff([]diffLine{
		{diffEqual, "ctx"},
		{diffDel, "gone"},
		{diffAdd, "new"},
	})
	for _, want := range []string{"  ctx", "- gone", "+ new"} {
		if !strings.Contains(out, want) {
			t.Fatalf("renderDiff missing %q; got:\n%s", want, out)
		}
	}
}
