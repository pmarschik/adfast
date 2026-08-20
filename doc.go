// Package adfast converts between Markdown and the Atlassian Document
// Format (ADF) with the pivot AST (ast.Node) as the explicit currency.
// The facade is FOUR primitives, named by their non-AST end, with From*
// and To* the inverses at the AST boundary:
//
//		md text  ──FromMarkdown──▶  adfast AST  ──ToADF──▶       ADF
//		ADF      ──FromADF──────▶   adfast AST  ──ToMarkdown──▶  md text
//
//	  - FromMarkdown(md, ...Option) ast.Node — parse only (the faithful
//	    pivot AST; no ADF, no normalization).
//	  - FromADF(doc, ...Option) ast.Node — decode an ADF document.
//	  - ToADF(n, ...Option) adf.Doc — encode the AST to ADF.
//	  - ToMarkdown(n, ...Option) string — render the AST to Markdown.
//
// The common conversions are compositions: md→adf is
// ToADF(FromMarkdown(md)), adf→md is ToMarkdown(FromADF(doc)), and the
// prettier md→md formatter is ToMarkdown(FromMarkdown(md,
// WithPrettierFormat()), WithPrettierFormat()) (add WithPrintWidth(w) to
// both calls for a custom width). Pass the same options to both halves of
// a composition; each primitive reads only the subset it needs. There is
// ONE shared option type, adfast.Option. The Pipeline bundles the
// cross-cutting options once for both directions.
//
// The Markdown dialect is CommonMark + GFM plus remark-directive-style
// generic directives (via github.com/pmarschik/goldmark-directive) for ADF
// features without native Markdown syntax — panels, expands, media, smart
// links, statuses, mentions, colors, sub/sup, underline, JQL datasource
// tables, table column widths — and remark-extended-table cell spans.
//
// Rendering matches remark-stringify byte-for-byte on the covered corpus
// (escaping rules, list alternation, wrapping, character references), and
// FromMarkdown→ToMarkdown is round-trip idempotent (continuously fuzzed).
//
// This package is the facade — the four From*/To* primitives, the
// Pipeline, and the shared option set — composing the layered
// subpackages:
//
//   - adf: the typed ADF document model — one Go type per known
//     node/mark kind, RawNode/RawMark for unknown kinds, Extra maps for
//     unmodeled attributes — with the lossless JSON codec and tree
//     helpers
//   - ast: the pivot Markdown AST both conversion directions share
//   - extension: the public AST extension contract — custom node kinds
//     covering all four pipeline paths (parse, render, encode, decode)
//   - dialect: the known directive dialect as typed AST nodes on the
//     extension contract, wired as the default registration set
//   - markdown: the goldmark parser assembly (dialect, directives) and
//     the remark-compatible renderer
//   - convert: the AST ⇄ ADF transforms, the shared Normalize
//     canonicalization pass, and their parameter types (SmartLinks,
//     LinkResolver, MediaAsset, resolvers, Diagnostic)
//   - debug: human-readable dumps of both trees for debugging (output
//     format not covered by compatibility guarantees)
//   - jira: a separate submodule bundling the Jira conventions
//   - confluence: a separate submodule bundling the Confluence
//     conventions (page smart links, code block macro languages)
//   - skill: a separate submodule shipping the markdown dialect as an
//     embeddable agent skill (SKILL.md + references)
package adfast
