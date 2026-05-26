module github.com/fallguyconsulting/rimsky-docs/examples

go 1.25.0

require (
	github.com/fallguyconsulting/rimsky/protocols v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.80.0
)

require (
	go.opentelemetry.io/otel/metric v1.41.0 // indirect
	go.opentelemetry.io/otel/trace v1.41.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260120221211-b8f7ae30c516 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

// Local-dev replace directive so sibling rimsky checkouts resolve without
// a tagged release. CI may strip this (or set the replaced version) when
// pinning to a published rimsky tag. Only `protocols/` is required —
// the examples build directly against the wire types and do not import
// sdk/go.
replace github.com/fallguyconsulting/rimsky/protocols => ../../rimsky/protocols
