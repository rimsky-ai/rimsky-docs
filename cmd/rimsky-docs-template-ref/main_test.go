// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rimsky-ai/rimsky-docs/cmd/internal/refpin"
)

// buildSpecFixture materializes a tiny spec package under a temp dir from the
// testdata fixture (stored as .go.txt so it is not compiled with the cmd
// module) and returns the package directory.
func buildSpecFixture(t *testing.T) string {
	t.Helper()
	specDir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("testdata", "spec", "widgetspec.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "widgetspec.go"), src, 0644); err != nil {
		t.Fatal(err)
	}
	return specDir
}

// testVersion is the reconciled-version pin threaded into run() by tests; the
// real binary resolves it from plugin.json reconciledAgainst.
const testVersion = "v9.9.9-test"

func TestRun_GeneratesExpectedSections(t *testing.T) {
	specDir := buildSpecFixture(t)
	out := filepath.Join(t.TempDir(), "template-schema.md")

	if err := run(specDir, out, testVersion, false); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	md := string(got)

	for _, want := range []string{
		autogenBanner,
		refpin.Banner(testVersion),
		"# rimsky template schema (`rimsky.yml`) reference",
		"## Structs",
		"### WidgetSpec",
		"### WidgetPart",
		// YAML keys resolve from tags.
		"| `name` |",
		// omitempty surfaces as optional.
		"`description`<br/>_(optional)_",
		// Slice-of-struct type links to the element subsection.
		"[`[]WidgetPart`](#widgetpart)",
		// map type renders readably.
		"`map[string]string`",
		// json-only tag is used when yaml tag is absent.
		"| `config`",
		// Typed enum.
		"## Enums",
		"### WidgetMode",
		"Named string type (`type WidgetMode string`)",
		"| `WidgetModeFast` | `fast` |",
		// Untyped const group, titled by shared prefix.
		"### WidgetState*",
		"Untyped string constant group",
		"| `WidgetStateOn` | `on` |",
		// Enum-typed field links to the enum subsection.
		"[`WidgetMode`](#widgetmode)",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("output missing %q\n---\n%s", want, md)
		}
	}
	// Numeric const must not leak into the enum tables.
	if strings.Contains(md, "MaxWidgets") {
		t.Errorf("numeric const leaked into enum reference:\n%s", md)
	}
	// Unexported nothing — fixture has no unexported symbols to leak, but the
	// renderer must not emit Go method/func sections (this is a schema ref).
	if strings.Contains(md, "func ") {
		t.Errorf("Go func surface leaked into schema reference:\n%s", md)
	}
}

func TestRun_CheckMode_PassesThenDetectsDrift(t *testing.T) {
	specDir := buildSpecFixture(t)
	out := filepath.Join(t.TempDir(), "template-schema.md")

	if err := run(specDir, out, testVersion, false); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := run(specDir, out, testVersion, true); err != nil {
		t.Errorf("expected check pass after generate, got %v", err)
	}

	if err := os.WriteFile(out, []byte("stale\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := run(specDir, out, testVersion, true); err == nil {
		t.Fatal("expected drift error, got nil")
	}
}

func TestGithubAnchor(t *testing.T) {
	if got := githubAnchor("TemplateNodeDef"); got != "templatenodedef" {
		t.Errorf("githubAnchor = %q", got)
	}
}

func TestGroupTitle(t *testing.T) {
	vals := []enumValue{{Name: "NodeStateFresh"}, {Name: "NodeStateStale"}, {Name: "NodeStateRunning"}}
	if got := groupTitle(vals); got != "NodeState*" {
		t.Errorf("groupTitle = %q, want NodeState*", got)
	}
}

// TestRun_AgainstRimskyRepo regenerates against a real rimsky checkout when
// RIMSKY_REPO is set, then asserts -check passes and the output names the
// load-bearing schema structs. Skips when RIMSKY_REPO is unset (matching the
// other docs generators' behavior).
func TestRun_AgainstRimskyRepo(t *testing.T) {
	repo := os.Getenv("RIMSKY_REPO")
	if repo == "" {
		t.Skip("RIMSKY_REPO unset; skipping real-source generation check")
	}
	specDir := filepath.Join(repo, "lib", "foundation", "spec")
	if _, err := os.Stat(specDir); err != nil {
		t.Skipf("spec dir not found at %s; skipping", specDir)
	}
	out := filepath.Join(t.TempDir(), "template-schema.md")

	if err := run(specDir, out, testVersion, false); err != nil {
		t.Fatalf("generate against RIMSKY_REPO: %v", err)
	}
	if err := run(specDir, out, testVersion, true); err != nil {
		t.Errorf("expected check pass after generate, got %v", err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	md := string(got)
	for _, want := range []string{
		"### TemplateSpec",
		"### TemplateNodeDef",
		"### NodeStoreRef",
		"### SubscriptionEntry",
		"### FanOutSpec",
		"## Enums",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("real-source output missing %q", want)
		}
	}
}
