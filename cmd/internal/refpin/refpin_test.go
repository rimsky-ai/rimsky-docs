// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

package refpin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeManifest(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plugin.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolve(t *testing.T) {
	path := writeManifest(t, `{"name":"rimsky","version":"1.4.0","reconciledAgainst":"v0.8.0"}`)
	got, err := Resolve(path)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "v0.8.0" {
		t.Errorf("Resolve = %q, want v0.8.0", got)
	}
}

func TestResolve_Errors(t *testing.T) {
	if _, err := Resolve(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Error("expected error for missing manifest")
	}
	if _, err := Resolve(writeManifest(t, `not json`)); err == nil {
		t.Error("expected error for malformed manifest")
	}
	if _, err := Resolve(writeManifest(t, `{"version":"1.4.0"}`)); err == nil {
		t.Error("expected error for empty reconciledAgainst")
	}
}

func TestBanner(t *testing.T) {
	b := Banner("v0.8.0")
	if !strings.Contains(b, "reflects rimsky v0.8.0") {
		t.Errorf("banner missing version: %q", b)
	}
	if !strings.HasPrefix(b, "<!--") || !strings.HasSuffix(b, "-->") {
		t.Errorf("banner is not a single HTML comment line: %q", b)
	}
}
