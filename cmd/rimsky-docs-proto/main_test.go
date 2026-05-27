// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// requireProtoc skips the test when protoc is not on PATH, so the suite stays
// green in environments without it.
func requireProtoc(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("protoc not found on PATH; skipping")
	}
}

func TestRun_GeneratesExpectedSections(t *testing.T) {
	requireProtoc(t)
	out := t.TempDir()
	if err := run("testdata", out, false); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(out, "reference", "sample.md"))
	if err != nil {
		t.Fatal(err)
	}
	md := string(got)

	for _, want := range []string{
		autogenBanner,
		"# Sample",
		"## Services",
		"### Greeter",
		"#### Greeter.Hello",
		"#### Greeter.Watch",
		"stream HelloResponse",
		"## Messages",
		"### HelloRequest",
		"| `name` | `string` | 1 |",
		"repeated string",
		"optional string",
		"## Enums",
		"### Mood",
		"| `MOOD_HAPPY` | 1 |",
		"greets the caller",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("output missing %q\n---\n%s", want, md)
		}
	}
}

func TestRun_CheckMode_PassesThenDetectsDrift(t *testing.T) {
	requireProtoc(t)
	out := t.TempDir()
	if err := run("testdata", out, false); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := run("testdata", out, true); err != nil {
		t.Errorf("expected check pass after generate, got %v", err)
	}

	// Mutate the file on disk; check must now report drift.
	p := filepath.Join(out, "reference", "sample.md")
	if err := os.WriteFile(p, []byte("stale\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := run("testdata", out, true); err == nil {
		t.Fatal("expected drift error, got nil")
	}
}

func TestHyphenateAndTitleize(t *testing.T) {
	if got := hyphenate("a/claim_producer.proto"); got != "claim-producer" {
		t.Errorf("hyphenate = %q", got)
	}
	if got := titleize("claim_producer.proto"); got != "Claim Producer" {
		t.Errorf("titleize = %q", got)
	}
}

func TestTypeBasename(t *testing.T) {
	if got := typeBasename(".rimsky.v1.OpenRequest"); got != "OpenRequest" {
		t.Errorf("typeBasename = %q", got)
	}
}
