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

// PreservedEscapes are the backslash escapes prettier keeps as literal text
// (its parser predates CommonMark's escapable set). The single faithful
// parse captures them undecoded on ast.Text.Raw (the escape provenance)
// while Value stays fully decoded, so the prettier formatter can re-emit
// them byte-for-byte (\~, \:, \-) without polluting the semantic value.
const PreservedEscapes = "~:-+"

type parseConfig struct {
	recoverNotice func()
	depthNotice   func()
	spanNotice    func(marker string, row, col int)
	extensions    []extension.Registration
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

// Parse converts Markdown source to the pivot AST. The conversion is
// text→AST: goldmark parses the source (guarded against known goldmark
// panics), goldmarkToAst lifts the parse tree into the
// source-independent Markdown AST, and the registered extension kinds
// (the dialect set by default, plus WithExtensions) promote their
// directive names into typed nodes. The source is expected to use \n
// line endings (the adfast facade normalizes CR/CRLF before splitting
// frontmatter).
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
// source is retried with tabs expanded to spaces; as a last resort the
// document parses as plain text lines.
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
