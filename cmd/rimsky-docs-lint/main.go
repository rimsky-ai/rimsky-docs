// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

// main.go — rimsky-docs-lint. Five structural lints enforce the integrity of
// the public-documentation surface (docs/concepts/, docs/protocols/,
// docs/agents/, docs/glossary.md): frontmatter (concept +
// error-file frontmatter shape), glossary-parity (docs/glossary.md matches the
// rimsky concept catalog), citation-drift (every `concept:<slug>` reference
// resolves to a published concept page), llms-txt-validity (llms.txt
// well-formed), and link-validity (every relative markdown link resolves on
// disk). These check mechanical correctness only — word choice and
// user-facing clarity are the doc-writing agents' judgment, not linted.
package main

import (
	"fmt"
	"os"
)

type subcommand struct {
	name string
	fn   func(args []string) error
	desc string
}

var subcommands = []subcommand{
	{"frontmatter", runFrontmatter, "validate frontmatter shape on all concept files and error files"},
	{"glossary-parity", runGlossaryParity, "verify docs/glossary.md matches generator output"},
	{"citation-drift", runCitationDrift, "verify every `concept:<slug>` reference resolves to a published concept page"},
	{"llms-txt-validity", runLLMSTxtValidity, "verify llms.txt is well-formed and links resolve"},
	{"link-validity", runLinkValidity, "verify every relative markdown link in docs/ resolves on disk"},
	{"all", runAll, "run all five lints; exits non-zero if any fail"},
}

func main() {
	if os.Getenv("RIMSKY_REPO") == "" {
		fmt.Fprintln(os.Stderr, "rimsky-docs-lint: RIMSKY_REPO is unset.")
		fmt.Fprintln(os.Stderr, "Set RIMSKY_REPO to a local rimsky checkout path, e.g.:")
		fmt.Fprintln(os.Stderr, "  RIMSKY_REPO=$(pwd)/../rimsky go run ./cmd/rimsky-docs-lint all")
		fmt.Fprintln(os.Stderr, "Glossary-parity and similar lints cross-check against the rimsky concept catalog under ${RIMSKY_REPO}/.ok-planner/design/.")
		os.Exit(2)
	}
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	for _, sc := range subcommands {
		if sc.name == cmd {
			if err := sc.fn(os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return
		}
	}
	fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", cmd)
	usage()
	os.Exit(2)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: rimsky-docs-lint <subcommand> [flags]")
	fmt.Fprintln(os.Stderr, "subcommands:")
	for _, sc := range subcommands {
		fmt.Fprintf(os.Stderr, "  %-25s %s\n", sc.name, sc.desc)
	}
}

// allLints is the static list of subcommand names runAll iterates. It mirrors
// the entries in `subcommands` (excluding "all" itself) but is a plain slice
// to avoid an initialization cycle between runAll and the subcommands table.
var allLints = []struct {
	name string
	fn   func(args []string) error
}{
	{"frontmatter", runFrontmatter},
	{"glossary-parity", runGlossaryParity},
	{"citation-drift", runCitationDrift},
	{"llms-txt-validity", runLLMSTxtValidity},
	{"link-validity", runLinkValidity},
}

func runAll(args []string) error {
	var errs []error
	for _, sc := range allLints {
		if err := sc.fn(args); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", sc.name, err))
			fmt.Fprintf(os.Stderr, "FAIL %s: %v\n", sc.name, err)
		} else {
			fmt.Fprintf(os.Stderr, "OK   %s\n", sc.name)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d lint(s) failed", len(errs))
	}
	return nil
}
