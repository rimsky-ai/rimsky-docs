// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

// main.go — rimsky-docs-cli-ref. Generates a complete, definitive reference for
// the `rimsky` CLI from the CLI's own self-documenting help output. The CLI is
// the human/scripting front-end to the REST control API; its help text is the
// authoritative description of every command, so this tool reproduces that text
// faithfully rather than paraphrasing it.
//
// It shells out to `go run ./cmd/rimsky ...` in the rimsky checkout
// (${RIMSKY_REPO}) — `go run ./cmd/rimsky help` for the top-level command tree,
// then a bare invocation of each command group (`rimsky template`,
// `rimsky instance`, ...) for that group's usage. The group list is discovered
// from the top-level help and walked in deterministic order so `-check` is
// stable. Stdlib only; no new dependencies.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	rimskyRepo := os.Getenv("RIMSKY_REPO")
	if rimskyRepo == "" {
		fmt.Fprintln(os.Stderr, "rimsky-docs-cli-ref: RIMSKY_REPO is unset.")
		fmt.Fprintln(os.Stderr, "Set RIMSKY_REPO to a local rimsky checkout path, e.g.:")
		fmt.Fprintln(os.Stderr, "  RIMSKY_REPO=$(pwd)/../rimsky go run ./cmd/rimsky-docs-cli-ref")
		fmt.Fprintln(os.Stderr, "Required by the docs reconciliation gate to cross-check generated content against rimsky source.")
		os.Exit(2)
	}

	out := flag.String("out", "../rimsky/skills/rimsky/docs/reference/cli.md", "path to write the CLI reference (relative to cmd/ cwd)")
	check := flag.Bool("check", false, "verify existing output matches regenerated content; exit non-zero on diff")
	flag.Parse()

	if err := run(rimskyRepo, *out, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
