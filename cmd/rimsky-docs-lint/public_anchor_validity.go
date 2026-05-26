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
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	// Match top-level `message`, `enum`, and `service` declarations. The
	// frontmatter field `proto_symbol` accepts any of these three top-level
	// symbol kinds — the three service-protocol concept files cite the
	// service symbol; other concept files cite messages or enums.
	protoSymbolRE = regexp.MustCompile(`(?m)^\s*(?:message|enum|service)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	configFieldRE = regexp.MustCompile(`^rimsky\.yml:[A-Za-z_][A-Za-z0-9_.\[\]]*$`)
	// Real rimsky control-api routes contain underscores
	// (e.g. `/worker_requests/{id}/trace`, `/admin/instances/{instance}/nodes/{node_id}/invalidate`).
	apiSurfaceRE = regexp.MustCompile(`^(GET|POST|PUT|DELETE|PATCH)\s+/[A-Za-z0-9_/\-{}]*$`)
)

type anchorFM struct {
	ProtoSymbol string `yaml:"proto_symbol"`
	ConfigField string `yaml:"config_field"`
	APISurface  string `yaml:"api_surface"`
}

func runPublicAnchorValidity(args []string) error {
	fs := flag.NewFlagSet("public-anchor-validity", flag.ContinueOnError)
	// Concept catalog lives in rimsky's `.ok-planner/design/concepts/`;
	// proto sources live in rimsky's `protocols/proto/v1/`.
	defaultConcepts := os.Getenv("RIMSKY_REPO") + "/.ok-planner/design/concepts"
	defaultProtoDir := os.Getenv("RIMSKY_REPO") + "/protocols/proto/v1"
	conceptsDir := fs.String("concepts-dir", defaultConcepts, "path to concept files (defaults to ${RIMSKY_REPO}/.ok-planner/design/concepts)")
	protoDir := fs.String("proto-dir", defaultProtoDir, "path to proto sources (default: ${RIMSKY_REPO}/protocols/proto/v1)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	protoSyms, err := collectProtoSymbols(*protoDir)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(*conceptsDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var hits []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "README.md" {
			continue
		}
		path := filepath.Join(*conceptsDir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.HasPrefix(raw, []byte("---\n")) {
			continue
		}
		end := bytes.Index(raw[4:], []byte("\n---\n"))
		if end < 0 {
			continue
		}
		fm := &anchorFM{}
		if err := yaml.Unmarshal(raw[4:4+end], fm); err != nil {
			continue
		}
		// Missing/empty keys are treated identically to the explicit
		// `(none)` sentinel — both signal "this concept has no proto /
		// config / API anchor." The lint only validates the shape of
		// concepts that opt in by setting a non-empty value.
		if fm.ProtoSymbol != "" && fm.ProtoSymbol != "(none)" {
			// Expected shape: `<Name> in protocols/proto/v1/<file>.proto`
			parts := strings.SplitN(fm.ProtoSymbol, " in ", 2)
			if len(parts) != 2 {
				hits = append(hits, fmt.Sprintf("%s: proto_symbol %q does not match shape `<Name> in protocols/proto/v1/<file>.proto`", path, fm.ProtoSymbol))
			} else {
				name := strings.TrimSpace(parts[0])
				if _, ok := protoSyms[name]; !ok {
					hits = append(hits, fmt.Sprintf("%s: proto_symbol references unknown proto symbol %q (no `message %s`, `enum %s`, or `service %s` found in %s)", path, name, name, name, name, *protoDir))
				}
			}
		}
		if fm.ConfigField != "" && fm.ConfigField != "(none)" && !configFieldRE.MatchString(fm.ConfigField) {
			hits = append(hits, fmt.Sprintf("%s: config_field %q does not match shape `rimsky.yml:<dotted.path>`", path, fm.ConfigField))
		}
		if fm.APISurface != "" && fm.APISurface != "(none)" && !apiSurfaceRE.MatchString(fm.APISurface) {
			hits = append(hits, fmt.Sprintf("%s: api_surface %q does not match shape `<HTTP_VERB> /<path>`", path, fm.APISurface))
		}
	}
	if len(hits) > 0 {
		return fmt.Errorf("public-anchor validity failed:\n  - %s", strings.Join(hits, "\n  - "))
	}
	return nil
}

func collectProtoSymbols(dir string) (map[string]struct{}, error) {
	symbols := map[string]struct{}{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".proto") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range protoSymbolRE.FindAllSubmatch(raw, -1) {
			symbols[string(m[1])] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return symbols, nil
}
