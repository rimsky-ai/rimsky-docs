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

// buildProtocolsFixture materializes a tiny protocols-module tree under a temp
// dir from the testdata fixture (stored as .go.txt so it is not compiled with
// the cmd module) and returns the module root. It also plants a package under
// proto/ to assert that subtree is excluded from the reference.
func buildProtocolsFixture(t *testing.T) string {
	t.Helper()
	protocolsDir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("testdata", "sample", "widget.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(protocolsDir, "sample")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "widget.go"), src, 0644); err != nil {
		t.Fatal(err)
	}
	// A generated-bindings stand-in under proto/ — must be skipped.
	genDir := filepath.Join(protocolsDir, "proto", "v1", "gen")
	if err := os.MkdirAll(genDir, 0755); err != nil {
		t.Fatal(err)
	}
	gen := "package gen\n\n// GeneratedWireType is a stand-in for protobuf bindings.\ntype GeneratedWireType struct{}\n"
	if err := os.WriteFile(filepath.Join(genDir, "wire.go"), []byte(gen), 0644); err != nil {
		t.Fatal(err)
	}
	return protocolsDir
}

func TestRun_GeneratesExpectedSections(t *testing.T) {
	protocolsDir := buildProtocolsFixture(t)
	out := filepath.Join(t.TempDir(), "go-packages.md")

	if err := run(protocolsDir, out, false); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	md := string(got)

	for _, want := range []string{
		autogenBanner,
		"# rimsky protocols module — Go package reference",
		"## sample",
		"### type Widget",
		"#### `func NewWidget(name string) *Widget`",
		"#### `func (w *Widget) Describe() string`",
		"### Functions",
		"#### `func Greet(name string) string`",
		"### Constants",
		"MaxWidgets",
		"constructs a Widget",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("output missing %q\n---\n%s", want, md)
		}
	}
	if strings.Contains(md, "unexported") {
		t.Errorf("unexported symbol leaked into reference")
	}
	// The proto/ subtree is the wire reference's job — it must not appear here.
	if strings.Contains(md, "GeneratedWireType") || strings.Contains(md, "proto/v1/gen") {
		t.Errorf("proto/ subtree leaked into Go package reference:\n%s", md)
	}
}

func TestRun_CheckMode_PassesThenDetectsDrift(t *testing.T) {
	protocolsDir := buildProtocolsFixture(t)
	out := filepath.Join(t.TempDir(), "go-packages.md")

	if err := run(protocolsDir, out, false); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := run(protocolsDir, out, true); err != nil {
		t.Errorf("expected check pass after generate, got %v", err)
	}

	if err := os.WriteFile(out, []byte("stale\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := run(protocolsDir, out, true); err == nil {
		t.Fatal("expected drift error, got nil")
	}
}

func TestRelImportPath(t *testing.T) {
	if got := relImportPath("/protocols", "/protocols/conformance/executor"); got != "conformance/executor" {
		t.Errorf("relImportPath = %q", got)
	}
	if got := relImportPath("/protocols", "/protocols"); got != "." {
		t.Errorf("relImportPath root = %q", got)
	}
}
