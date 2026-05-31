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

// linkRE matches inline markdown link/image targets — the `(...)` in
// `[text](target)` and `![alt](target)`. The target is the captured group.
var linkRE = regexp.MustCompile(`\]\(([^)]+)\)`)

// runLinkValidity verifies that every relative markdown link/image target in
// the published docs resolves to a real file or directory on disk. External
// links (http/https/mailto), in-page anchors (`#…`), and the anchor fragment
// of `file.md#section` links are not link targets to a local file and are
// skipped. This catches the failure class the other lints miss: a guide that
// links to a moved/renamed/source-tree path, or a cross-surface link to a file
// another surface removed.
func runLinkValidity(args []string) error {
	fs := flag.NewFlagSet("link-validity", flag.ContinueOnError)
	scope := fs.String("scope", "../rimsky/skills/rimsky/docs", "comma-separated doc roots to scan (relative to cmd/ cwd)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var hits []string
	for _, root := range strings.Split(*scope, ",") {
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
			fileHits, err := scanLinks(path)
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
		return fmt.Errorf("broken relative links detected:\n  - %s", strings.Join(hits, "\n  - "))
	}
	return nil
}

// scanLinks reports every relative markdown link/image target in path whose
// resolved on-disk target (relative to the file's directory) does not exist.
func scanLinks(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dir := filepath.Dir(path)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	var hits []string
	line := 0
	for scanner.Scan() {
		line++
		for _, m := range linkRE.FindAllStringSubmatch(scanner.Text(), -1) {
			target := localLinkTarget(m[1])
			if target == "" {
				continue
			}
			if _, err := os.Stat(filepath.Join(dir, target)); err != nil {
				if os.IsNotExist(err) {
					hits = append(hits, fmt.Sprintf("%s:%d links to missing target %q", path, line, target))
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

// localLinkTarget normalizes a raw markdown link target to the local path that
// must exist on disk, or "" if it is not a local-file reference (external URL
// or in-page anchor). The `#fragment` of a `file#section` link is stripped.
func localLinkTarget(raw string) string {
	t := strings.TrimSpace(raw)
	// A markdown link target may carry an optional title: `(path "title")`.
	if i := strings.IndexAny(t, " \t"); i >= 0 {
		t = t[:i]
	}
	if t == "" || strings.HasPrefix(t, "#") {
		return ""
	}
	if strings.HasPrefix(t, "http://") || strings.HasPrefix(t, "https://") || strings.HasPrefix(t, "mailto:") {
		return ""
	}
	if i := strings.IndexByte(t, '#'); i >= 0 {
		t = t[:i]
	}
	return t
}
