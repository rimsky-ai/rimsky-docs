// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRun_WriteMode_CopiesCatalogVerbatim(t *testing.T) {
	tmp := t.TempDir()
	catalog := filepath.Join(tmp, "concepts.md")
	want := []byte("# Concept catalog\n\n## Concepts\n\n- `claim` — a thing.\n")
	if err := os.WriteFile(catalog, want, 0644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "glossary.md")
	if err := run(catalog, out, false); err != nil {
		t.Fatalf("run write: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output is not a verbatim copy of the catalog\n got:  %q\n want: %q", got, want)
	}
}

func TestRun_CheckMode_PassesWhenIdentical(t *testing.T) {
	tmp := t.TempDir()
	catalog := filepath.Join(tmp, "concepts.md")
	content := []byte("# Concept catalog\n\n- `claim` — a thing.\n")
	if err := os.WriteFile(catalog, content, 0644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "glossary.md")
	if err := os.WriteFile(out, content, 0644); err != nil {
		t.Fatal(err)
	}
	if err := run(catalog, out, true); err != nil {
		t.Errorf("expected check pass, got %v", err)
	}
}

func TestRun_CheckMode_DetectsDrift(t *testing.T) {
	tmp := t.TempDir()
	catalog := filepath.Join(tmp, "concepts.md")
	if err := os.WriteFile(catalog, []byte("source content\n"), 0644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "glossary.md")
	if err := os.WriteFile(out, []byte("stale content\n"), 0644); err != nil {
		t.Fatal(err)
	}
	err := run(catalog, out, true)
	if err == nil {
		t.Fatal("expected drift error, got nil")
	}
	if !contains(err.Error(), "differs from source catalog") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_MissingCatalog_Errors(t *testing.T) {
	tmp := t.TempDir()
	err := run(filepath.Join(tmp, "nope.md"), filepath.Join(tmp, "glossary.md"), false)
	if err == nil {
		t.Fatal("expected error for missing catalog")
	}
}

func contains(haystack, needle string) bool {
	return bytes.Contains([]byte(haystack), []byte(needle))
}
