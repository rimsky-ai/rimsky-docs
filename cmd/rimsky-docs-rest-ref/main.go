// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

// main.go — rimsky-docs-rest-ref. Generates a complete, mechanical reference
// for rimsky's REST / HTTP control API from the canonical action registry in
// `${RIMSKY_REPO}/lib/control/controlapi/actions.go`.
//
// The registry is the single source of truth for the control-api route → action
// mapping: the auth middleware resolves an incoming request to an action and
// gates it against the requesting key's permission grant (see
// concept:control-api). This tool parses that file's AST with the standard
// library's go/parser + go/ast — no rimsky import, no new dependencies — and
// emits one table per resource prefix listing every METHOD · PATH · action ·
// auth gate. An agent reading the output sees the entire HTTP surface without
// inferring it from prose.
//
// The MCP tool surface (also declared in actions.go) is documented elsewhere;
// this tool covers the HTTP routes only.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	rimskyRepo := os.Getenv("RIMSKY_REPO")
	if rimskyRepo == "" {
		fmt.Fprintln(os.Stderr, "rimsky-docs-rest-ref: RIMSKY_REPO is unset.")
		fmt.Fprintln(os.Stderr, "Set RIMSKY_REPO to a local rimsky checkout path, e.g.:")
		fmt.Fprintln(os.Stderr, "  RIMSKY_REPO=$(pwd)/../rimsky go run ./cmd/rimsky-docs-rest-ref")
		fmt.Fprintln(os.Stderr, "Required by the docs reconciliation gate to cross-check generated content against rimsky source.")
		os.Exit(2)
	}

	defaultActionsFile := rimskyRepo + "/lib/control/controlapi/actions.go"
	actionsFile := flag.String("actions-file", defaultActionsFile, "rimsky control-api action registry source (defaults to ${RIMSKY_REPO}/lib/control/controlapi/actions.go)")
	out := flag.String("out", "../rimsky/skills/rimsky/docs/reference/rest-api.md", "path to write the REST API reference (relative to cmd/ cwd)")
	check := flag.Bool("check", false, "verify existing output matches regenerated content; exit non-zero on diff")
	flag.Parse()

	if err := run(*actionsFile, *out, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
