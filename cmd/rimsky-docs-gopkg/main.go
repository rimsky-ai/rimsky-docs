// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

// main.go — rimsky-docs-gopkg. Generates a markdown reference for the
// hand-written Go packages of rimsky's protocols module
// (`${RIMSKY_REPO}/protocols/`) using only the standard library's go/parser
// and go/doc. The protocols module is the single public Go module a service
// implementer imports: the wire contract plus a few optional helper packages.
// This tool documents those Go packages — the contract ergonomics
// (claimproducer, lifecycle), the optional helpers (serverkit, publisherkit,
// action), and the conformance library — one section per package, listing
// exported types (with methods), funcs, and consts plus their doc comments.
//
// The generated wire reference (the protobuf bindings under proto/) is NOT
// covered here; that is rimsky-docs-proto's job. This tool skips the proto/
// subtree so the two references do not overlap.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	rimskyRepo := os.Getenv("RIMSKY_REPO")
	if rimskyRepo == "" {
		fmt.Fprintln(os.Stderr, "rimsky-docs-gopkg: RIMSKY_REPO is unset.")
		fmt.Fprintln(os.Stderr, "Set RIMSKY_REPO to a local rimsky checkout path, e.g.:")
		fmt.Fprintln(os.Stderr, "  RIMSKY_REPO=$(pwd)/../rimsky go run ./cmd/rimsky-docs-gopkg")
		fmt.Fprintln(os.Stderr, "Required by the docs reconciliation gate to cross-check generated content against rimsky source.")
		os.Exit(2)
	}

	defaultProtocolsDir := rimskyRepo + "/protocols"
	protocolsDir := flag.String("protocols-dir", defaultProtocolsDir, "rimsky protocols module directory (defaults to ${RIMSKY_REPO}/protocols)")
	out := flag.String("out", "../docs/protocols/go-packages.md", "path to write the Go package reference (relative to cmd/ cwd)")
	check := flag.Bool("check", false, "verify existing output matches regenerated content; exit non-zero on diff")
	flag.Parse()

	if err := run(*protocolsDir, *out, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
