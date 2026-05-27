// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

package main

import (
	"strings"
	"testing"
)

func TestLinkValidity_ResolvedLinksPass(t *testing.T) {
	// links-good/a.md exercises a sibling file, a subdir file, a directory,
	// an in-page anchor, a file#fragment link, an external URL, and an image.
	err := runLinkValidity([]string{"-scope=testdata/links-good"})
	if err != nil {
		t.Errorf("expected pass, got %v", err)
	}
}

func TestLinkValidity_BrokenLinkFails(t *testing.T) {
	err := runLinkValidity([]string{"-scope=testdata/links-bad"})
	if err == nil {
		t.Fatal("expected failure")
	}
	if !strings.Contains(err.Error(), "does-not-exist.md") {
		t.Errorf("expected the missing target in the error, got %v", err)
	}
}

func TestLinkValidity_MissingScopeSkipped(t *testing.T) {
	err := runLinkValidity([]string{"-scope=testdata/links-does-not-exist"})
	if err != nil {
		t.Errorf("expected missing scope root to be skipped silently, got %v", err)
	}
}
