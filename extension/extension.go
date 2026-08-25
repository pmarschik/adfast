// Package extension defines the public AST extension contract: how a
// consumer adds custom node kinds that the core pipeline treats as
// first-class citizens on all four paths — markdown parse (md→ast),
// markdown render (ast→md), ADF encode (ast→adf), and ADF decode
// (adf→ast). Capability fragments (render-only or encode-only kinds) are
// deliberately impossible: a kind enters the pipeline by implementing
// Node (render + encode as methods) and shipping a Registration (parse +
// decode as factory hooks), and Registration.Validate rejects incomplete
// bundles at registration time.
//
// The dialect package is the dogfood: every known dialect directive
// (panels, expand, media, smart-link cards, JQL datasource tables,
// column widths, mentions, statuses, and the inline mark directives) is
// implemented on this contract; the generic directive kinds in ast
// remain only as the fallback for unknown directives, which degrade
// exactly like remark.
//
// # Conflict policy
//
// User-supplied registrations override the default dialect set in BOTH
// directions: a directive name registered by the user wins over the
// dialect's parse promotion, and user decode hooks (DecodeBlock,
// DecodeBlockList, DecodeInline, DecodeTextMark) are tried BEFORE the
// dialect's. Within one user-supplied set, however, registering the same
// directive name twice is a configuration bug and panics at registration
// time (see ValidateSet).
//
// # What stays core-only
//
// A few decode behaviors are structural — they cross node boundaries and
// therefore cannot live on a per-node hook (Registration.DecodedByCore
// marks their registrations):
//
//   - ::colwidths: widths are emitted from the FOLLOWING sibling table's
//     cell attributes (and re-attached to it on encode) — a cross-sibling
//     application.
//   - the block-mark wrappers (:::center/:::end, :::indent, :::breakout,
//     :::dataConsumer, :::fragment): ADF block marks wrap the marked
//     block's entire decoded form, including companions like a table's
//     ::colwidths node.
//   - :emoji: the inline visitor special-cases text-present emojis and
//     the shortname→unicode table before falling back to the directive.
//
// The ADF-text-mark-backed kinds (:color, :bg, :u, :sub, :sup,
// :fontSize, :annotation) decode through DecodeTextMark: the core mark
// machinery owns which marks project and their canonical nesting order,
// and dispatches each mark to the registered hooks for node
// construction.
//
// The three context interfaces (RenderContext, EncodeContext,
// DecodeContext) expose the controlled primitives of the markdown and
// convert packages without importing them: markdown implements
// RenderContext, convert implements EncodeContext and DecodeContext.
// Extension nodes therefore depend only on ast, adf, and this package.
package extension

import (
	"errors"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
)

// Node is an AST extension node: a kind the core pipeline does not know,
// carrying its own markdown rendering and ADF encoding. The parse and
// decode directions are factory hooks on its Registration, so every
// registered kind supports all four pipeline paths.
type Node interface {
	ast.Node
	// RenderMarkdown writes the node's markdown form through the
	// renderer-controlled primitives. Escaping, wrapping, and directive
	// serialization quirks stay inside the renderer.
	RenderMarkdown(ctx RenderContext)
	// EncodeADF returns the node's ADF form. Most kinds return a single
	// node; an empty result drops the node (remark degradation).
	EncodeADF(ctx EncodeContext) []adf.Node
}

// RenderContext exposes the renderer's directive-form primitives to
// RenderMarkdown. The markdown package implements it — once for block
// position and once for inline position; calling a method that does not
// apply to the node's position is a no-op:
//
//   - WriteContainerDirective and WriteLeafDirective apply in block
//     position,
//   - WriteTextDirective applies in inline position.
//
// Directive forms are the only forms the parser can extend (parse hooks
// promote generic directive nodes), so they are also the only forms the
// render contract offers.
type RenderContext interface {
	// WriteContainerDirective writes a :::name fenced container
	// directive. A leading child paragraph flagged DirectiveLabel
	// renders as the [label] on the fence line; the remaining children
	// render as blank-line-separated blocks and the fence grows around
	// nested container directives (:::: > :::), like remark-stringify.
	// Attributes serialize on the fence line after the label, exactly
	// like the leaf form (nil for none).
	WriteContainerDirective(name string, attrs map[string]string, children []ast.Node)
	// WriteLeafDirective writes ::name[label]{attrs}: the label is the
	// children's plain text (brackets escaped), attributes serialize
	// like mdast-util-directive ({#id} shortcut, sorted quoted pairs,
	// bare names for empty values).
	WriteLeafDirective(name string, attrs map[string]string, children []ast.Node)
	// WriteTextDirective writes :name[label]{attrs} in inline position;
	// the label children render as inline content under the surrounding
	// escape context.
	WriteTextDirective(name string, attrs map[string]string, children []ast.Node)
}

// InlineLead lets an inline extension node report the first byte of its
// markdown form (mdast-util-to-markdown's peek()); the renderer feeds it
// to the previous sibling's escape checks. Inline nodes rendering a
// directive form report ':'. Without it the lead is unknown (0), which
// the escape rules treat like an empty node.
type InlineLead interface {
	MarkdownLead() byte
}

// ContainerForm marks extension nodes that render through
// RenderContext.WriteContainerDirective. The renderer sizes outer fences
// around nested container directives (:::: > :::) by walking children
// for container-form nodes — implement this so enclosing fences grow
// around the node.
type ContainerForm interface {
	Node
	// ContainerDirectiveForm is a marker method; it is never called.
	ContainerDirectiveForm()
}

// InlineStyle is the mark overlay an inline extension node applies to
// its children during ADF encoding (EncodeContext.EncodeInlinesStyled).
// Pointer fields distinguish "inherit" (nil) from "overwrite" (set, even
// to the empty string, which clears an inherited mark).
type InlineStyle struct {
	// TextColor overwrites the inherited textColor mark value.
	TextColor *string
	// BackgroundColor overwrites the inherited backgroundColor value.
	BackgroundColor *string
	// SubSup overwrites the inherited subsup mark type ("sub" | "sup").
	SubSup *string
	// FontSize overwrites the inherited fontSize mark value.
	FontSize *string
	// Annotation adds an annotation mark around the content; nested
	// annotations accumulate (never overwrite), mirroring ADF's multiple
	// annotation marks per text node.
	Annotation *Annotation
	// Underline adds the underline mark (never clears an inherited one).
	Underline bool
}

// Annotation identifies one annotation mark (a Confluence inline
// comment anchor): the annotation id and its type ("inlineComment").
type Annotation struct {
	ID             string
	AnnotationType string
}

// EncodeContext exposes the AST→ADF converter's primitives to EncodeADF.
// The convert package implements it; in inline position the context
// carries the marks inherited from enclosing constructs.
type EncodeContext interface {
	// EncodeBlocks converts child blocks to ADF block nodes.
	EncodeBlocks(children []ast.Node) []adf.Node
	// EncodeInlines converts child inlines to flat ADF inline nodes
	// under the inherited mark context.
	EncodeInlines(children []ast.Node) []adf.Node
	// EncodeInlinesStyled converts child inlines with style layered on
	// the inherited mark context (mark order stays canonical).
	EncodeInlinesStyled(style InlineStyle, children []ast.Node) []adf.Node
	// SmartLinkURL resolves a smart-link label to a URL: bare keys
	// expand via the configured SmartLinks resolver when known (and stay
	// as-is otherwise), full URLs pass through; "" for a blank label.
	SmartLinkURL(label string) string
	// AssetID resolves a markdown-relative asset reference (e.g.
	// "assets/shot.png") back to its product media id via the configured
	// asset-store resolver. Lets a media directive omit an explicit id when
	// the store can recover it. Returns ("", false) when no resolver is
	// configured or the reference is unknown.
	AssetID(ref string) (id string, ok bool)
	// AssetDims resolves the intrinsic pixel dimensions of a markdown-relative
	// asset reference from the local file. Lets a media directive omit width/
	// height when they match the file. Returns (0, 0, false) when no resolver
	// is configured or the file is not a measurable image.
	AssetDims(ref string) (width, height int, ok bool)
}

// MediaAsset describes a downloaded attachment available on disk for
// image-form rendering (see DecodeContext.Asset).
type MediaAsset struct {
	// Path is relative to the markdown file (e.g. "assets/KEY-shot.png").
	Path string
	// Width/Height are the intrinsic pixel dimensions; valid when HasDim.
	Width  int
	Height int
	// HasDim reports whether the asset is a parseable image with known
	// dimensions.
	HasDim bool
}

// DecodeContext exposes the ADF→AST converter's primitives to the decode
// hooks. The convert package implements it.
type DecodeContext interface {
	// DecodeBlocks converts ADF block nodes to AST blocks (extension
	// dispatch included).
	DecodeBlocks(nodes []adf.Node) []ast.Node
	// DecodeInlines converts ADF inline nodes to AST inlines, rebuilding
	// nested mark wrappers from ADF's flat mark arrays.
	DecodeInlines(nodes []adf.Node) []ast.Node
	// SmartLinkLabel is the display label for a smart-link URL: the
	// short key when the configured SmartLinks resolver knows the URL,
	// the URL itself otherwise.
	SmartLinkLabel(url string) string
	// Asset looks up a downloaded media asset by media id.
	Asset(id string) (MediaAsset, bool)
	// PreserveLocalImages reports whether external media carrying a
	// document-relative URL should render back to a plain ![alt](path)
	// image (the WithPreserveLocalImages round-trip) rather than a
	// ::media directive. Off by default.
	PreserveLocalImages() bool
}

// Registration bundles the two factory-direction hooks of one extension
// kind: parse (promoting a generic directive node into the typed node)
// and decode (recognizing the ADF shape the kind owns). Together with
// the Node methods (render, encode) a registered kind covers all four
// pipeline paths; Validate rejects incomplete bundles.
type Registration struct {
	Containers      map[string]func(*ast.ContainerDirective) Node
	Leaves          map[string]func(*ast.LeafDirective) Node
	Texts           map[string]func(*ast.TextDirective) Node
	DecodeBlock     func(n adf.Node, ctx DecodeContext) (ast.Node, bool)
	DecodeBlockList func(n adf.Node, ctx DecodeContext) ([]ast.Node, bool)
	DecodeInline    func(n adf.Node, ctx DecodeContext) ([]ast.Node, bool)
	// DecodeTextMark decodes one ADF text MARK the kind owns into its
	// typed wrapper node around the already-decoded inner content
	// (adf→ast for mark-backed kinds like :u or :fontSize, which have no
	// ADF node of their own). The core mark machinery owns which marks
	// project and their canonical nesting order; it calls the registered
	// hooks in order for each projected mark and uses the first claimed
	// result. Marks the core does not consume itself are offered in ADF
	// mark-array order, wrapping outside the canonical stack with the
	// FIRST mark innermost (matching the block-mark canon); unclaimed
	// marks drop, as before.
	DecodeTextMark func(mark adf.Mark, inner []ast.Node) (ast.Node, bool)
	Kind           string
	DecodedByCore  bool
}

// Validate reports whether the registration is a complete, well-formed
// bundle:
//
//   - at least one parse constructor (Containers/Leaves/Texts),
//   - a decode hook (DecodeBlock/DecodeBlockList/DecodeInline/
//     DecodeTextMark) or the documented DecodedByCore exemption,
//   - every constructor produces a structurally valid prototype: non-nil,
//     implementing ast.Parent (child access for the generic tree walks),
//     ContainerForm for container constructors (fence sizing), and
//     InlineLead for text constructors (escape lookahead).
//
// The prototype check runs each constructor once on an empty directive,
// so structural mistakes fail fast at registration instead of panicking
// mid-conversion. The markdown and convert registries call Validate when
// extensions are supplied.
func (r Registration) Validate() error {
	if err := r.validateHooks(); err != nil {
		return err
	}
	if err := validateCtors(r, r.Containers, blankContainer, true, false); err != nil {
		return err
	}
	if err := validateCtors(r, r.Leaves, blankLeaf, false, false); err != nil {
		return err
	}
	return validateCtors(r, r.Texts, blankText, false, true)
}

// validateHooks checks that the bundle has both directions wired: at
// least one parse constructor and one decode hook (or the DecodedByCore
// exemption).
func (r Registration) validateHooks() error {
	if len(r.Containers)+len(r.Leaves)+len(r.Texts) == 0 {
		return errors.New("extension registration " + r.Kind + ": no parse constructors (Containers/Leaves/Texts)")
	}
	if r.DecodeBlock == nil && r.DecodeBlockList == nil && r.DecodeInline == nil && r.DecodeTextMark == nil && !r.DecodedByCore {
		return errors.New("extension registration " + r.Kind + ": no decode hook (DecodeBlock/DecodeBlockList/DecodeInline/DecodeTextMark) and not DecodedByCore")
	}
	return nil
}

// validateCtors runs every constructor of one position on an empty
// directive of that position and validates the prototype it returns. The
// three positions differ only in their directive type, which blank
// supplies.
func validateCtors[D any](r Registration, ctors map[string]func(D) Node, blank func(name string) D, wantContainer, wantInline bool) error {
	for name, ctor := range ctors {
		if err := r.validatePrototype(name, ctor(blank(name)), wantContainer, wantInline); err != nil {
			return err
		}
	}
	return nil
}

func blankContainer(name string) *ast.ContainerDirective { return &ast.ContainerDirective{Name: name} }
func blankLeaf(name string) *ast.LeafDirective           { return &ast.LeafDirective{Name: name} }
func blankText(name string) *ast.TextDirective           { return &ast.TextDirective{Name: name} }

// validatePrototype checks one constructed instance against the
// structural requirements of its position.
func (r Registration) validatePrototype(name string, n Node, wantContainer, wantInline bool) error {
	where := "extension registration " + r.Kind + ", constructor " + name
	if n == nil {
		return errors.New(where + ": constructor returned nil")
	}
	if _, ok := ast.Node(n).(ast.Parent); !ok {
		return errors.New(where + ": node does not implement ast.Parent (add ChildNodes/SetChildNodes so tree walks reach its children)")
	}
	if wantContainer {
		if _, ok := ast.Node(n).(ContainerForm); !ok {
			return errors.New(where + ": container node does not implement ContainerForm (add the ContainerDirectiveForm marker so enclosing fences grow around it)")
		}
	}
	if wantInline {
		if _, ok := ast.Node(n).(InlineLead); !ok {
			return errors.New(where + ": inline node does not implement InlineLead (add MarkdownLead so neighbor escape checks see its first byte)")
		}
	}
	return nil
}

// ValidateSet validates a user-supplied registration set: every
// registration must pass Validate, and no directive name may be
// registered twice within the set (per position — container, leaf,
// text). Duplicates ACROSS sets are legal and ordered — user
// registrations deliberately override the default dialect — but within
// one set a duplicate is a configuration bug, and the registries panic
// on the returned error at registration time.
func ValidateSet(regs []Registration) error {
	seen := dirNames{}
	for _, reg := range regs {
		if err := reg.Validate(); err != nil {
			return err
		}
		if err := claimNames(seen, "container", reg.Containers, reg.Kind); err != nil {
			return err
		}
		if err := claimNames(seen, "leaf", reg.Leaves, reg.Kind); err != nil {
			return err
		}
		if err := claimNames(seen, "text", reg.Texts, reg.Kind); err != nil {
			return err
		}
	}
	return nil
}

// dirNames records which registration claimed each (position, name) pair
// within one set.
type dirNames map[dirNameKey]string

type dirNameKey struct{ position, name string }

// claim records one directive name for a position, reporting the clash
// when an earlier registration in the same set already took it.
func (s dirNames) claim(position, name, kind string) error {
	key := dirNameKey{position, name}
	if prev, dup := s[key]; dup {
		return errors.New("extension registrations " + prev + " and " + kind +
			" both register " + position + " directive name " + name +
			" — duplicate names within one set are ambiguous")
	}
	s[key] = kind
	return nil
}

// claimNames claims every name in one position's constructor map.
func claimNames[D any](s dirNames, position string, ctors map[string]func(D) Node, kind string) error {
	for name := range ctors {
		if err := s.claim(position, name, kind); err != nil {
			return err
		}
	}
	return nil
}
