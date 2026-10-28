// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pointlander/peg/tree"
)

// compileWith compiles a grammar source with the given tree options and
// returns the generated source.
func compileWith(t *testing.T, source string,
	inline, _switch, noast, leftRecursion, strict bool) string {
	t.Helper()
	p := &Peg[uint32]{Tree: tree.New(inline, _switch, noast, leftRecursion), Buffer: source}
	if err := p.Init(Size[uint32](1 << 15)); err != nil {
		t.Fatal(err)
	}
	if err := p.Parse(); err != nil {
		t.Fatal(err)
	}
	p.Execute()
	p.Strict = strict
	out := &bytes.Buffer{}
	if err := p.Compile("gen.peg.go", []string{"peg"}, out); err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return out.String()
}

// TestLeftRecursionNoASTUnsupported checks the explicit diagnostic when
// combining -leftrecursion with -noast.
func TestLeftRecursionNoASTUnsupported(t *testing.T) {
	source := `package main
type test Peg {}
Expr <- Expr '-' Term / Term
Term <- [0-9]+
`
	p := &Peg[uint32]{Tree: tree.New(false, false, true, true), Buffer: source}
	if err := p.Init(Size[uint32](1 << 15)); err != nil {
		t.Fatal(err)
	}
	if err := p.Parse(); err != nil {
		t.Fatal(err)
	}
	p.Execute()
	out := &bytes.Buffer{}
	err := p.Compile("gen.peg.go", []string{"peg", "-leftrecursion", "-noast"}, out)
	if err == nil {
		t.Fatal("expected an error for -leftrecursion -noast")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "ast") {
		t.Fatalf("expected an AST requirement error, got: %v", err)
	}
}

// TestLeftRecursionStrictCompiles verifies left recursive grammars produce no
// "possible infinite left recursion" warning under -strict when the new mode
// is enabled.
func TestLeftRecursionStrictCompiles(t *testing.T) {
	source := `package main
type test Peg {}
Expr <- Expr '-' Term / Term
Term <- < [0-9]+ >
`
	_ = compileWith(t, source, false, false, false, true, true)
}

// TestLeftRecursionDefaultModeStillWarns verifies the original strict
// diagnostic is untouched when the flag is off.
func TestLeftRecursionDefaultModeStillWarns(t *testing.T) {
	source := `package main
type test Peg {}
Expr <- Expr '-' Term
Term <- [0-9]+
`
	p := &Peg[uint32]{Tree: tree.New(false, false, false, false), Buffer: source}
	if err := p.Init(Size[uint32](1 << 15)); err != nil {
		t.Fatal(err)
	}
	if err := p.Parse(); err != nil {
		t.Fatal(err)
	}
	p.Execute()
	p.Strict = true
	out := &bytes.Buffer{}
	err := p.Compile("gen.peg.go", []string{"peg", "-strict"}, out)
	if err == nil {
		t.Fatal("expected a left recursion warning in default mode")
	}
	if !strings.Contains(err.Error(), "left recursion") {
		t.Fatalf("expected a left recursion warning, got: %v", err)
	}
}

// TestLeftRecursionFlagNoLRGrammar verifies enabling the flag for a grammar
// without left recursion produces exactly the default-mode output, so plain
// grammars keep their original behavior.
func TestLeftRecursionFlagNoLRGrammar(t *testing.T) {
	source := `package main
type test Peg {}
Start <- ('a' / 'b')+ !.
`
	without := compileWith(t, source, true, true, false, false, false)
	with := compileWith(t, source, true, true, false, true, false)
	if without != with {
		t.Fatal("generated output differs when -leftrecursion is enabled " +
			"but the grammar has no left recursive rules")
	}
}

// TestLeftRecursionAnalysis verifies which rules get the seed/grow treatment:
// direct and mutual cycles are detected, ordinary recursion is not.
func TestLeftRecursionAnalysis(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   map[string]bool
	}{
		{
			name: "direct",
			source: `package main
type test Peg {}
Expr <- Expr '-' Term / Term
Term <- [0-9]+
`,
			want: map[string]bool{"Expr": true},
		},
		{
			name: "mutual",
			source: `package main
type test Peg {}
A <- B / 'x'
B <- A 'y'
`,
			want: map[string]bool{"A": true, "B": true},
		},
		{
			name: "guarded",
			source: `package main
type test Peg {}
A <- 'x' A / 'x'*
`,
			want: map[string]bool{},
		},
		{
			name: "nullable prefix",
			source: `package main
type test Peg {}
A <- 'x'? A / 'y'
`,
			want: map[string]bool{"A": true},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := &Peg[uint32]{Tree: tree.New(false, false, false, true), Buffer: c.source}
			if err := p.Init(Size[uint32](1 << 15)); err != nil {
				t.Fatal(err)
			}
			if err := p.Parse(); err != nil {
				t.Fatal(err)
			}
			p.Execute()
			if err := p.Compile("gen.peg.go", []string{"peg"}, &bytes.Buffer{}); err != nil {
				t.Fatal(err)
			}
			got := p.LeftRecursiveRules()
			for rule := range c.want {
				if !got[rule] {
					t.Errorf("rule %s should be marked left recursive; got %v", rule, got)
				}
			}
			for rule := range got {
				if !c.want[rule] {
					t.Errorf("rule %s should not be marked left recursive; got %v", rule, got)
				}
			}
		})
	}
}
