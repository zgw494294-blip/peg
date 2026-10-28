// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildPegCLI builds the peg command line tool into dir and returns its path.
func buildPegCLI(t *testing.T, dir string) string {
	t.Helper()
	binary := filepath.Join(dir, "peg")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("go build unavailable: %v\n%s", err, out)
	}
	return binary
}

// generateConsumer builds the peg CLI, generates a parser with the given flags
// into a fresh standalone module and runs the independent public consumer.
func generateConsumer(t *testing.T, flags []string) string {
	t.Helper()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}

	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	module := t.TempDir()

	peg := buildPegCLI(t, module)

	grammar, err := os.ReadFile(filepath.Join(root, "testdata", "leftrec_cli", "cli.peg"))
	if err != nil {
		t.Fatal(err)
	}
	grammarPath := filepath.Join(module, "cli.peg")
	if err := os.WriteFile(grammarPath, grammar, 0o644); err != nil {
		t.Fatal(err)
	}

	args := append(append([]string{}, flags...), "-output",
		filepath.Join(module, "cli.peg.go"), grammarPath)
	cmd := exec.Command(peg, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("peg %v: %v\n%s", flags, err, out)
	}

	consumer, err := os.ReadFile(filepath.Join(root, "testdata", "leftrec_cli", "main.go.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "main.go"), consumer, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "go.mod"),
		[]byte("module clilr\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("go", "run", ".")
	cmd.Dir = module
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("consumer build/run failed (%v):\n%s", err, out)
	}
	return string(out)
}

// expectedConsumerOutput is the behavior every flag combination must share.
const expectedConsumerOutput = `events_after_parse=0
events=num:8,num:3,sub,num:1,sub
tree=0:Start:8-3-1,1:Root:8-3-1,2:Expr:8-3-1,3:Expr:8-3,4:Expr:8,5:Num:8,6:PegText:8,4:Num:3,5:PegText:3,3:Num:1,4:PegText:1
events_reset=num:2,num:1,sub
bad_parse=failed
good_events=num:9
`

// TestLeftRecursionCLIConsumer generates a parser from the command line with
// every supported optimization combination, compiles it inside an
// independent module that only depends on the generated file, and checks
// recognition, named tree nesting, the deferred action stream and reset.
func TestLeftRecursionCLIConsumer(t *testing.T) {
	combinations := [][]string{
		{"-leftrecursion"},
		{"-leftrecursion", "-inline"},
		{"-leftrecursion", "-switch"},
		{"-leftrecursion", "-inline", "-switch"},
	}
	for _, flags := range combinations {
		t.Run(strings.Join(flags, "+"), func(t *testing.T) {
			out := generateConsumer(t, flags)
			if out != expectedConsumerOutput {
				t.Errorf("consumer output mismatch for %v\ngot:\n%s\nwant:\n%s",
					flags, out, expectedConsumerOutput)
			}
		})
	}
}

// TestLeftRecursionCLIUnsupported verifies the documented diagnostics at the
// command line level: -strict keeps the left recursion diagnostic for the
// default mode, and -leftrecursion rejects -noast.
func TestLeftRecursionCLIUnsupported(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}
	module := t.TempDir()
	peg := buildPegCLI(t, module)

	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	grammar, err := os.ReadFile(filepath.Join(root, "testdata", "leftrec_cli", "cli.peg"))
	if err != nil {
		t.Fatal(err)
	}
	grammarPath := filepath.Join(module, "cli.peg")
	if err := os.WriteFile(grammarPath, grammar, 0o644); err != nil {
		t.Fatal(err)
	}

	// Default mode keeps flagging left recursion under -strict.
	cmd := exec.Command(peg, "-strict", "-output",
		filepath.Join(module, "out1.go"), grammarPath)
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Errorf("strict without -leftrecursion should fail:\n%s", out)
	} else if !strings.Contains(string(out), "left recursion") {
		t.Errorf("expected a left recursion diagnostic, got:\n%s", out)
	}

	// Left recursion mode requires the AST.
	cmd = exec.Command(peg, "-strict", "-leftrecursion", "-noast", "-output",
		filepath.Join(module, "out2.go"), grammarPath)
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Errorf("-leftrecursion -noast should fail:\n%s", out)
	} else if !strings.Contains(strings.ToLower(string(out)), "ast") {
		t.Errorf("expected an AST requirement diagnostic, got:\n%s", out)
	}
}
