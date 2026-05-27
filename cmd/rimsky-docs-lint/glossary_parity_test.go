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

// findModuleRoot walks up from the test's cwd looking for the cmd module's
// go.mod. Tests run with cwd = the package directory
// (cmd/rimsky-docs-lint/), so one level up is the module root. We walk to be
// defensive against future relayouts.
func findModuleRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Skipf("module root (go.mod) not found above %s", wd)
	return ""
}

// writeCatalog stages a fake RIMSKY_REPO whose concept catalog has the given
// content, sets RIMSKY_REPO for the test, and returns the catalog content.
func stageCatalog(t *testing.T, content string) {
	t.Helper()
	repo := t.TempDir()
	catalogDir := filepath.Join(repo, ".ok-planner", "design")
	if err := os.MkdirAll(catalogDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(catalogDir, "concepts.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RIMSKY_REPO", repo)
}

func TestGlossaryParity_DetectsDrift(t *testing.T) {
	moduleRoot := findModuleRoot(t)
	stageCatalog(t, "# Catalog\n\n- `claim` — a thing.\n")
	tmp := t.TempDir()
	outAbs := filepath.Join(tmp, "glossary.md")
	if err := os.WriteFile(outAbs, []byte("stale content"), 0644); err != nil {
		t.Fatal(err)
	}
	err := runGlossaryParity([]string{
		"-repo-root=" + moduleRoot,
		"-output=" + outAbs,
	})
	if err == nil {
		t.Fatal("expected drift error")
	}
	if !strings.Contains(err.Error(), "glossary parity failed") {
		t.Errorf("unexpected error shape: %v", err)
	}
}

func TestGlossaryParity_RoundTrip(t *testing.T) {
	moduleRoot := findModuleRoot(t)
	content := "# Catalog\n\n- `claim` — a thing.\n"
	stageCatalog(t, content)
	tmp := t.TempDir()
	outAbs := filepath.Join(tmp, "glossary.md")
	// The published glossary is a verbatim copy of the catalog.
	if err := os.WriteFile(outAbs, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	err := runGlossaryParity([]string{
		"-repo-root=" + moduleRoot,
		"-output=" + outAbs,
	})
	if err != nil {
		t.Errorf("expected parity pass, got %v", err)
	}
}
