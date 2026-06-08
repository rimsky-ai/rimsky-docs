// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// referenceGenerators are the binaries that generate a documentation reference
// directly from rimsky source. Each supports a -check mode that regenerates and
// byte-compares against the committed output (using the binary's own default
// output path, which points at the relocated corpus), exiting non-zero on any
// drift. The glossary generator is checked separately by glossary-parity.
var referenceGenerators = []string{
	"rimsky-docs-proto",        // protocols/reference/*.md (needs protoc)
	"rimsky-docs-gopkg",        // protocols/go-packages.md
	"rimsky-docs-template-ref", // reference/template-schema.md
	"rimsky-docs-rest-ref",     // reference/rest-api.md
	"rimsky-docs-cli-ref",         // reference/cli.md (builds the rimsky CLI)
	"rimsky-docs-vendor-examples", // examples/ (vendored from rimsky-core/examples at the reconciled tag)
}

// runReferenceParity shells each generated-reference binary in -check mode and
// fails if any reference has drifted from what regenerating from source would
// produce. It is the accuracy model's ring 1 as a gate: a generated surface that
// no longer matches source — because someone hand-edited it, or the source moved
// without a regen — is caught here rather than shipped stale.
//
// Unlike the other lints this is NOT corpus-internal: it needs RIMSKY_REPO (a
// rimsky checkout), `protoc` (for the proto reference), and a buildable rimsky
// tree (for the CLI reference). It is the heaviest check in the suite.
func runReferenceParity(args []string) error {
	fs := flag.NewFlagSet("reference-parity", flag.ContinueOnError)
	// repoRoot is the cwd for the inner `go run`. Default "." is the lint
	// binary's own cwd (rimsky-docs/cmd/); tests override it to the module root.
	repoRoot := fs.String("repo-root", ".", "cwd for the inner `go run ./rimsky-docs-*` (default: rimsky-docs/cmd/)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var failed []string
	for _, gen := range referenceGenerators {
		cmd := exec.Command("go", "run", "./"+gen, "-check=true")
		cmd.Dir = *repoRoot
		cmd.Env = append(os.Environ(), "RIMSKY_REPO="+os.Getenv("RIMSKY_REPO"))
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			failed = append(failed, gen)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("generated reference(s) failed parity (stale, or could not be regenerated — see stderr; run the generator to refresh): %s", strings.Join(failed, ", "))
	}
	return nil
}
