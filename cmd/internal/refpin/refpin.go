// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

// Package refpin resolves the rimsky release the corpus documents — the
// `reconciledAgainst` field of the plugin manifest — and renders the version
// banner every generated reference page carries.
//
// The banner names a concrete release tag. Hand-written prose must never do
// that (a hard-coded tag silently drifts; prose defers to the manifest
// instead), but generated pages are regenerated from source on every
// reconcile and byte-compared by the reference-parity lint, so a concrete tag
// here cannot drift without the gate going red.
//
// The pin is read from plugin.json only — not $RIMSKY_TAG — so regenerating
// and lint-checking produce identical bytes regardless of environment. During
// a /build-docs run the skill-packaging surface stamps reconciledAgainst
// (step 1) before mechanical generation runs (step 2), so the manifest is
// already current when the generators read it.
package refpin

import (
	"encoding/json"
	"fmt"
	"os"
)

// Resolve reads reconciledAgainst from the plugin manifest at pluginPath.
// An unreadable manifest or an empty field is an error: a generated reference
// without a version pin is exactly the silent skew the banner exists to kill.
func Resolve(pluginPath string) (string, error) {
	data, err := os.ReadFile(pluginPath)
	if err != nil {
		return "", fmt.Errorf("read plugin manifest %s: %w", pluginPath, err)
	}
	var p struct {
		ReconciledAgainst string `json:"reconciledAgainst"`
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return "", fmt.Errorf("parse plugin manifest %s: %w", pluginPath, err)
	}
	if p.ReconciledAgainst == "" {
		return "", fmt.Errorf("plugin manifest %s has no reconciledAgainst", pluginPath)
	}
	return p.ReconciledAgainst, nil
}

// Banner renders the one-line version banner for a generated reference page.
// The exact string is shared with the reference-parity lint's banner check;
// change it here and the lint follows.
func Banner(version string) string {
	return fmt.Sprintf("<!-- This reference reflects rimsky %s — the `reconciledAgainst` release in .claude-plugin/plugin.json. -->", version)
}
