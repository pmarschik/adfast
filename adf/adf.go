// Package adf defines the typed Atlassian Document Format (ADF) document
// model — Doc, the Node and Mark interfaces with one Go type per known
// node/mark kind, and the JSON codec — together with the tree helpers
// every stage that touches ADF trees shares.
//
// # Typed with a lossless escape hatch
//
// The model is typed by design: every node kind the pipeline knows has a
// dedicated Go type (Paragraph, Heading, Media, …) whose documented
// attributes are struct fields, so converters dispatch on Go types and
// read fields instead of probing attribute maps. Unknown node kinds
// decode to RawNode (and unknown mark kinds to RawMark), which keep the
// generic type/attrs/marks/text/content shape verbatim.
//
// Losslessness is a codec invariant: DecodeDoc → json.Marshal reproduces
// the input document semantically. Every typed node and mark carries an
// Extra map holding the attributes its typed fields do not model —
// Atlassian extends nodes over time — and encoding merges Extra back
// (Extra entries win over typed fields). Known attributes whose decoded
// value cannot be represented faithfully by the typed field alone (a
// zero value where encoding only emits non-zero fields, or a non-integral
// number for an int field) are also kept in Extra, so re-encoding stays
// exact while converters keep reading the typed field.
//
// Decoding can report what it could not type: pass DecodeOptions with a
// Diagnostics sink to receive "unknown-node", "unknown-attr", and
// "unknown-mark" diagnostics.
//
// # Synthetic kinds
//
// A few constructs are internal to the markdown conversion and never
// appear in wire documents sent to Jira: ColwidthsHint is the
// placeholder the ::colwidths directive encodes to before the convert
// package resolves it onto the following table, the "tight"
// list-tightness attribute (a typed field on the list kinds) records
// source looseness for WithPreserveListTightness, the heading "anchor"
// attribute carries a markdown {#id}, and the table "align" attribute
// carries a GFM table's column alignment. IsWireSafe and StripSynthetic
// are the guard and the cleanup for all of them.
//
// Builders/constructors beyond plain struct literals can come later; the
// struct literals double as the documentation of each kind's shape.
//
// The audience is document-transform authors (e.g. the jira submodule's
// doc rewrites) and anyone walking or building ADF trees directly —
// Walk and Transform are the tree-traversal primitives, IsWireSafe and
// StripSynthetic the wire-submission guards. The surface here is stable
// alongside the root package.
package adf

// Doc represents a top-level ADF document.
type Doc struct {
	Type    string `json:"type"`
	Content []Node `json:"content,omitempty"`
	Version int    `json:"version"`
}

// Node is a single node in an ADF document tree: one Go type per known
// node kind plus RawNode for unknown kinds. The interface is sealed —
// the codec and the converters enumerate the implementations.
type Node interface {
	// Kind returns the ADF node type string ("paragraph", "text", …);
	// for RawNode it is the verbatim unknown type.
	Kind() string
	adfNode()
}

// Mark is an inline mark (bold, italic, link, …) on a text node: one Go
// type per known mark kind plus RawMark for unknown kinds. The interface
// is sealed like Node.
type Mark interface {
	// Kind returns the ADF mark type string ("strong", "link", …).
	Kind() string
	adfMark()
}

// Diagnostic reports a non-fatal issue: input that was accepted but
// could not be fully honored or fully typed (dropped constructs,
// recovered parses, unknown node/mark kinds, unmodeled attributes).
// Code is a stable identifier suitable for lint rules; the codes this
// package emits are the Code* constants below, and the convert package
// re-exports them alongside the conversion-side codes.
type Diagnostic struct {
	Code    string
	Message string
}

// Diagnostic codes emitted by the ADF decode codec (see DecodeOptions).
const (
	// CodeUnknownNode reports an ADF node type the typed model does not
	// know; the node is kept losslessly as a RawNode.
	CodeUnknownNode = "unknown-node"
	// CodeUnknownMark reports an ADF mark type the typed model does not
	// know; the mark is kept losslessly as a RawMark.
	CodeUnknownMark = "unknown-mark"
	// CodeUnknownAttr reports an attribute a known kind's typed fields do
	// not model; the value is kept losslessly in the node's Extra map.
	CodeUnknownAttr = "unknown-attr"
	// CodeDepthExceeded reports that a document nested deeper than the
	// decoder's recursion cap; content below the cap is truncated.
	CodeDepthExceeded = "depth-exceeded"
)
