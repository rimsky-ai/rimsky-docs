// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
)

func runGlossaryParity(args []string) error {
	fs := flag.NewFlagSet("glossary-parity", flag.ContinueOnError)
	// Concept catalog lives in rimsky's `.ok-planner/design/concepts/`;
	// the glossary lives in this repo at `../docs/glossary.md`.
	defaultConcepts := os.Getenv("RIMSKY_REPO") + "/.ok-planner/design/concepts"
	outputPath := fs.String("output", "../docs/glossary.md", "path to existing glossary file (relative to exec cwd)")
	conceptsDir := fs.String("concepts-dir", defaultConcepts, "path to concept files (defaults to ${RIMSKY_REPO}/.ok-planner/design/concepts)")
	execCwd := fs.String("exec-cwd", ".", "cwd for the inner `go run ./rimsky-docs-glossary` (default: current dir of this lint binary, which is rimsky-docs/cmd/)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cmd := exec.Command("go", "run", "./rimsky-docs-glossary",
		"-concepts-dir="+*conceptsDir, "-output="+*outputPath, "-check=true")
	cmd.Dir = *execCwd
	cmd.Env = append(os.Environ(), "RIMSKY_REPO="+os.Getenv("RIMSKY_REPO"))
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("glossary parity failed: %w", err)
	}
	return nil
}
