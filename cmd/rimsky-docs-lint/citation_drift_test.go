// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

package main

import (
	"strings"
	"testing"
)

func TestCitationDrift_ResolvedReferencePasses(t *testing.T) {
	err := runCitationDrift([]string{
		"-scope=testdata/citation-good/docs,testdata/citation-good/concepts",
		"-concepts-dir=testdata/citation-good/concepts",
	})
	if err != nil {
		t.Errorf("expected pass, got %v", err)
	}
}

func TestCitationDrift_UnknownReferenceFails(t *testing.T) {
	err := runCitationDrift([]string{
		"-scope=testdata/citation-bad/docs,testdata/citation-bad/concepts",
		"-concepts-dir=testdata/citation-bad/concepts",
	})
	if err == nil {
		t.Fatal("expected failure")
	}
	if !strings.Contains(err.Error(), "references unknown concept 'does-not-exist'") {
		t.Errorf("expected unknown-concept error, got %v", err)
	}
}

func TestCitationDrift_MissingScopeSkipped(t *testing.T) {
	err := runCitationDrift([]string{
		"-scope=testdata/citation-does-not-exist",
		"-concepts-dir=testdata/citation-good/concepts",
	})
	if err != nil {
		t.Errorf("expected missing scope root to be skipped silently, got %v", err)
	}
}
