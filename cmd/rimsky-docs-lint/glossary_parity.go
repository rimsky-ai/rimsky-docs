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

// runGlossaryParity shells out to the glossary binary in check mode. The
// glossary binary derives its `-catalog` from RIMSKY_REPO, so we only pass
// `-output` and `-check=true`; non-zero exit means docs/glossary.md drifted
// from the source concept catalog.
func runGlossaryParity(args []string) error {
	fs := flag.NewFlagSet("glossary-parity", flag.ContinueOnError)
	outputPath := fs.String("output", "../docs/glossary.md", "path to existing glossary file (relative to exec cwd)")
	// repoRoot is the cwd for the inner `go run ./rimsky-docs-glossary`.
	// Default "." resolves the module path from the lint binary's own cwd
	// (rimsky-docs/cmd/). Tests override it to point at the module root.
	repoRoot := fs.String("repo-root", ".", "cwd for the inner `go run ./rimsky-docs-glossary` (default: current dir, i.e. rimsky-docs/cmd/)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cmd := exec.Command("go", "run", "./rimsky-docs-glossary",
		"-output="+*outputPath, "-check=true")
	cmd.Dir = *repoRoot
	cmd.Env = append(os.Environ(), "RIMSKY_REPO="+os.Getenv("RIMSKY_REPO"))
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("glossary parity failed: %w", err)
	}
	return nil
}
