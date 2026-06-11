// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rimsky-ai/rimsky-docs/cmd/internal/refpin"
)

// referenceGenerators are the binaries that generate a documentation reference
// directly from rimsky source. Each supports a -check mode that regenerates and
// byte-compares against the committed output (using the binary's own default
// output path, which points at the relocated corpus), exiting non-zero on any
// drift. The glossary generator is checked separately by glossary-parity.
var referenceGenerators = []string{
	"rimsky-docs-proto",           // protocols/reference/*.md (needs protoc)
	"rimsky-docs-gopkg",           // protocols/go-packages.md
	"rimsky-docs-template-ref",    // reference/template-schema.md
	"rimsky-docs-rest-ref",        // reference/rest-api.md
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
	return checkVersionBanners(*repoRoot)
}

// checkVersionBanners verifies every generated reference page carries the
// version banner naming the reconciledAgainst release. Parity alone proves
// committed == regenerated; this additionally proves the regenerated content
// states which rimsky release it reflects — a generator regression that drops
// the banner would pass parity but ship unversioned references.
func checkVersionBanners(repoRoot string) error {
	version, err := refpin.Resolve(filepath.Join(repoRoot, "../rimsky/.claude-plugin/plugin.json"))
	if err != nil {
		return fmt.Errorf("version-banner check: %w", err)
	}
	banner := refpin.Banner(version)

	docsRoot := filepath.Join(repoRoot, "../rimsky/skills/rimsky/docs")
	pages := []string{
		filepath.Join(docsRoot, "protocols/go-packages.md"),
		filepath.Join(docsRoot, "reference/template-schema.md"),
		filepath.Join(docsRoot, "reference/rest-api.md"),
		filepath.Join(docsRoot, "reference/cli.md"),
	}
	wireRefs, err := filepath.Glob(filepath.Join(docsRoot, "protocols/reference/*.md"))
	if err != nil {
		return err
	}
	pages = append(pages, wireRefs...)

	var missing []string
	for _, page := range pages {
		content, err := os.ReadFile(page)
		if err != nil {
			missing = append(missing, page+" (unreadable: "+err.Error()+")")
			continue
		}
		if !strings.Contains(string(content), banner) {
			missing = append(missing, page)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("generated reference(s) missing the version banner for %s (regenerate with the current generators):\n  %s", version, strings.Join(missing, "\n  "))
	}
	return nil
}
