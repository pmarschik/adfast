// Package markdown owns the Markdown text edge of the pipeline: the
// goldmark parser assembly (GFM plus the remark-directive-compatible
// dialect, see NewParser and dialect.go), the guarded Parse entry that
// lifts a parsed source into the pivot AST (ast.Node), and the
// remark-compatible Render that serializes an AST tree back to Markdown
// text.
//
// The audience is the root adfast facade and advanced consumers that need
// direct parser access (e.g. syntax tooling over the generic directive
// nodes); most users should stay on the root facade. The surface is
// stable alongside the root package.
//
// The goldmark tree deliberately carries only GENERIC directive nodes:
// directive names bind to typed kinds in exactly one place,
// dialect.Registrations(), which Parse applies as promotions on the
// lifted AST. A second, goldmark-level typed-node registry would
// duplicate that name table.
package markdown

import (
	"bytes"
	"fmt"
	"maps"
	"strings"

	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/dialect"
	"github.com/pmarschik/adfast/extension"
)

// PreservedEscapes are the backslash escapes the prettier formatter keeps
// as literal source bytes. The single faithful parse captures them
// undecoded on ast.Text.Raw (the escape provenance) while Value stays
// fully decoded, so the formatter can re-emit them byte-for-byte without
// polluting the semantic value.
//
// It is the CommonMark escapable set (isEscapableASCIIPunct) MINUS '_',
// '*', '`' and '#'. The reference formatter preserves an authored escape
// for every other punctuation byte — measured over all 32 of them, it keeps
// 31 and drops only '_', whose escape it recomputes from its word-boundary
// rule (an intraword "a_b\_c d" comes back "a_b_c d"). '*' and '`' are left
// out for a different reason: the render escapes both unconditionally
// (escapesInlineMarker), so decoding them and letting that rule write the
// backslash back reproduces the reference byte-for-byte with no provenance
// to carry.
//
// ADDING '`' HERE IS A MEASURED NO-OP, and the reason is worth stating
// because the byte looks like the lever for the one row where this leg and
// prettier 3.8.1 disagree. Provenance can only carry a backslash the author
// WROTE. On "a\`b" the escape already survives byte for byte without it —
// escapesInlineMarker re-derives the same backslash — and the divergent row
// is the opposite spelling, a BARE backtick: "a`b" formats to "a\`b" here and
// prettier 3.8.1 leaves it alone. That source has no backslash run to record,
// so no widening of this set can reach it; the escape decision is
// escapesInlineMarker's alone. Measured 2026-09-09 by adding '`' and
// re-rendering 13,986 documents over {a ` \ * _ space} in nine block and
// inline contexts on both legs: zero byte differences, whole suite green.
// (Dropping '\' from the set as a control moved 1,648 of them, so the corpus
// could see a change.) '*' does not have that divergence at all: prettier
// 3.8.1 escapes a bare "a*b" to "a\*b" just as this leg does.
//
// Making the bare-backtick row match would mean gating escapesInlineMarker on
// provenance, which is a different and much larger change: the unconditional
// escape is what keeps an ADF-origin text value containing a backtick from
// re-parsing as a code span, and prettier only gets away without it by
// echoing source it never has to re-derive.
//
// '#' IS LEFT OUT UNDER PROTEST, and the reason is a rule that runs after
// the escaper rather than a property of the character. An ATX heading's
// trailing '#' would re-parse as a closing sequence, so renderHeading
// rewrites the LAST byte of the finished inline string into "\#". That
// rewrite reads no parity, so an escape already standing there comes back
// doubled: "## ends \#" formatted to "## ends \\#", whose value gained a
// backslash and lost the '#' escape, and a third backslash appeared on the
// next pass. Dropping '#' here costs byte parity only — never a value. A
// '#' escape is defensive in every position the render can produce (a
// line-leading run is re-escaped by escapeParagraphLeadingMarker and a
// heading trailer by renderHeading), so decoding it and letting those rules
// write it back keeps the document's meaning. Guarding the renderHeading
// rewrite with the parity test heading_anchor.go already carries
// (escapedAt, which escapeHeadingAnchorTail uses for exactly this reason)
// would let '#' rejoin the set.
//
// '\' ITSELF IS IN THE SET, and that is what makes the provenance total
// rather than a guess. With it, Raw is the verbatim source escape form: a
// backslash run is exactly as the author wrote it, so "\~" (an authored
// escape) and "\\~" (an authored LITERAL backslash before a tilde) stay
// distinct all the way to the render, which reads the two apart by the
// parity of the backslash run (see mdRenderer.sourceEscaped). Keeping any
// byte in this set WITHOUT '\' would trade one loss for the other: the
// escape survives and the literal backslash is deleted instead.
//
// Text that carries no Raw — built from ADF or by hand — is lifted into the
// same verbatim form by ast.Text.Rendered, so the render sees one
// convention whatever the tree's origin.
const PreservedEscapes = "!\"$%&'()+,-./:;<=>?@[\\]^{|}~"

type parseConfig struct {
	recoverNotice     func()
	depthNotice       func()
	spanNotice        func(marker string, row, col int)
	extensions        []extension.Registration
	genericDirectives bool
}

// ParseOption configures Parse.
type ParseOption func(*parseConfig)

// WithRecoverNotice registers fn, called when the guarded parse recovered
// from a goldmark parser panic by re-parsing a normalized source (see
// parseGuarded); callers surface this as a diagnostic.
func WithRecoverNotice(fn func()) ParseOption {
	return func(c *parseConfig) { c.recoverNotice = fn }
}

// WithDepthExceededNotice registers fn, called (once per parse) when
// the source nested deeper than the lift's recursion cap and deeper
// content was truncated; callers surface this as a diagnostic. Without
// the cap, adversarial nesting (e.g. thousands of blockquote markers)
// would overflow the stack, which Go cannot recover from.
func WithDepthExceededNotice(fn func()) ParseOption {
	return func(c *parseConfig) { c.depthNotice = fn }
}

// WithTableSpanNotice registers fn, called for every table span marker
// (a cell containing only ">" or "^") sitting in a position where its
// merge cannot apply — a ">" with no content cell to its right in the
// row, or a "^" with no spanning cell above its visual column. The
// marker is kept as literal cell text (the historical fallback); row
// and col are 1-based within the table (row 1 is the header row).
// Callers surface this as a diagnostic.
func WithTableSpanNotice(fn func(marker string, row, col int)) ParseOption {
	return func(c *parseConfig) { c.spanNotice = fn }
}

// WithExtensions registers additional AST extension kinds (see the
// extension package) on top of the default dialect set: after the
// generic lift, directive nodes whose names a registration owns are
// promoted into the typed extension nodes. A user registration's name
// overrides the dialect's promotion of the same name; duplicate names
// WITHIN the user-supplied set panic at parse time (see
// extension.ValidateSet), as does an incomplete bundle
// (Registration.Validate).
func WithExtensions(regs ...extension.Registration) ParseOption {
	return func(c *parseConfig) { c.extensions = append(c.extensions, regs...) }
}

// WithGenericDirectives skips the typed-node promotion step entirely, so
// EVERY directive — a dialect name, a name a WithExtensions registration
// owns, and an unknown name alike — stays an
// ast.ContainerDirective/LeafDirective/TextDirective carrying its own Name.
//
// It exists because promotion is NOT invertible: several directive names
// share one typed kind (:::info, :::note, :::warning, :::success and
// :::error all promote to dialect.Panel; :::center and :::end both to
// dialect.Align), so a walk over a promoted tree cannot recover the name
// the author wrote. A consumer that needs the literal directive name —
// syntax tooling, or a plain-text projection that must not silently drop
// an accidental intraword colon like "deploy:status" — parses with this
// option and reads Name off the generic node.
//
// The typed kinds are what the ADF conversions consume, so a tree parsed
// this way is for INSPECTION only: do not feed it to ToADF or to the
// prettier formatter, which both expect the promoted form.
func WithGenericDirectives() ParseOption {
	return func(c *parseConfig) { c.genericDirectives = true }
}

// Parse converts Markdown source to the pivot AST. The conversion is
// text→AST: goldmark parses the source (guarded against known goldmark
// panics), goldmarkToAst lifts the parse tree into the
// source-independent Markdown AST, and the registered extension kinds
// (the dialect set by default, plus WithExtensions) promote their
// directive names into typed nodes. The source is expected to use \n
// line endings.
//
// Parse is NOT FRONTMATTER AWARE and never produces an ast.Frontmatter
// node. Leading document metadata is a caller-defined convention, so the
// split lives one layer up, in the root adfast facade's
// FrontmatterProvider (see adfast.WithFrontmatterProvider), which also
// normalizes CR/CRLF and peels a byte order mark before this parse sees
// the source. Handed a document that opens a YAML frontmatter fence,
// Parse applies plain CommonMark and reads the metadata as content:
// "---\nstatus: Open\n---\nBody.\n" lifts to a thematicBreak, a setext
// heading ("status: Open") and a paragraph, so Render(Parse(src))
// rewrites the metadata block as prose instead of reproducing it.
//
// For Markdown→Markdown over documents that may carry metadata, use the
// facade pair adfast.ToMarkdown(adfast.FromMarkdown(md)) instead: the
// same two-call shape with no ADF vocabulary, and plain ToMarkdown is a
// pure projection that calls this Render with the same defaults, so the
// only differences are the metadata split and the byte-order-mark and
// line-ending normalization. adfast.Pipeline.Format wires the same two
// halves for the prettier md→md mode.
func Parse(source []byte, opts ...ParseOption) ast.Node {
	cfg := parseConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	if err := extension.ValidateSet(cfg.extensions); err != nil {
		panic(err)
	}
	tree, src := parseGuarded(NewParser(), source)
	if len(src) != len(source) && cfg.recoverNotice != nil {
		cfg.recoverNotice()
	}
	root := goldmarkToAst(tree, src, cfg.depthNotice, cfg.spanNotice)
	if cfg.genericDirectives {
		// The caller reads directive names off the generic nodes; skipping
		// promotion keeps every name readable (see WithGenericDirectives).
		return root
	}
	// Dialect first, user registrations after: promotion is last-wins per
	// name, so user registrations override the dialect (the decode-side
	// dispatch achieves the same by trying user hooks first).
	promoteExtensions(root, append(dialect.Registrations(), cfg.extensions...))
	return root
}

// promotionIndex is the per-parse lookup of directive-name promotions.
type promotionIndex struct {
	containers map[string]func(*ast.ContainerDirective) extension.Node
	leaves     map[string]func(*ast.LeafDirective) extension.Node
	texts      map[string]func(*ast.TextDirective) extension.Node
}

// promoteExtensions replaces generic directive nodes whose names a
// registration owns with the constructed typed extension nodes,
// children-first so nested directives are already promoted when a
// constructor runs. Promotion happens AFTER the full generic lift
// (including URL relinkification), so remark's text-level behaviors keep
// operating on the generic tree.
func promoteExtensions(root ast.Node, regs []extension.Registration) {
	idx := promotionIndex{
		containers: map[string]func(*ast.ContainerDirective) extension.Node{},
		leaves:     map[string]func(*ast.LeafDirective) extension.Node{},
		texts:      map[string]func(*ast.TextDirective) extension.Node{},
	}
	for _, reg := range regs {
		if err := reg.Validate(); err != nil {
			panic(err)
		}
		maps.Copy(idx.containers, reg.Containers)
		maps.Copy(idx.leaves, reg.Leaves)
		maps.Copy(idx.texts, reg.Texts)
	}
	promoteChildren(root, idx)
}

// promoteChildren promotes n's subtree in place.
func promoteChildren(n ast.Node, idx promotionIndex) {
	kids := ast.Children(n)
	for i := range kids {
		kids[i] = promoteNode(kids[i], idx)
	}
	ast.SetChildren(n, kids)
}

// promoteNode returns n's replacement: the typed extension node when a
// registration owns the directive name, n itself otherwise.
func promoteNode(n ast.Node, idx promotionIndex) ast.Node {
	promoteChildren(n, idx)
	switch d := n.(type) {
	case *ast.ContainerDirective:
		if ctor := idx.containers[d.Name]; ctor != nil {
			return ctor(d)
		}
	case *ast.LeafDirective:
		if ctor := idx.leaves[d.Name]; ctor != nil {
			return ctor(d)
		}
	case *ast.TextDirective:
		if ctor := idx.texts[d.Name]; ctor != nil {
			return ctor(d)
		}
	}
	return n
}

// parseGuarded parses markdown, recovering from goldmark parser panics.
// goldmark ≤1.8.4 crashes on some tab-indented fence-trigger lines inside
// list items ("*\n  \t\x60": BlockOffset returns -1 and fcode_block indexes
// with it), and user-authored files must never crash the CLI. On panic the
// source is retried with tabs expanded to spaces, then with its backticks
// escaped; as a last resort the document is parsed EMPTY, which loses the
// content but keeps the caller running. The record said "plain text lines"
// here, which is not what the last rung does.
//
// The rungs are exercised with an injected panic, because no source is known
// to make the pinned goldmark take any of them — see parse_guarded_test.go.
func parseGuarded(p parser.Parser, src []byte) (node gast.Node, out []byte) {
	tree, err := tryParse(p, src)
	if err == nil {
		return tree, src
	}
	expanded := bytes.ReplaceAll(src, []byte("\t"), []byte("    "))
	if tree, err = tryParse(p, expanded); err == nil {
		return tree, expanded
	}
	escaped := []byte(strings.ReplaceAll(string(src), "\x60", "\\\x60"))
	if tree, err = tryParse(p, escaped); err == nil {
		return tree, escaped
	}
	empty := []byte{}
	if tree, err = tryParse(p, empty); err != nil {
		// Unreachable: goldmark's panic paths need actual content, so an
		// empty source always parses; fall through to a direct parse.
		tree = p.Parse(text.NewReader(empty))
	}
	return tree, empty
}

func tryParse(p parser.Parser, src []byte) (node gast.Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("goldmark parse panic: %v", r)
		}
	}()
	return p.Parse(text.NewReader(src)), nil
}
