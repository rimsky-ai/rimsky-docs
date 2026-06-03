// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// camelRE matches a multi-word CamelCase identifier at a word boundary: a
// capital, at least one lower/digit, a second capital, then the rest. This is
// the distinctive shape of a proto message / RPC / enum or Go type name
// (ExecuteRequest, StreamClose, PublisherKindCapability). It deliberately does
// NOT match single-word capitals (Success, Error, Struct, Timestamp — too
// ambiguous with prose), all-caps acronyms (HTTP, JSON, MCP), lower-initial
// names (gRPC), or a CamelCase run embedded mid-token (the `\b` keeps
// `lowerCamelCase` from matching the inner `CamelCase`).
var camelRE = regexp.MustCompile(`\b[A-Z][a-z0-9]+[A-Z][A-Za-z0-9]*\b`)

// inlineCodeRE captures the contents of an inline-code span, `like this`.
var inlineCodeRE = regexp.MustCompile("`([^`]+)`")

// verifiedInternalSymbols are multi-word CamelCase identifiers the docs cite that
// are real — verified in rimsky source (lib/ internal types, or stub APIs under
// test/support/) or a well-known stdlib/external API — but absent from the
// *public* generated references. They are exempt so the non-protocol surfaces
// can name them. Adding here is a deliberate "verified-real" decision: confirm
// the symbol exists in source first. The protocol guides need none of these —
// their symbols ARE the public references; this list is the cost of checking the
// broader corpus against a public-only oracle. A source-backed oracle would
// remove the need for it.
var verifiedInternalSymbols = []string{
	// rimsky internal (lib/)
	"FrameDeliveryMode", "FrameResolutionMode", "SweepOrphanedBlobs",
	"NodeID", "AttributeName", "CheckGrant", "ValidateBlobConfig", "LargeObjects",
	"IsPermissiveExecutorSchema",  // lib/graph/node/template_validator.go::IsPermissiveExecutorSchema
	"ListByClaimHandleID",         // lib/foundation/persistence/claim_holders.go::ClaimHolderTable.ListByClaimHandleID
	"HoldingSubgraphsForTemplate", // lib/graph/node/inheritance.go::HoldingSubgraphsForTemplate
	"IsHeld",                      // lib/graph/node/inheritance.go::HoldingSubgraph.IsHeld
	"OrphanBlobSweepInterval",     // lib/graph/scheduler/scheduler.go::Config.OrphanBlobSweepInterval
	// stub test-support APIs (test/support/)
	"WhenType", "EmitNamedEvent", "EnableStubMode", "StubAttributesFor",
	"EnableDataProcessing", "EnableLifecycle",
	// stdlib / external
	"ExpandEnv", "FromDockerfile",
}

// runSymbolExistence verifies that every multi-word CamelCase symbol a
// hand-written guide names in a code span — an inline `Backtick` span, or any
// line inside a fenced block — appears in the generated references (the in-repo
// projection of the source). It is the accuracy gate for the #1 hallucination
// class: a guide naming a type / message / RPC that does not exist.
//
// It is corpus-internal — no RIMSKY_REPO — because the generated references are
// already reconciled against source (their parity is reference-parity's job); a
// guide's named symbols must be a subset of theirs. The default scope is the
// whole corpus: the protocol guides match the references exactly, while other
// surfaces also cite internal/stdlib symbols the public references don't surface
// — those are exempted via verifiedInternalSymbols (above) or -allow.
func runSymbolExistence(args []string) error {
	fs := flag.NewFlagSet("symbol-existence", flag.ContinueOnError)
	guides := fs.String("guides", "../rimsky/skills/rimsky/docs", "comma-separated guide roots to check (relative to cmd/ cwd; the reference/ dirs and go-packages.md are auto-skipped — they are the oracle)")
	oracle := fs.String("oracle", "../rimsky/skills/rimsky/docs/protocols/reference,../rimsky/skills/rimsky/docs/protocols/go-packages.md,../rimsky/skills/rimsky/docs/reference", "comma-separated generated-reference roots/files defining the known-symbol set (relative to cmd/ cwd)")
	allow := fs.String("allow", "", "comma-separated CamelCase symbols to exempt (legit symbols absent from every generated reference)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	known, err := buildSymbolOracle(*oracle)
	if err != nil {
		return err
	}
	for _, s := range verifiedInternalSymbols {
		known[s] = struct{}{}
	}
	for _, a := range strings.Split(*allow, ",") {
		if a = strings.TrimSpace(a); a != "" {
			known[a] = struct{}{}
		}
	}

	var hits []string
	for _, root := range strings.Split(*guides, ",") {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if info.IsDir() {
				// The generated references are the oracle, not guides to check.
				if info.Name() == "reference" {
					return filepath.SkipDir
				}
				return nil
			}
			// go-packages.md is itself a generated reference (the oracle), not a guide.
			if !strings.HasSuffix(path, ".md") || info.Name() == "go-packages.md" {
				return nil
			}
			fileHits, err := scanGuideSymbols(path, known)
			if err != nil {
				return err
			}
			hits = append(hits, fileHits...)
			return nil
		})
		if err != nil {
			return err
		}
	}
	if len(hits) > 0 {
		return fmt.Errorf("symbols named in a guide but absent from every generated reference (hallucination or drift; -allow if a legit external symbol):\n  - %s", strings.Join(hits, "\n  - "))
	}
	return nil
}

// buildSymbolOracle collects every multi-word CamelCase token appearing
// anywhere in the generated-reference roots/files — the set of symbols that
// provably exist in the source they were generated from.
func buildSymbolOracle(roots string) (map[string]struct{}, error) {
	known := map[string]struct{}{}
	add := func(path string) error {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range camelRE.FindAllString(string(raw), -1) {
			known[m] = struct{}{}
		}
		return nil
	}
	for _, root := range strings.Split(roots, ",") {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		info, err := os.Stat(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if !info.IsDir() {
			if err := add(root); err != nil {
				return nil, err
			}
			continue
		}
		err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}
			return add(path)
		})
		if err != nil {
			return nil, err
		}
	}
	return known, nil
}

// scanGuideSymbols reports every multi-word CamelCase symbol that appears in a
// code span of the guide — an inline `code` span, or any line inside a fenced
// block — but is not in the known set. Symbols in prose (unbackticked) are
// skipped: backticking is the doc's own signal "this is a code symbol", which
// keeps proper nouns (GitHub, PostgreSQL) out of the check. Findings are
// deduped per file.
func scanGuideSymbols(path string, known map[string]struct{}) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	var hits []string
	seen := map[string]struct{}{}
	line := 0
	inFence := false
	for scanner.Scan() {
		line++
		text := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(text), "```") {
			inFence = !inFence
			continue
		}
		var regions []string
		if inFence {
			regions = []string{text}
		} else {
			for _, m := range inlineCodeRE.FindAllStringSubmatch(text, -1) {
				regions = append(regions, m[1])
			}
		}
		for _, region := range regions {
			for _, sym := range camelRE.FindAllString(region, -1) {
				if _, ok := known[sym]; ok {
					continue
				}
				if _, dup := seen[sym]; dup {
					continue
				}
				seen[sym] = struct{}{}
				hits = append(hits, fmt.Sprintf("%s:%d unknown symbol %q", path, line, sym))
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Strings(hits)
	return hits, nil
}
