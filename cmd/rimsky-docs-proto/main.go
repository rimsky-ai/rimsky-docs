// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

// main.go — rimsky-docs-proto. Generates a markdown wire reference for each
// protobuf file in rimsky's `lib/protocols/proto/v1/`, capturing services,
// methods, messages, fields, and enums *with* their doc comments.
//
// protobuf doc comments only survive in source info, which compiled
// descriptors drop. So we shell out to `protoc --include_source_info` to emit
// a FileDescriptorSet, unmarshal it, and reconstruct comments by mapping
// SourceCodeInfo location paths back to declarations.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rimsky-ai/rimsky-docs/cmd/internal/refpin"
)

func main() {
	rimskyRepo := os.Getenv("RIMSKY_REPO")
	if rimskyRepo == "" {
		fmt.Fprintln(os.Stderr, "rimsky-docs-proto: RIMSKY_REPO is unset.")
		fmt.Fprintln(os.Stderr, "Set RIMSKY_REPO to a local rimsky checkout path, e.g.:")
		fmt.Fprintln(os.Stderr, "  RIMSKY_REPO=$(pwd)/../rimsky go run ./cmd/rimsky-docs-proto")
		fmt.Fprintln(os.Stderr, "Required by the docs reconciliation gate to cross-check generated content against rimsky source.")
		os.Exit(2)
	}

	defaultProtoDir := rimskyRepo + "/lib/protocols/proto/v1"
	protoDir := flag.String("proto-dir", defaultProtoDir, "directory of .proto files (defaults to ${RIMSKY_REPO}/lib/protocols/proto/v1)")
	outDir := flag.String("out-dir", "../rimsky/skills/rimsky/docs/protocols", "directory to write reference markdown into (relative to cmd/ cwd)")
	pluginJSON := flag.String("plugin", "../rimsky/.claude-plugin/plugin.json", "plugin.json carrying the reconciledAgainst version banner pin")
	check := flag.Bool("check", false, "verify existing output matches regenerated content; exit non-zero on diff")
	flag.Parse()

	version, err := refpin.Resolve(*pluginJSON)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := run(*protoDir, *outDir, version, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
