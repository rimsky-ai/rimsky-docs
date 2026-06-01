// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

package main

import (
	"strings"
	"testing"
)

// TestReferenceParity_DetectsGeneratorFailure exercises the orchestration: with
// RIMSKY_REPO unset, every generator exits non-zero (it cannot reconcile), and
// reference-parity must aggregate that into one failure naming the generators.
// The all-pass path needs a real RIMSKY_REPO + protoc + a buildable rimsky tree,
// so it is covered by the integration gate (`rimsky-docs-lint reference-parity`
// against a checkout), not a hermetic unit test.
func TestReferenceParity_DetectsGeneratorFailure(t *testing.T) {
	moduleRoot := findModuleRoot(t)
	t.Setenv("RIMSKY_REPO", "")
	err := runReferenceParity([]string{"-repo-root=" + moduleRoot})
	if err == nil {
		t.Fatal("expected failure when the generators cannot reconcile")
	}
	if !strings.Contains(err.Error(), "rimsky-docs-") {
		t.Errorf("expected the failing generator(s) named in the error, got %v", err)
	}
}
