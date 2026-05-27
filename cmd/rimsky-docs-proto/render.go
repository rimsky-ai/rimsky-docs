// Copyright © 2026 Fall Guy Consulting.
// Licensed under the Apache License, Version 2.0. See LICENSE.apache at the
// repo root, or http://www.apache.org/licenses/LICENSE-2.0.

// render.go — renders a single FileDescriptorProto to markdown. The
// SourceCodeInfo path encoding used to look up comments:
//   file-level comment .......... [12]   (FileDescriptorProto.syntax) is not
//                                         it; the file comment is the location
//                                         with an empty path or path [2]
//                                         (package). We use path [] / [2].
//   service i ................... [6, i]
//     method j .................. [6, i, 2, j]
//   message i ................... [4, i]
//     field j ................... [4, i, 2, j]
//   enum i ...................... [5, i]
//     enum value j .............. [5, i, 2, j]
package main

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/descriptorpb"
)

// FileDescriptorProto field numbers used in source-info paths.
const (
	fileMessageType = 4
	fileEnumType    = 5
	fileService     = 6
	filePackage     = 2
)

// DescriptorProto / ServiceDescriptorProto / EnumDescriptorProto field
// numbers for their repeated children.
const (
	msgField       = 2
	svcMethod      = 2
	enumValueField = 2
)

func renderFile(fdp *descriptorpb.FileDescriptorProto) string {
	idx := buildCommentIndex(fdp)
	var b strings.Builder

	fmt.Fprintf(&b, "%s\n\n", autogenBanner)
	fmt.Fprintf(&b, "# %s\n\n", titleize(fdp.GetName()))
	fmt.Fprintf(&b, "Source: `protocols/proto/v1/%s`\n\n", fdp.GetName())

	// File-level comment: protoc records it under the package-declaration
	// location ([2]) or an empty path.
	if c := firstNonEmpty(idx[pathKey([]int32{filePackage})], idx[""]); c != "" {
		fmt.Fprintf(&b, "%s\n\n", c)
	}

	renderServices(&b, fdp, idx)
	renderMessages(&b, fdp, idx)
	renderEnums(&b, fdp, idx)

	return b.String()
}

func renderServices(b *strings.Builder, fdp *descriptorpb.FileDescriptorProto, idx commentIndex) {
	svcs := fdp.GetService()
	if len(svcs) == 0 {
		return
	}
	fmt.Fprintf(b, "## Services\n\n")
	for i, svc := range svcs {
		if c := idx[pathKey([]int32{fileService, int32(i)})]; c != "" {
			fmt.Fprintf(b, "### %s\n\n%s\n\n", svc.GetName(), c)
		} else {
			fmt.Fprintf(b, "### %s\n\n", svc.GetName())
		}
		for j, m := range svc.GetMethod() {
			fmt.Fprintf(b, "#### %s.%s\n\n", svc.GetName(), m.GetName())
			in := typeBasename(m.GetInputType())
			out := typeBasename(m.GetOutputType())
			if m.GetClientStreaming() {
				in = "stream " + in
			}
			if m.GetServerStreaming() {
				out = "stream " + out
			}
			fmt.Fprintf(b, "- Request: `%s`\n- Response: `%s`\n\n", in, out)
			if c := idx[pathKey([]int32{fileService, int32(i), svcMethod, int32(j)})]; c != "" {
				fmt.Fprintf(b, "%s\n\n", c)
			}
		}
	}
}

func renderMessages(b *strings.Builder, fdp *descriptorpb.FileDescriptorProto, idx commentIndex) {
	msgs := fdp.GetMessageType()
	if len(msgs) == 0 {
		return
	}
	fmt.Fprintf(b, "## Messages\n\n")
	for i, msg := range msgs {
		renderMessage(b, msg, idx, []int32{fileMessageType, int32(i)}, msg.GetName())
	}
}

// renderMessage emits one message and recurses into nested messages/enums,
// prefixing nested type names with the parent for clarity.
func renderMessage(b *strings.Builder, msg *descriptorpb.DescriptorProto, idx commentIndex, path []int32, displayName string) {
	if c := idx[pathKey(path)]; c != "" {
		fmt.Fprintf(b, "### %s\n\n%s\n\n", displayName, c)
	} else {
		fmt.Fprintf(b, "### %s\n\n", displayName)
	}

	fields := msg.GetField()
	if len(fields) > 0 {
		fmt.Fprintf(b, "| Field | Type | # | Description |\n")
		fmt.Fprintf(b, "|-------|------|---|-------------|\n")
		for j, f := range fields {
			desc := inlineComment(idx[pathKey(append(append([]int32{}, path...), msgField, int32(j)))])
			fmt.Fprintf(b, "| `%s` | `%s` | %d | %s |\n",
				f.GetName(), fieldType(f), f.GetNumber(), desc)
		}
		fmt.Fprintf(b, "\n")
	}

	// Nested messages: path [..., 3, k]; nested enums: path [..., 4, k].
	for k, nested := range msg.GetNestedType() {
		if nested.GetOptions().GetMapEntry() {
			continue // synthetic map-entry messages; the map field already documents them
		}
		renderMessage(b, nested, idx,
			append(append([]int32{}, path...), 3, int32(k)),
			displayName+"."+nested.GetName())
	}
	for k, nested := range msg.GetEnumType() {
		renderEnum(b, nested, idx,
			append(append([]int32{}, path...), 4, int32(k)),
			displayName+"."+nested.GetName())
	}
}

func renderEnums(b *strings.Builder, fdp *descriptorpb.FileDescriptorProto, idx commentIndex) {
	enums := fdp.GetEnumType()
	if len(enums) == 0 {
		return
	}
	fmt.Fprintf(b, "## Enums\n\n")
	for i, e := range enums {
		renderEnum(b, e, idx, []int32{fileEnumType, int32(i)}, e.GetName())
	}
}

func renderEnum(b *strings.Builder, e *descriptorpb.EnumDescriptorProto, idx commentIndex, path []int32, displayName string) {
	if c := idx[pathKey(path)]; c != "" {
		fmt.Fprintf(b, "### %s\n\n%s\n\n", displayName, c)
	} else {
		fmt.Fprintf(b, "### %s\n\n", displayName)
	}
	fmt.Fprintf(b, "| Value | # | Description |\n")
	fmt.Fprintf(b, "|-------|---|-------------|\n")
	for j, v := range e.GetValue() {
		desc := inlineComment(idx[pathKey(append(append([]int32{}, path...), enumValueField, int32(j)))])
		fmt.Fprintf(b, "| `%s` | %d | %s |\n", v.GetName(), v.GetNumber(), desc)
	}
	fmt.Fprintf(b, "\n")
}

// fieldType renders a field's type: scalar name, or the basename of a
// message/enum type, with a repeated/optional label prefix where set.
func fieldType(f *descriptorpb.FieldDescriptorProto) string {
	t := scalarOrTypeName(f)
	switch f.GetLabel() {
	case descriptorpb.FieldDescriptorProto_LABEL_REPEATED:
		return "repeated " + t
	case descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL:
		if f.GetProto3Optional() {
			return "optional " + t
		}
	}
	return t
}

func scalarOrTypeName(f *descriptorpb.FieldDescriptorProto) string {
	switch f.GetType() {
	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE,
		descriptorpb.FieldDescriptorProto_TYPE_ENUM:
		return typeBasename(f.GetTypeName())
	}
	return scalarName(f.GetType())
}

func scalarName(t descriptorpb.FieldDescriptorProto_Type) string {
	name := t.String() // e.g. "TYPE_STRING"
	name = strings.TrimPrefix(name, "TYPE_")
	return strings.ToLower(name)
}

// typeBasename strips the leading dot and package from a fully-qualified proto
// type name (".rimsky.v1.OpenRequest" → "OpenRequest"); keeps the last segment.
func typeBasename(name string) string {
	name = strings.TrimPrefix(name, ".")
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	return name
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
