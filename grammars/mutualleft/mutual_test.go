// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go tool peg -strict -leftrecursion mutual.peg

package mutualleft

import (
	"strings"
	"testing"
)

// TestMutualLeftRecursion verifies a recursion spanning two named rules keeps
// the named derivation layers in the parse tree instead of flattening them.
func TestMutualLeftRecursion(t *testing.T) {
	p := &Mutual[uint32]{Buffer: "8-3-1"}
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}
	if err := p.Parse(); err != nil {
		t.Fatal(err)
	}

	var rules []string
	for _, line := range strings.Split(strings.TrimSpace(p.SprintSyntaxTree()), "\n") {
		line = strings.TrimSpace(line)
		rule, _, _ := strings.Cut(line, " ")
		rules = append(rules, rule)
	}

	// The Stmt/Expr layers alternate down to the base Num. Flattening the
	// repetition would remove the Stmt layers entirely.
	want := []string{
		"Start", "Root",
		"Expr", "Stmt", "Expr", "Stmt", "Expr",
		"Num", "PegText", "Num", "PegText", "Num", "PegText",
	}
	if len(rules) != len(want) {
		t.Fatalf("rule layers %v, want %v", rules, want)
	}
	for i := range want {
		if rules[i] != want[i] {
			t.Fatalf("rule layers %v, want %v", rules, want)
		}
	}
}

// TestSeedlessRecursionFails verifies recursive cycles without a successful
// base (including a nullable one) terminate with an ordinary parse error
// instead of recursing forever.
func TestSeedlessRecursionFails(t *testing.T) {
	// The grammar exposes the seedless rule through a failing first
	// alternative of Root; a valid input still parses through the fallback.
	p := &Mutual[uint32]{Buffer: "1-2"}
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}
	if err := p.Parse(); err != nil {
		t.Fatalf("seedless side rules broke the normal parse: %v", err)
	}

	// Inputs that only the seedless cycles could match must fail fast.
	for _, input := range []string{"@", "1-"} {
		q := &Mutual[uint32]{Buffer: input}
		if err := q.Init(); err != nil {
			t.Fatal(err)
		}
		if err := q.Parse(); err == nil {
			t.Errorf("Parse(%q) should have failed", input)
		}
	}
}
