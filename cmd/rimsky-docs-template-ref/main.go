// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

// main.go — rimsky-docs-template-ref. Generates a complete, mechanical
// reference for the rimsky template / `rimsky.yml` schema, derived from the
// spec structs under `${RIMSKY_REPO}/lib/foundation/spec/` using only the
// standard library's go/parser and go/doc.
//
// The spec package is pure data: the persistable row-types and enums that the
// template canonicalizer parses out of a `rimsky.yml`. This tool documents
// every exported struct (one field table per struct — YAML key, Go type, doc
// comment) and every enum (the named string types and the standalone const
// groups) so an agent can read the full schema top-down without inferring it
// from examples.
//
// This is a schema reference, not a Go-API reference: the hand-written Go
// packages a service implementer imports are rimsky-docs-gopkg's job, and the
// wire protobufs are rimsky-docs-proto's. This tool reads exactly one package
// (`lib/foundation/spec`) and emits exactly one file.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	rimskyRepo := os.Getenv("RIMSKY_REPO")
	if rimskyRepo == "" {
		fmt.Fprintln(os.Stderr, "rimsky-docs-template-ref: RIMSKY_REPO is unset.")
		fmt.Fprintln(os.Stderr, "Set RIMSKY_REPO to a local rimsky checkout path, e.g.:")
		fmt.Fprintln(os.Stderr, "  RIMSKY_REPO=$(pwd)/../rimsky go run ./cmd/rimsky-docs-template-ref")
		fmt.Fprintln(os.Stderr, "Required by the docs reconciliation gate to cross-check generated content against rimsky source.")
		os.Exit(2)
	}

	defaultSpecDir := rimskyRepo + "/lib/foundation/spec"
	specDir := flag.String("spec-dir", defaultSpecDir, "rimsky spec package directory (defaults to ${RIMSKY_REPO}/lib/foundation/spec)")
	out := flag.String("out", "../rimsky/skills/rimsky/docs/reference/template-schema.md", "path to write the template schema reference (relative to cmd/ cwd)")
	check := flag.Bool("check", false, "verify existing output matches regenerated content; exit non-zero on diff")
	flag.Parse()

	if err := run(*specDir, *out, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
