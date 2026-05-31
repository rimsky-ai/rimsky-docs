// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sampleCapture is a tiny stand-in for the CLI's real help output, used to
// exercise the pure assembly layer without the slow `go run` shell-out.
func sampleCapture() capture {
	return capture{
		topLevel: "rimsky — orchestration CLI for the rimsky platform.\n\nDev-loop:\n  run <file>\n\nLiteral API:\n  template register | lint",
		groups: []groupHelp{
			{name: "template", help: "usage: rimsky template <register|lint|list|get|deploy|undeploy|rm> ..."},
			{name: "instance", help: "usage: rimsky instance <create|list|get|status|delete|kill|nodes|events> ..."},
			{name: "blank", help: "   \n  "}, // empty-capture path
		},
	}
}

func TestAssemble_RendersTreeAndGroups(t *testing.T) {
	md := assemble(sampleCapture())

	for _, want := range []string{
		autogenBanner,
		"# rimsky CLI reference",
		"RIMSKY_CONTROL_API",
		"REST control API",
		"## Command tree",
		"orchestration CLI for the rimsky platform.",
		"## Command groups",
		"### `rimsky template`",
		"usage: rimsky template <register|lint|list|get|deploy|undeploy|rm> ...",
		"### `rimsky instance`",
		"### `rimsky blank`",
		"_No usage text captured for this group._",
		"```text",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("assembled output missing %q\n---\n%s", want, md)
		}
	}
	if !strings.HasSuffix(md, "\n") {
		t.Error("output should end with a trailing newline")
	}
}

func TestWriteOrCheck_WriteThenCheckThenDrift(t *testing.T) {
	out := filepath.Join(t.TempDir(), "reference", "cli.md")
	md := assemble(sampleCapture())

	if err := writeOrCheck(md, out, false); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := writeOrCheck(md, out, true); err != nil {
		t.Errorf("expected check pass after write, got %v", err)
	}

	if err := os.WriteFile(out, []byte("stale\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeOrCheck(md, out, true); err == nil {
		t.Fatal("expected drift error, got nil")
	}
}

func TestStripGoRunExit(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"trailing exit status", "usage: rimsky template ...\nexit status 2\n", "usage: rimsky template ..."},
		{"trailing blank lines", "usage line\n\n\n", "usage line"},
		{"no exit line", "clean output\n", "clean output"},
		{"exit then blanks", "u\nexit status 1\n\n", "u"},
		{"only exit status", "exit status 2\n", ""},
	}
	for _, c := range cases {
		if got := stripGoRunExit(c.in); got != c.want {
			t.Errorf("%s: stripGoRunExit(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

// TestRun_AgainstRealCLI exercises the full capture+assemble+check pipeline
// against a real rimsky checkout. It skips gracefully when RIMSKY_REPO is
// unset (matching the other generators) so the suite stays green without a
// local checkout. `go run` is slow, so this runs only when explicitly opted
// into via RIMSKY_REPO.
func TestRun_AgainstRealCLI(t *testing.T) {
	rimskyRepo := os.Getenv("RIMSKY_REPO")
	if rimskyRepo == "" {
		t.Skip("RIMSKY_REPO unset; skipping live CLI capture")
	}

	out := filepath.Join(t.TempDir(), "cli.md")
	if err := run(rimskyRepo, out, false); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	md := string(got)

	for _, want := range []string{
		autogenBanner,
		"# rimsky CLI reference",
		"## Command tree",
		"### `rimsky template`",
		"### `rimsky instance`",
		"### `rimsky conformance`",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("real-CLI output missing %q", want)
		}
	}

	// -check must be stable immediately after generation.
	if err := run(rimskyRepo, out, true); err != nil {
		t.Errorf("expected check pass after generate, got %v", err)
	}
}
