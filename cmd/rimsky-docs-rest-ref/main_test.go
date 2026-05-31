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

// buildActionsFixture materializes a miniature actions.go under a temp dir from
// the testdata fixture (stored as .go.txt so it is not compiled with the cmd
// module) and returns its path.
func buildActionsFixture(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", "actions.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "actions.go")
	if err := os.WriteFile(path, src, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRun_GeneratesExpectedTable(t *testing.T) {
	actionsFile := buildActionsFixture(t)
	out := filepath.Join(t.TempDir(), "rest-api.md")

	if err := run(actionsFile, out, false); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	md := string(got)

	for _, want := range []string{
		autogenBanner,
		"# REST / HTTP control API reference",
		"Bare, unversioned paths",
		"auth-gated per-action",
		// Counts: 6 routes (2 + 1 + 2 + 1) across 4 actions.
		"6 routes across 4 actions.",
		// Grouped headers by resource prefix.
		"## /instances",
		"## /nodes",
		"## /admin",
		"## /v1",
		// Route rows: method, path, gate, R/W, verbatim description.
		"| `GET` | `/instances` | `instance:read` | R |",
		"| `POST` | `/instances` | `instance:create` | W |",
		"| `POST` | `/nodes/{id}/invalidate` | `node:invalidate` | W |",
		// A second route on the same action lands in a different group.
		"| `POST` | `/admin/instances/{instance}/nodes/{node_id}/invalidate` | `node:invalidate` | W |",
		"| `GET` | `/v1/observability/*` | `observability:read` | R |",
		"Read instances; list all or get one by id-or-key.",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("output missing %q\n---\n%s", want, md)
		}
	}
	if strings.Contains(md, "MCPTools") || strings.Contains(md, "instance_list") {
		t.Errorf("MCP tool surface leaked into the HTTP reference:\n%s", md)
	}
}

func TestRun_CheckMode_PassesThenDetectsDrift(t *testing.T) {
	actionsFile := buildActionsFixture(t)
	out := filepath.Join(t.TempDir(), "rest-api.md")

	if err := run(actionsFile, out, false); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := run(actionsFile, out, true); err != nil {
		t.Errorf("expected check pass after generate, got %v", err)
	}

	if err := os.WriteFile(out, []byte("stale\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := run(actionsFile, out, true); err == nil {
		t.Fatal("expected drift error, got nil")
	}
}

// TestRun_AgainstRimskyRepo generates against the real registry when
// RIMSKY_REPO is set, verifying the output is well-formed and -check is stable.
// Skips gracefully when the env var is unset (CI without a rimsky checkout).
func TestRun_AgainstRimskyRepo(t *testing.T) {
	repo := os.Getenv("RIMSKY_REPO")
	if repo == "" {
		t.Skip("RIMSKY_REPO unset; skipping generation against real registry")
	}
	actionsFile := filepath.Join(repo, "lib", "control", "controlapi", "actions.go")
	if _, err := os.Stat(actionsFile); err != nil {
		t.Skipf("registry not found at %s: %v", actionsFile, err)
	}

	out := filepath.Join(t.TempDir(), "rest-api.md")
	if err := run(actionsFile, out, false); err != nil {
		t.Fatalf("run against %s: %v", actionsFile, err)
	}
	md, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		autogenBanner,
		"# REST / HTTP control API reference",
		"## /instances",
		"## /admin",
		"| `GET` | `/instances` | `instance:read` |",
	} {
		if !strings.Contains(string(md), want) {
			t.Errorf("real-registry output missing %q", want)
		}
	}
	// -check against the just-generated file must pass (deterministic output).
	if err := run(actionsFile, out, true); err != nil {
		t.Errorf("check after generate against real registry: %v", err)
	}
}

func TestPrefixOf(t *testing.T) {
	cases := map[string]string{
		"/instances":                       "/instances",
		"/instances/{idOrKey}/pause":       "/instances",
		"/admin/diagnostics/parked-nodes":  "/admin",
		"/v1/observability/*":              "/v1",
		"/lock-holders/{id}/claim-holders": "/lock-holders",
	}
	for path, want := range cases {
		if got := prefixOf(path); got != want {
			t.Errorf("prefixOf(%q) = %q, want %q", path, got, want)
		}
	}
}
