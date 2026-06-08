// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

// main.go — rimsky-docs-vendor-examples. Vendors the Apache-licensed example
// servers from a pinned rimsky checkout (${RIMSKY_REPO}/examples) into the
// corpus (docs/examples/) as read-and-adapt source.
//
// The source of truth is rimsky-core/examples — a real, gate-tested module.
// This tool only projects a snapshot into the docs, exactly as the other
// generators project the proto/go-package references. Transforms:
//
//   - *.go            copied verbatim (the import paths are full-module, so the
//                     copied source compiles unchanged against a pinned tag).
//   - go.mod          the in-tree `replace ... => ../lib/protocols` is dropped
//                     and the lib/protocols require is pinned to the reconciled
//                     tag, so the copied module is standalone.
//   - README.md       a generated version banner is prepended.
//   - go.sum, bin     omitted; a copier runs `go mod tidy`.
//
// Nobody imports the vendored copy as a dependency (a success-only example is
// not deployable) — the shipped go.mod states the exact pin and anchors the
// version banner. -check regenerates and byte-compares for the reference-parity
// lint.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const protocolsModule = "github.com/rimsky-ai/rimsky-core/lib/protocols"

func main() {
	rimskyRepo := os.Getenv("RIMSKY_REPO")
	if rimskyRepo == "" {
		fmt.Fprintln(os.Stderr, "rimsky-docs-vendor-examples: RIMSKY_REPO is unset.")
		fmt.Fprintln(os.Stderr, "Set RIMSKY_REPO to a local rimsky checkout path, e.g.:")
		fmt.Fprintln(os.Stderr, "  RIMSKY_REPO=$(pwd)/../rimsky-core go run ./cmd/rimsky-docs-vendor-examples")
		os.Exit(2)
	}
	src := flag.String("src", rimskyRepo+"/examples", "source examples module (defaults to ${RIMSKY_REPO}/examples)")
	out := flag.String("out", "../rimsky/skills/rimsky/docs/examples", "corpus output dir (relative to cmd/ cwd)")
	version := flag.String("version", "", "lib/protocols tag to pin (default: $RIMSKY_TAG, else plugin.json reconciledAgainst)")
	pluginJSON := flag.String("plugin", "../rimsky/.claude-plugin/plugin.json", "plugin.json for the version fallback")
	check := flag.Bool("check", false, "verify existing output matches; exit non-zero on drift")
	flag.Parse()

	ver := resolveVersion(*version, *pluginJSON)
	if ver == "" {
		fmt.Fprintln(os.Stderr, "rimsky-docs-vendor-examples: could not determine lib/protocols version (set -version, $RIMSKY_TAG, or plugin.json reconciledAgainst)")
		os.Exit(1)
	}

	if err := run(*src, *out, ver, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// resolveVersion picks the lib/protocols tag to pin: explicit flag, then the
// build-docs-exported $RIMSKY_TAG, then the corpus's own reconciledAgainst.
func resolveVersion(flagVal, pluginPath string) string {
	if flagVal != "" {
		return flagVal
	}
	if t := os.Getenv("RIMSKY_TAG"); t != "" {
		return t
	}
	data, err := os.ReadFile(pluginPath)
	if err != nil {
		return ""
	}
	var p struct {
		ReconciledAgainst string `json:"reconciledAgainst"`
	}
	if json.Unmarshal(data, &p) != nil {
		return ""
	}
	return p.ReconciledAgainst
}

func run(srcDir, outDir, version string, check bool) error {
	want, err := vendorSet(srcDir, version)
	if err != nil {
		return fmt.Errorf("read source examples %s: %w", srcDir, err)
	}
	if len(want) == 0 {
		return fmt.Errorf("no example files found under %s", srcDir)
	}
	if check {
		return checkAgainst(outDir, want)
	}
	return writeOut(outDir, want)
}

// vendorSet computes the deterministic relpath->content map of vendored files.
func vendorSet(srcDir, version string) (map[string][]byte, error) {
	set := map[string][]byte{}
	err := filepath.WalkDir(srcDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if skip(rel) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		switch rel {
		case "go.mod":
			content = rewriteGoMod(content, version)
		case "README.md":
			content = append([]byte(banner(version)), content...)
		}
		set[rel] = content
		return nil
	})
	return set, err
}

// skip drops build artifacts and anything that shouldn't be vendored. go.sum is
// omitted because a copier regenerates it with `go mod tidy`.
func skip(rel string) bool {
	if rel == "go.sum" {
		return true
	}
	if strings.HasPrefix(rel, ".") || strings.Contains(rel, "/.") {
		return true
	}
	if strings.HasPrefix(rel, "bin/") {
		return true
	}
	return false
}

// rewriteGoMod drops the in-tree replace and pins the lib/protocols require to
// the reconciled tag, yielding a standalone module file.
func rewriteGoMod(content []byte, version string) []byte {
	var out []string
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "replace ") {
			continue
		}
		if strings.HasPrefix(trimmed, protocolsModule+" ") {
			out = append(out, "\t"+protocolsModule+" "+version)
			continue
		}
		out = append(out, line)
	}
	joined := strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
	return []byte(joined)
}

func banner(version string) string {
	return fmt.Sprintf(
		"<!-- AUTOGENERATED by rimsky-docs-vendor-examples from rimsky-core examples/ at lib/protocols %s.\n"+
			"     Do not edit here; edit examples/ in rimsky-core and re-run /build-docs. -->\n"+
			"<!-- This code reflects rimsky lib/protocols %s. For another pin, read the package at your tag. -->\n\n",
		version, version)
}

// checkAgainst compares the existing output tree to the regenerated set,
// failing on any missing, extra, or differing file.
func checkAgainst(outDir string, want map[string][]byte) error {
	got := map[string][]byte{}
	err := filepath.WalkDir(outDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(outDir, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		got[filepath.ToSlash(rel)] = content
		return nil
	})
	if err != nil {
		return fmt.Errorf("read vendored output %s: %w (run `go run ./cmd/rimsky-docs-vendor-examples` to regenerate)", outDir, err)
	}
	var diffs []string
	for rel, w := range want {
		g, ok := got[rel]
		if !ok {
			diffs = append(diffs, "missing: "+rel)
		} else if !bytes.Equal(g, w) {
			diffs = append(diffs, "differs: "+rel)
		}
	}
	for rel := range got {
		if _, ok := want[rel]; !ok {
			diffs = append(diffs, "stale: "+rel)
		}
	}
	if len(diffs) > 0 {
		sort.Strings(diffs)
		return fmt.Errorf("vendored examples drifted from source; run `go run ./cmd/rimsky-docs-vendor-examples` to regenerate:\n  %s",
			strings.Join(diffs, "\n  "))
	}
	return nil
}

// writeOut replaces the generator-owned output dir with the regenerated set.
func writeOut(outDir string, want map[string][]byte) error {
	if filepath.Base(outDir) != "examples" {
		return fmt.Errorf("refusing to write: -out %q does not end in 'examples' (safety check)", outDir)
	}
	if err := os.RemoveAll(outDir); err != nil {
		return fmt.Errorf("clean %s: %w", outDir, err)
	}
	for rel, content := range want {
		dst := filepath.Join(outDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}
