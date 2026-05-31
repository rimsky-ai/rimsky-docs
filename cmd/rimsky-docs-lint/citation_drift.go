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
	"strings"
)

// conceptRefRE matches inline concept-reference tokens like `concept:claim` or
// `` `concept:claim-handle` `` that appear in concept prose. The slug is the
// captured group and must match the published concept filename (minus `.md`).
var conceptRefRE = regexp.MustCompile(`concept:([a-z0-9-]+)`)

func runCitationDrift(args []string) error {
	fs := flag.NewFlagSet("citation-drift", flag.ContinueOnError)
	publicSurface := fs.String("scope", "../rimsky/skills/rimsky/docs/concepts,../rimsky/skills/rimsky/docs/protocols", "comma-separated public-surface roots (relative to cmd/ cwd)")
	conceptsDir := fs.String("concepts-dir", "../rimsky/skills/rimsky/docs/concepts", "path to published concept pages (reference targets; relative to cmd/ cwd)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var hits []string
	for _, root := range strings.Split(*publicSurface, ",") {
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
			if info.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}
			fileHits, err := scanConceptRefs(path, *conceptsDir)
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
		return fmt.Errorf("citation drift detected:\n  - %s", strings.Join(hits, "\n  - "))
	}
	return nil
}

// scanConceptRefs reports every `concept:<slug>` reference in path whose
// published target page (`<conceptsDir>/<slug>.md`) does not exist.
func scanConceptRefs(path, conceptsDir string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	var hits []string
	line := 0
	for scanner.Scan() {
		line++
		for _, m := range conceptRefRE.FindAllStringSubmatch(scanner.Text(), -1) {
			slug := m[1]
			target := filepath.Join(conceptsDir, slug+".md")
			if _, err := os.Stat(target); err != nil {
				if os.IsNotExist(err) {
					hits = append(hits, fmt.Sprintf("%s:%d references unknown concept '%s'", path, line, slug))
					continue
				}
				return nil, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return hits, nil
}
