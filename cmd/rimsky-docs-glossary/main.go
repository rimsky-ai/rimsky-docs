// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

// main.go — rimsky-docs-glossary. Publishes rimsky's auto-generated concept
// catalog (`${RIMSKY_REPO}/.ok-planner/design/concepts.md`) verbatim to the
// public glossary (docs/glossary.md).
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
	// The concept catalog/glossary lives in rimsky at
	// `.ok-planner/design/concepts.md` — an auto-generated markdown doc that
	// IS the public glossary. We publish it verbatim.
	defaultCatalog := rimskyRepo + "/.ok-planner/design/concepts.md"
	catalogFile := flag.String("catalog", defaultCatalog, "path to the source concept catalog (defaults to ${RIMSKY_REPO}/.ok-planner/design/concepts.md)")
	outputFile := flag.String("output", "../rimsky/skills/rimsky/docs/glossary.md", "path to write the published glossary (relative to cmd/ cwd)")
	check := flag.Bool("check", false, "verify existing output matches source catalog; exit non-zero on diff")
	flag.Parse()

	if err := run(*catalogFile, *outputFile, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(catalogPath, outputPath string, check bool) error {
	got, err := os.ReadFile(catalogPath)
	if err != nil {
		return fmt.Errorf("%s: %w", catalogPath, err)
	}
	if check {
		want, err := os.ReadFile(outputPath)
		if err != nil {
			return fmt.Errorf("%s: %w", outputPath, err)
		}
		if !bytes.Equal(got, want) {
			return fmt.Errorf("%s differs from source catalog %s; run `go run ./cmd/rimsky-docs-glossary` to regenerate", outputPath, catalogPath)
		}
		return nil
	}
	return os.WriteFile(outputPath, got, 0644)
}
