// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

package main

import (
	"strings"
	"testing"
)

func TestSymbolExistence_KnownSymbolsPass(t *testing.T) {
	// guide.md names OpenRequest / StreamClose / ExecuteEvent in code spans
	// (all in the oracle) and GitHub / PostgreSQL in prose (skipped).
	err := runSymbolExistence([]string{
		"-guides=testdata/symbol-existence-good/guides",
		"-oracle=testdata/symbol-existence-good/oracle",
	})
	if err != nil {
		t.Errorf("expected pass, got %v", err)
	}
}

func TestSymbolExistence_UnknownSymbolFails(t *testing.T) {
	err := runSymbolExistence([]string{
		"-guides=testdata/symbol-existence-bad/guides",
		"-oracle=testdata/symbol-existence-bad/oracle",
	})
	if err == nil {
		t.Fatal("expected failure")
	}
	if !strings.Contains(err.Error(), "BogusMessage") {
		t.Errorf("expected the unknown symbol in the error, got %v", err)
	}
}

func TestSymbolExistence_AllowlistExempts(t *testing.T) {
	err := runSymbolExistence([]string{
		"-guides=testdata/symbol-existence-bad/guides",
		"-oracle=testdata/symbol-existence-bad/oracle",
		"-allow=BogusMessage",
	})
	if err != nil {
		t.Errorf("expected -allow to exempt the symbol, got %v", err)
	}
}

func TestSymbolExistence_MissingGuidesSkipped(t *testing.T) {
	err := runSymbolExistence([]string{
		"-guides=testdata/symbol-existence-does-not-exist",
		"-oracle=testdata/symbol-existence-good/oracle",
	})
	if err != nil {
		t.Errorf("expected missing guides root to be skipped, got %v", err)
	}
}
