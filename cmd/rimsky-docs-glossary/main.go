// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

// main.go — rimsky-docs-glossary. Reads docs/concepts/*.md frontmatter
// and emits docs/glossary.md.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
)

func main() {
	rimskyRepo := os.Getenv("RIMSKY_REPO")
	if rimskyRepo == "" {
		fmt.Fprintln(os.Stderr, "rimsky-docs-glossary: RIMSKY_REPO is unset.")
		fmt.Fprintln(os.Stderr, "Set RIMSKY_REPO to a local rimsky checkout path, e.g.:")
		fmt.Fprintln(os.Stderr, "  RIMSKY_REPO=$(pwd)/../rimsky go run ./cmd/rimsky-docs-glossary")
		fmt.Fprintln(os.Stderr, "Required by the docs reconciliation gate to cross-check generated content against rimsky source.")
		os.Exit(2)
	}
	// Concept catalog lives in rimsky's `.ok-planner/design/concepts/` —
	// the authoritative source for public-glossary frontmatter per
	// `concept:module-layout` and the rimsky CLAUDE.md pointer index.
	defaultConcepts := rimskyRepo + "/.ok-planner/design/concepts"
	conceptsDir := flag.String("concepts-dir", defaultConcepts, "path to concept files (defaults to ${RIMSKY_REPO}/.ok-planner/design/concepts)")
	outputFile := flag.String("output", "../docs/glossary.md", "path to write generated glossary (relative to cmd/ cwd)")
	check := flag.Bool("check", false, "verify existing output matches generated; exit non-zero on diff")
	flag.Parse()

	if err := run(*conceptsDir, *outputFile, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(conceptsDir, outputPath string, check bool) error {
	got, err := generate(conceptsDir)
	if err != nil {
		return err
	}
	if check {
		want, err := os.ReadFile(outputPath)
		if err != nil {
			return fmt.Errorf("%s: %w", outputPath, err)
		}
		if !bytes.Equal(got, want) {
			return fmt.Errorf("%s differs from generator output; run `go run ./cmd/rimsky-docs-glossary` to regenerate", outputPath)
		}
		return nil
	}
	return os.WriteFile(outputPath, got, 0644)
}
