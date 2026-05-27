// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// fmShape is the new concept frontmatter: exactly four keys. `concept` and
// `status` are required; `aliases` and `references` are optional lists.
// KnownFields(true) rejects any unexpected key.
type fmShape struct {
	Concept    string   `yaml:"concept"`
	Status     string   `yaml:"status"`
	Aliases    []string `yaml:"aliases"`
	References []string `yaml:"references"`
}

// errorFM is the frontmatter shape for files under docs/agents/errors/.
// Per spec §8.4, every entry has `error: <code>` and `surfaced_to: <one
// of the allowlist>`. Anything else fails the lint.
type errorFM struct {
	Error      string `yaml:"error"`
	SurfacedTo string `yaml:"surfaced_to"`
}

// surfacedToAllowlist is spec §8.4 verbatim. Adding new values here is a
// spec-level change.
var surfacedToAllowlist = map[string]struct{}{
	"executor":             {},
	"claim-producer":       {},
	"lifecycle-subscriber": {},
	"operator":             {},
	"cli-user":             {},
}

func runFrontmatter(args []string) error {
	fs := flag.NewFlagSet("frontmatter", flag.ContinueOnError)
	dir := fs.String("dir", "../docs/concepts", "concept directory to validate (relative to cmd/ cwd)")
	errorsDir := fs.String("errors-dir", "../docs/agents/errors", "errors directory to validate (relative to cmd/ cwd)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var problems []string
	conceptEntries, err := os.ReadDir(*dir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, e := range conceptEntries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "README.md" {
			continue
		}
		if err := validateFile(filepath.Join(*dir, e.Name())); err != nil {
			problems = append(problems, err.Error())
		}
	}
	if *errorsDir != "" {
		errorEntries, err := os.ReadDir(*errorsDir)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		for _, e := range errorEntries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "README.md" {
				continue
			}
			if err := validateErrorFile(filepath.Join(*errorsDir, e.Name())); err != nil {
				problems = append(problems, err.Error())
			}
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("frontmatter validation failed:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}

func validateFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.HasPrefix(raw, []byte("---\n")) {
		return fmt.Errorf("%s: missing frontmatter (must start with `---`)", path)
	}
	end := bytes.Index(raw[4:], []byte("\n---\n"))
	if end < 0 {
		return fmt.Errorf("%s: unterminated frontmatter", path)
	}
	fm := &fmShape{}
	dec := yaml.NewDecoder(bytes.NewReader(raw[4 : 4+end]))
	dec.KnownFields(true)
	if err := dec.Decode(fm); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	var missing []string
	if strings.TrimSpace(fm.Concept) == "" {
		missing = append(missing, "concept")
	}
	if strings.TrimSpace(fm.Status) == "" {
		missing = append(missing, "status")
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s: missing required field(s): %s", path, strings.Join(missing, ", "))
	}
	expectedConcept := strings.TrimSuffix(filepath.Base(path), ".md")
	if fm.Concept != expectedConcept {
		return fmt.Errorf("%s: frontmatter `concept: %s` does not match filename (expected %q)", path, fm.Concept, expectedConcept)
	}
	return nil
}

// validateErrorFile enforces the docs/agents/errors/<file>.md frontmatter
// shape from spec §8.4: `error:` and `surfaced_to:` are both required, and
// `surfaced_to:` must be one of the allowlist values.
func validateErrorFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.HasPrefix(raw, []byte("---\n")) {
		return fmt.Errorf("%s: missing frontmatter (must start with `---`)", path)
	}
	end := bytes.Index(raw[4:], []byte("\n---\n"))
	if end < 0 {
		return fmt.Errorf("%s: unterminated frontmatter", path)
	}
	fm := &errorFM{}
	dec := yaml.NewDecoder(bytes.NewReader(raw[4 : 4+end]))
	dec.KnownFields(true)
	if err := dec.Decode(fm); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	var missing []string
	if strings.TrimSpace(fm.Error) == "" {
		missing = append(missing, "error")
	}
	if strings.TrimSpace(fm.SurfacedTo) == "" {
		missing = append(missing, "surfaced_to")
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s: missing required field(s): %s", path, strings.Join(missing, ", "))
	}
	if _, ok := surfacedToAllowlist[fm.SurfacedTo]; !ok {
		allowed := []string{"executor", "claim-producer", "lifecycle-subscriber", "operator", "cli-user"}
		return fmt.Errorf("%s: surfaced_to %q not in allowlist (must be one of %s)", path, fm.SurfacedTo, strings.Join(allowed, ", "))
	}
	return nil
}
