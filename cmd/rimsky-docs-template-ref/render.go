// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

// render.go — renders a specModel to markdown, plus the small AST/string
// helpers (tag parsing, readable type rendering, anchor linking) the model
// build and the render share. Stdlib only.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"reflect"
	"sort"
	"strings"
)

// structOrder is the curated top-down rendering order for the schema structs:
// the template root first, then the node shape and each nested directive type,
// then defaults, then graph/publisher/policy types. Any exported struct not
// listed here is appended afterward in source order, so the reference stays
// complete even if the spec package gains a struct this list does not name.
var structOrder = []string{
	// Template root.
	"TemplateSpec",
	// Graph container (the post-2026-05-15 nested form).
	"GraphSpec",
	// The node shape and its nested directive types.
	"TemplateNodeDef",
	"NodeStoreRef",
	"NodeLockRef",
	"HoldsBinding",
	"NodeAttributesDef",
	"InheritEntry",
	"SubscriptionEntry",
	"FanOutSpec",
	"AggregationPolicy",
	"ErrorTypePolicy",
	"PolicyAction",
	// Per-instance publisher subscriptions.
	"PublisherSpec",
	// Template-author attribute defaults.
	"TemplateDefaults",
	"TemplateAttributeDefaults",
}

// renderReference builds the full markdown schema reference from the model.
func renderReference(m *specModel) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", autogenBanner)
	fmt.Fprintf(&b, "# rimsky template schema (`rimsky.yml`) reference\n\n")
	fmt.Fprintf(&b, "This is the complete, mechanical reference for the rimsky template "+
		"schema — the shape of a `rimsky.yml`. It is generated from the spec "+
		"row-types under `lib/foundation/spec/` (the persistable data the "+
		"template canonicalizer parses out of a template). Every exported "+
		"struct is documented as a field table (YAML key, Go type, description); "+
		"every enum is documented as a value table. Read it top-down: the "+
		"template root, then the node shape and each nested directive, then "+
		"defaults, then the enums.\n\n")
	if m.PackageDoc != "" {
		fmt.Fprintf(&b, "> **Package note.** %s\n\n", collapseDoc(m.PackageDoc))
	}

	var schema, runtime []string
	for _, name := range orderedStructNames(m) {
		if m.Structs[name].Tagged {
			schema = append(schema, name)
		} else {
			runtime = append(runtime, name)
		}
	}

	fmt.Fprintf(&b, "## Structs\n\n")
	for _, name := range schema {
		renderStruct(&b, m, m.Structs[name])
	}

	if len(runtime) > 0 {
		fmt.Fprintf(&b, "## Runtime-only types\n\n")
		fmt.Fprintf(&b, "These exported spec types carry no `yaml:`/`json:` tags and are **not "+
			"part of the template YAML**. They are runtime decision outputs and "+
			"DB-row projections, listed here for completeness; the column shows the "+
			"Go field name, not a YAML key.\n\n")
		for _, name := range runtime {
			renderStruct(&b, m, m.Structs[name])
		}
	}

	renderEnums(&b, m)
	return b.String()
}

// orderedStructNames returns the struct names in curated order, with any
// unlisted exported struct appended in source order.
func orderedStructNames(m *specModel) []string {
	listed := map[string]bool{}
	var out []string
	for _, name := range structOrder {
		if _, ok := m.Structs[name]; ok {
			out = append(out, name)
			listed[name] = true
		}
	}
	for _, name := range m.StructList {
		if !listed[name] {
			out = append(out, name)
		}
	}
	return out
}

func renderStruct(b *strings.Builder, m *specModel, s *specStruct) {
	fmt.Fprintf(b, "### %s\n\n", s.Name)
	if s.Doc != "" {
		fmt.Fprintf(b, "%s\n\n", s.Doc)
	}
	if len(s.Fields) == 0 {
		fmt.Fprintf(b, "_No exported fields._\n\n")
		return
	}
	keyHeader := "YAML key"
	if !s.Tagged {
		keyHeader = "Go field"
	}
	fmt.Fprintf(b, "| %s | Go type | Description |\n", keyHeader)
	fmt.Fprintf(b, "|----------|---------|-------------|\n")
	for _, f := range s.Fields {
		key := f.YAMLKey
		if !s.Tagged {
			key = f.GoField
		}
		var notes []string
		if f.Inline {
			notes = append(notes, "inline")
		}
		if f.OmitEmpty {
			notes = append(notes, "optional")
		}
		if len(notes) > 0 {
			key = fmt.Sprintf("`%s`<br/>_(%s)_", key, strings.Join(notes, ", "))
		} else {
			key = fmt.Sprintf("`%s`", key)
		}
		fmt.Fprintf(b, "| %s | %s | %s |\n", key, linkType(m, f.GoType), inlineDoc(f.Doc))
	}
	fmt.Fprintf(b, "\n")
}

func renderEnums(b *strings.Builder, m *specModel) {
	if len(m.Enums) == 0 {
		return
	}
	fmt.Fprintf(b, "## Enums\n\n")
	for _, e := range m.Enums {
		if e.Typed {
			fmt.Fprintf(b, "### %s\n\n", e.Name)
			fmt.Fprintf(b, "_Named string type (`type %s string`)._\n\n", e.Name)
		} else {
			fmt.Fprintf(b, "### %s\n\n", e.Name)
			fmt.Fprintf(b, "_Untyped string constant group._\n\n")
		}
		if e.Doc != "" {
			fmt.Fprintf(b, "%s\n\n", e.Doc)
		}
		fmt.Fprintf(b, "| Constant | Value | Description |\n")
		fmt.Fprintf(b, "|----------|-------|-------------|\n")
		for _, v := range e.Values {
			fmt.Fprintf(b, "| `%s` | `%s` | %s |\n", v.Name, v.Value, inlineDoc(v.Doc))
		}
		fmt.Fprintf(b, "\n")
	}
}

// linkType wraps a rendered Go type in a markdown anchor link when it refers to
// a struct or enum documented elsewhere in this reference. Slice/map/pointer
// wrappers around a known type are linked on the inner element.
func linkType(m *specModel, goType string) string {
	base := strings.TrimPrefix(goType, "*")
	base = strings.TrimPrefix(base, "[]")
	if anchor, ok := typeAnchor(m, base); ok {
		// Render the full type in code font but link the whole cell to the
		// element type's section.
		return fmt.Sprintf("[`%s`](#%s)", goType, anchor)
	}
	if anchor, ok := mapValueAnchor(m, base); ok {
		return fmt.Sprintf("[`%s`](#%s)", goType, anchor)
	}
	return "`" + goType + "`"
}

// typeAnchor returns the GitHub-style markdown anchor for a known struct/enum
// type name.
func typeAnchor(m *specModel, name string) (string, bool) {
	if m.structNames[name] || m.enumNames[name] {
		return githubAnchor(name), true
	}
	return "", false
}

// mapValueAnchor links a `map[K]V` type on V when V is a known type.
func mapValueAnchor(m *specModel, base string) (string, bool) {
	if !strings.HasPrefix(base, "map[") {
		return "", false
	}
	i := strings.LastIndex(base, "]")
	if i < 0 {
		return "", false
	}
	return typeAnchor(m, base[i+1:])
}

// githubAnchor mirrors GitHub's heading-anchor slug rule for our heading text
// (the bare type name): lowercased, non-alphanumerics dropped.
func githubAnchor(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-':
			b.WriteRune('-')
		}
	}
	return b.String()
}

// renderType renders an AST type expression as readable Go source on one line
// (e.g. `[]NodeStoreRef`, `map[string]string`, `*FanOutSpec`).
func renderType(fset *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	cfg := printer.Config{Mode: printer.UseSpaces, Tabwidth: 4}
	if err := cfg.Fprint(&buf, fset, expr); err != nil {
		return ""
	}
	// Qualified types from other packages (json.RawMessage, signal.Signal)
	// render as-is; that is the readable form a reader expects.
	return strings.Join(strings.Fields(buf.String()), " ")
}

// inlineDoc collapses a multi-line doc comment into a single table-cell-safe
// line (newlines → spaces, pipes escaped).
func inlineDoc(s string) string {
	s = collapseDoc(s)
	return strings.ReplaceAll(s, "|", "\\|")
}

// collapseDoc collapses internal whitespace runs (including newlines) to single
// spaces and trims the result.
func collapseDoc(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// --- struct-tag helpers -----------------------------------------------------

// tagValue returns the (name, options) for a struct tag key, e.g. for
// `yaml:"foo,omitempty"` it returns ("foo", ["omitempty"]). The name may be
// empty (e.g. `yaml:",inline"`).
func tagValue(tag *ast.BasicLit, key string) (string, []string) {
	raw := tagString(tag)
	if raw == "" {
		return "", nil
	}
	val, ok := reflect.StructTag(raw).Lookup(key)
	if !ok {
		return "", nil
	}
	parts := strings.Split(val, ",")
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}

// hasTag reports whether the tag declares the given key at all (even with an
// empty name, as in `yaml:",inline"`).
func hasTag(tag *ast.BasicLit, key string) bool {
	raw := tagString(tag)
	if raw == "" {
		return false
	}
	_, ok := reflect.StructTag(raw).Lookup(key)
	return ok
}

// tagString unquotes a struct-tag literal (the backtick-delimited tag text).
func tagString(tag *ast.BasicLit) string {
	if tag == nil {
		return ""
	}
	return strings.Trim(tag.Value, "`")
}

// --- untyped const group titling --------------------------------------------

// groupTitle derives a stable, human-readable title for an untyped const
// group from the longest shared prefix of its constant names (e.g.
// NodeStateFresh/NodeStateStale → "NodeState* (untyped string set)"). Falls
// back to the first const name when no shared prefix is found.
func groupTitle(vals []enumValue) string {
	if len(vals) == 0 {
		return "constants"
	}
	names := make([]string, len(vals))
	for i, v := range vals {
		names[i] = v.Name
	}
	sort.Strings(names)
	prefix := commonCamelPrefix(names)
	if prefix == "" {
		return names[0]
	}
	return prefix + "*"
}

// commonCamelPrefix returns the longest shared prefix of names truncated to a
// CamelCase word boundary, so "NodeStateFresh"/"NodeStateStale" yields
// "NodeState" (each name continues with a fresh uppercase word) rather than
// ending mid-word. Returns "" when names share no leading word.
func commonCamelPrefix(names []string) string {
	if len(names) < 2 {
		return ""
	}
	prefix := names[0]
	for _, n := range names[1:] {
		prefix = sharedPrefix(prefix, n)
		if prefix == "" {
			return ""
		}
	}
	// The shared prefix is a clean word boundary if, in every name, the rune
	// immediately after the prefix starts a new CamelCase word (an uppercase
	// letter) or the name ends exactly at the prefix. Otherwise the prefix cuts
	// through a word, so trim back to the last uppercase boundary.
	if isWordBoundary(prefix, names) {
		return prefix
	}
	runes := []rune(prefix)
	for i := len(runes) - 1; i > 0; i-- {
		if runes[i] >= 'A' && runes[i] <= 'Z' {
			return string(runes[:i])
		}
	}
	return prefix
}

// isWordBoundary reports whether prefix ends on a CamelCase word boundary
// across all names: the next rune after the prefix is uppercase, or the name
// ends at the prefix.
func isWordBoundary(prefix string, names []string) bool {
	for _, n := range names {
		if len(n) == len(prefix) {
			continue
		}
		next := n[len(prefix)]
		if next < 'A' || next > 'Z' {
			return false
		}
	}
	return true
}

func sharedPrefix(a, b string) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return a[:i]
}
