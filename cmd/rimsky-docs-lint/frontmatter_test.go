// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

package main

import (
	"strings"
	"testing"
)

func TestFrontmatter_GoodFixturePasses(t *testing.T) {
	if err := runFrontmatter([]string{"-dir=testdata/frontmatter-good", "-errors-dir="}); err != nil {
		t.Errorf("expected pass, got %v", err)
	}
}

func TestFrontmatter_UnknownKeyFails(t *testing.T) {
	err := runFrontmatter([]string{"-dir=testdata/frontmatter-bad-unknown-key", "-errors-dir="})
	if err == nil {
		t.Fatal("expected failure")
	}
	if !strings.Contains(err.Error(), "definition") {
		t.Errorf("expected unknown-key (definition) in error, got %v", err)
	}
}

func TestFrontmatter_MissingConceptFails(t *testing.T) {
	err := runFrontmatter([]string{"-dir=testdata/frontmatter-bad-missing-concept", "-errors-dir="})
	if err == nil {
		t.Fatal("expected failure")
	}
	if !strings.Contains(err.Error(), "concept") {
		t.Errorf("expected concept in error, got %v", err)
	}
}

func TestFrontmatter_MissingStatusFails(t *testing.T) {
	err := runFrontmatter([]string{"-dir=testdata/frontmatter-bad-missing-status", "-errors-dir="})
	if err == nil {
		t.Fatal("expected failure")
	}
	if !strings.Contains(err.Error(), "status") {
		t.Errorf("expected status in error, got %v", err)
	}
}

func TestFrontmatter_FilenameMismatchFails(t *testing.T) {
	err := runFrontmatter([]string{"-dir=testdata/frontmatter-bad-name-mismatch", "-errors-dir="})
	if err == nil {
		t.Fatal("expected failure")
	}
	if !strings.Contains(err.Error(), "does not match filename") {
		t.Errorf("expected filename-mismatch error, got %v", err)
	}
}
