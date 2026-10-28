// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go tool peg -strict -switch -inline -leftrecursion leftrec.peg

package leftrecursion

import (
	"strings"
	"testing"
)

func parseLR(t *testing.T, input string) *LR[uint32] {
	t.Helper()
	p := &LR[uint32]{Buffer: input}
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse(%q): %v", input, err)
	}
	return p
}

// tokensOf extracts the ordered (rule, text) pairs printed by
// SprintSyntaxTree, ignoring the indentation that encodes depth.
func tokensOf(tree string) [][2]string {
	var tokens [][2]string
	for _, line := range strings.Split(strings.TrimSpace(tree), "\n") {
		line = strings.TrimSpace(line)
		rule, quoted, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		text := strings.Trim(quoted, `"`)
		tokens = append(tokens, [2]string{rule, text})
	}
	return tokens
}

// containsSubsequence reports whether tokens contains want in order.
func containsSubsequence(tokens [][2]string, want ...[2]string) bool {
	for i := 0; i+len(want) <= len(tokens); i++ {
		match := true
		for j := range want {
			if tokens[i+j] != want[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// TestLeftRecursionTree verifies that winning derivations keep the original
// named rule nesting: "8-3-1" is Expr(8-3-1) > Expr(8-3) > Expr(8), not a
// flattened repetition.
func TestLeftRecursionTree(t *testing.T) {
	p := parseLR(t, "8-3-1")
	tokens := tokensOf(p.SprintSyntaxTree())
	want := [][2]string{
		{"Start", "8-3-1"},
		{"Expr", "8-3-1"},
		{"Expr", "8-3"},
		{"Expr", "8"},
		{"Term", "8"},
		{"Factor", "8"},
		{"Post", "8"},
		{"Atom", "8"},
		{"Number", "8"},
		{"PegText", "8"},
		{"Term", "3"},
		{"Factor", "3"},
		{"Post", "3"},
		{"Atom", "3"},
		{"Number", "3"},
		{"PegText", "3"},
		{"Term", "1"},
		{"Factor", "1"},
		{"Post", "1"},
		{"Atom", "1"},
		{"Number", "1"},
		{"PegText", "1"},
	}
	if len(tokens) != len(want) {
		t.Fatalf("unexpected syntax tree tokens:\n%v\nwant:\n%v", tokens, want)
	}
	for i := range want {
		if tokens[i] != want[i] {
			t.Fatalf("token %d = %v, want %v\nfull tree:\n%v", i, tokens[i], want[i], tokens)
		}
	}
}

// TestLeftRecursionPostfix verifies continuous call/member suffixes keep the
// original rule nesting as well, in growth order.
func TestLeftRecursionPostfix(t *testing.T) {
	cases := []struct {
		input string
		chain [][2]string
	}{
		{"a.b.c", [][2]string{
			{"Post", "a.b.c"},
			{"Post", "a.b"},
			{"Post", "a"},
		}},
		{"f().g()", [][2]string{
			{"Post", "f().g()"},
			{"Post", "f().g"},
			{"Post", "f()"},
			{"Post", "f"},
		}},
	}
	for _, c := range cases {
		p := parseLR(t, c.input)
		tokens := tokensOf(p.SprintSyntaxTree())
		if !containsSubsequence(tokens, c.chain...) {
			t.Errorf("Parse(%q): missing left nested chain %v in tree:\n%v",
				c.input, c.chain, tokens)
		}
	}
}

// TestLeftRecursionPrecedence verifies ordinary precedence and parentheses
// survive left recursion mode unchanged.
func TestLeftRecursionPrecedence(t *testing.T) {
	cases := []struct {
		input   string
		actions []string
	}{
		{"1+2*3", []string{"num:1", "num:2", "num:3", "mul", "add"}},
		{"(1+2)*3", []string{"num:1", "num:2", "add", "num:3", "mul"}},
		{"8-3-1", []string{"num:8", "num:3", "sub", "num:1", "sub"}},
		{"8/4/2", []string{"num:8", "num:4", "div", "num:2", "div"}},
		{"a.b", []string{"member:b"}},
		{"f()", []string{"call"}},
		{"x.y().z", []string{"member:y", "call", "member:z"}},
		{" 2 * 3 ", []string{"num:2", "num:3", "mul"}},
	}
	for _, c := range cases {
		p := parseLR(t, c.input)
		// Parse must not run any Go action.
		if len(p.Events) != 0 {
			t.Errorf("Parse(%q) ran actions before Execute: %v", c.input, p.Events)
		}
		p.Execute()
		if len(p.Events) != len(c.actions) {
			t.Errorf("Parse(%q): actions %v, want %v", c.input, p.Events, c.actions)
			continue
		}
		for i := range c.actions {
			if p.Events[i] != c.actions[i] {
				t.Errorf("Parse(%q): actions %v, want %v", c.input, p.Events, c.actions)
				break
			}
		}
	}
}

// TestLeftRecursionRunes verifies captures keep the original text and
// nesting works with multibyte rune positions.
func TestLeftRecursionRunes(t *testing.T) {
	p := parseLR(t, "猫-狗-鱼")
	// Multibyte inputs must be captured verbatim in left nested Expr layers.
	tokens := tokensOf(p.SprintSyntaxTree())
	expected := [][2]string{
		{"Expr", "猫-狗-鱼"},
		{"Expr", "猫-狗"},
		{"Expr", "猫"},
		{"PegText", "猫"},
		{"PegText", "狗"},
		{"PegText", "鱼"},
	}
	for _, e := range expected {
		if !containsSubsequence(tokens, e) {
			t.Errorf("missing token %v in tree:\n%v", e, tokens)
		}
	}
}

// TestLeftRecursionFailures verifies inputs that cannot match return an error
// without hanging (no infinite recursion) and without leaving actions.
func TestLeftRecursionFailures(t *testing.T) {
	for _, input := range []string{"", "1+", "8--3", "@", "1 2", "(1", "f.(x)"} {
		p := &LR[uint32]{Buffer: input}
		if err := p.Init(); err != nil {
			t.Fatal(err)
		}
		if err := p.Parse(); err == nil {
			t.Errorf("Parse(%q) unexpectedly succeeded", input)
		}
		if len(p.Events) != 0 {
			t.Errorf("failed Parse(%q) left actions: %v", input, p.Events)
		}
	}
}

// TestLeftRecursionReset verifies that recursive results, captures and the
// action stream do not leak across resets, including failure followed by
// success on the same parser instance.
func TestLeftRecursionReset(t *testing.T) {
	p := &LR[uint32]{Buffer: "1+"}
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}
	if err := p.Parse(); err == nil {
		t.Fatal("expected failure for 1+")
	}

	// Failure followed by success on the same instance must not be polluted.
	p.Reset()
	if err := p.Parse(); err == nil {
		t.Fatal("expected 1+ to keep failing")
	}
	p.Buffer = "1+2"
	p.Reset()
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse after reset: %v", err)
	}
	p.Execute()
	want := []string{"num:1", "num:2", "add"}
	if len(p.Events) != len(want) {
		t.Fatalf("actions after failed parse: %v, want %v", p.Events, want)
	}
	for i := range want {
		if p.Events[i] != want[i] {
			t.Fatalf("actions %v, want %v", p.Events, want)
		}
	}

	// A new buffer must parse independently.
	p.Buffer = "a.b.c"
	p.Events = nil
	p.Reset()
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse a.b.c: %v", err)
	}
	p.Execute()
	if len(p.Events) != 2 || p.Events[0] != "member:b" || p.Events[1] != "member:c" {
		t.Fatalf("a.b.c actions: %v", p.Events)
	}

	// A freshly initialized parser is independent as well.
	q := &LR[uint32]{Buffer: "8-3-1"}
	if err := q.Init(); err != nil {
		t.Fatal(err)
	}
	if err := q.Parse(); err != nil {
		t.Fatal(err)
	}
	q.Execute()
	if len(q.Events) != 5 {
		t.Fatalf("fresh parser actions: %v", q.Events)
	}
}

// TestLeftRecursionDisableMemoize verifies the documented unsupported
// combination reports an error instead of misbehaving.
func TestLeftRecursionDisableMemoize(t *testing.T) {
	p := &LR[uint32]{Buffer: "1+2"}
	if err := p.Init(DisableMemoize[uint32]()); err != nil {
		t.Fatal(err)
	}
	if err := p.Parse(); err == nil {
		t.Fatal("Parse with DisableMemoize should fail in left recursion mode")
	}
}
