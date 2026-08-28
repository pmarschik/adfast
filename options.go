package adfast

import (
	"strings"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/convert"
	"github.com/pmarschik/adfast/extension"
)

// Option configures the facade primitives. The facade shares ONE option
// type across FromMarkdown, FromADF, ToADF, and ToMarkdown: every option
// sets a field, and each primitive reads only the fields that apply to
// it (documented on the option and on the primitive). This keeps one
// coherent vocabulary — a smart-link scheme, an extension bundle, or a
// diagnostics sink is described once and reused wherever it is relevant —
// instead of four parallel families that drift apart. Options a primitive
// does not read are ignored.
type Option func(*options)

// options is the union of everything the four primitives read.
type options struct {
	smartLinks          convert.SmartLinks
	linkResolver        convert.LinkResolver
	fileCards           convert.FileCards
	mediaAssets         map[string]convert.MediaAsset
	resolveMediaAsset   convert.MediaAssetResolver
	frontmatter         FrontmatterProvider
	diagnostics         func(convert.Diagnostic)
	blockSep            *string
	printWidth          *int
	resolveImageDims    convert.ImageDimsResolver
	resolveAssetID      convert.AssetIDResolver
	noAnchorsProduct    string
	unsupportedKind     string
	unsupportedKinds    []string
	codeLanguages       []string
	codeLanguageAliases map[string]string
	docTransforms       []func(adf.Doc) adf.Doc
	adfTransforms       []func(adf.Doc) adf.Doc
	astTransforms       []func(ast.Node)
	extensions          []extension.Registration
	noHeadingAnchors    bool
	preserveTight       bool
	preserveLocalImages bool
	incrementLists      bool
	noWrap              bool
	prettier            bool
	noSpaceEscapes      bool
}

func newOptions(opts []Option) options {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// convertOptions builds the convert.Option list common to the AST⇄ADF
// conversions from the resolved facade options. The groups append in a
// fixed order: a later option overrides an earlier one, so the order the
// helpers run in is part of the contract.
func (o *options) convertOptions() []convert.Option {
	var out []convert.Option
	out = o.appendLinkOptions(out)
	out = o.appendPolicyOptions(out)
	out = o.appendShapeOptions(out)
	out = o.appendAssetOptions(out)
	if len(o.extensions) > 0 {
		out = append(out, convert.WithExtensions(o.extensions...))
	}
	if o.diagnostics != nil {
		out = append(out, convert.WithDiagnostics(o.diagnostics))
	}
	return out
}

// appendLinkOptions carries the URL-shaping resolvers.
func (o *options) appendLinkOptions(out []convert.Option) []convert.Option {
	if o.smartLinks.KeyFromURL != nil || o.smartLinks.URLForKey != nil {
		out = append(out, convert.WithSmartLinks(o.smartLinks))
	}
	if o.linkResolver.Encode != nil || o.linkResolver.Decode != nil {
		out = append(out, convert.WithLinkResolver(o.linkResolver))
	}
	if o.fileCards.Card != nil || o.fileCards.Link != nil {
		out = append(out, convert.WithFileCards(o.fileCards))
	}
	return out
}

// appendPolicyOptions carries what the target product accepts.
func (o *options) appendPolicyOptions(out []convert.Option) []convert.Option {
	if len(o.codeLanguages) > 0 {
		out = append(out, convert.WithCodeLanguages(o.codeLanguages))
	}
	if len(o.codeLanguageAliases) > 0 {
		out = append(out, convert.WithCanonicalCodeLanguages(o.codeLanguageAliases))
	}
	if len(o.unsupportedKinds) > 0 {
		out = append(out, convert.WithUnsupportedKinds(o.unsupportedKind, o.unsupportedKinds))
	}
	if o.noHeadingAnchors {
		out = append(out, convert.WithoutHeadingAnchors(o.noAnchorsProduct))
	}
	return out
}

// appendShapeOptions carries the markdown-surface preservation flags.
func (o *options) appendShapeOptions(out []convert.Option) []convert.Option {
	if o.preserveTight {
		out = append(out, convert.WithPreserveListTightness())
	}
	if o.preserveLocalImages {
		out = append(out, convert.WithPreserveLocalImages())
	}
	if o.incrementLists {
		out = append(out, convert.WithIncrementListMarkers())
	}
	return out
}

// appendAssetOptions carries the media/attachment resolvers.
func (o *options) appendAssetOptions(out []convert.Option) []convert.Option {
	if o.resolveImageDims != nil {
		out = append(out, convert.WithImageDimsResolver(o.resolveImageDims))
	}
	if o.resolveAssetID != nil {
		out = append(out, convert.WithAssetIDResolver(o.resolveAssetID))
	}
	if len(o.mediaAssets) > 0 {
		out = append(out, convert.WithMediaAssets(o.mediaAssets))
	}
	if o.resolveMediaAsset != nil {
		out = append(out, convert.WithMediaAssetResolver(o.resolveMediaAsset))
	}
	return out
}

// WithExtensions registers additional AST extension kinds (see the
// extension package) on top of the default dialect set. Read by
// FromMarkdown (their directive names promote to the typed nodes on
// parse), ToADF (the typed nodes encode themselves — no registry needed),
// FromADF (their decode hooks recognize the ADF shapes they own), and the
// prettier-format mode of ToMarkdown. Register the same bundle wherever
// both conversion directions run so the parse and decode halves never
// drift apart.
func WithExtensions(regs ...extension.Registration) Option {
	return func(o *options) { o.extensions = append(o.extensions, regs...) }
}

// WithSmartLinks teaches the conversion about a host product's URL scheme
// (see convert.SmartLinks). Read by ToADF (links whose text equals the
// derived key encode as inlineCards; bare ::linkCard/::linkEmbed key
// labels expand to URLs), FromADF and the prettier-format mode of
// ToMarkdown (inlineCards and card labels use the short key). One scheme
// serves every direction.
func WithSmartLinks(sl convert.SmartLinks) Option {
	return func(o *options) { o.smartLinks = sl }
}

// WithLinkResolver rewrites ordinary labeled-link destinations between their
// Markdown and ADF forms (see convert.LinkResolver). Read by ToADF, FromADF,
// and the prettier-format mode of ToMarkdown. Smart-link cards and media are
// unaffected.
func WithLinkResolver(r convert.LinkResolver) Option {
	return func(o *options) { o.linkResolver = r }
}

// WithFileCards publishes the links a host product owns as the inline file
// cards its own editor writes for an attached file, and reads those cards back
// as the same links (see convert.FileCards). Read by ToADF and FromADF. Only
// the links the resolver answers for are affected.
func WithFileCards(f convert.FileCards) Option {
	return func(o *options) { o.fileCards = f }
}

// WithCodeLanguages declares the code-block languages the host product
// supports (see convert.WithCodeLanguages): fenced code blocks whose
// language tag falls outside the set report an "unsupported-code-language"
// diagnostic; conversion is unchanged. Read by ToADF and the
// prettier-format mode of ToMarkdown.
func WithCodeLanguages(langs []string) Option {
	return func(o *options) { o.codeLanguages = append(o.codeLanguages, langs...) }
}

// WithCanonicalCodeLanguages normalizes a fenced code block's language
// tag to the host editor's canonical spelling on the way into ADF (see
// convert.WithCanonicalCodeLanguages): with
// AtlaskitCodeLanguageAliases, a ```bash fence encodes as codeBlock
// language "shell" — the identifier Atlassian's own picker writes back —
// and an unknown tag encodes verbatim. Read by ToADF ONLY: the render
// direction has nothing to normalize, and ToMarkdown therefore leaves
// every fence exactly as written, which is what keeps a local file's
// ```bash intact.
//
// The trade this makes is stated in full on convert.WithCanonicalCodeLanguages:
// the alias does not survive a round trip through a product that stored
// the canonical spelling, but a diff that canonicalizes both sides stops
// reporting the difference as a change.
//
// A later call replaces an earlier one. Read by ToADF.
func WithCanonicalCodeLanguages(aliases map[string]string) Option {
	return func(o *options) { o.codeLanguageAliases = aliases }
}

// WithUnsupportedKinds declares the ADF node/mark kinds the target
// product does not render (see convert.WithUnsupportedKinds): after
// ToADF produces the document it walks both nodes and marks and emits
// one "unsupported-in-product" diagnostic per distinct offending kind,
// naming the product and the kind. The mechanism is product-neutral —
// the caller owns the set and the product label; the jira and confluence
// submodules supply their own sets via MarkdownOptions. Conversion
// output is unchanged (diagnostic-only) and the consumer decides
// severity. Read by ToADF.
func WithUnsupportedKinds(product string, kinds []string) Option {
	return func(o *options) {
		if len(kinds) == 0 {
			return
		}
		o.unsupportedKind = product
		o.unsupportedKinds = append(o.unsupportedKinds, kinds...)
	}
}

// WithoutHeadingAnchors declares that the target product has no heading
// anchor construct (see convert.WithoutHeadingAnchors): ToADF drops every
// heading's {#id} anchor and reports each one as a
// "heading-anchor-dropped" diagnostic naming the product.
//
// A heading anchor has no wire form of its own, so on the encode side it
// must either be lowered to the product's own construct — Confluence's
// anchor macro, which confluence.MarkdownOptions installs — or dropped
// here. jira.MarkdownOptions supplies this half; Jira has no anchors.
// Unlike WithUnsupportedKinds this is not diagnostic-only: it changes the
// document. Read by ToADF.
func WithoutHeadingAnchors(product string) Option {
	return func(o *options) {
		o.noHeadingAnchors = true
		o.noAnchorsProduct = product
	}
}

// WithPreserveListTightness stores the source list tightness on ADF list
// nodes so that FromADF can reproduce tight lists without blank lines.
// Read by ToADF. Do NOT use when the resulting ADF is diffed against
// Jira-sourced documents, which lack this attribute.
func WithPreserveListTightness() Option {
	return func(o *options) { o.preserveTight = true }
}

// WithIncrementListMarkers renumbers the items of an ADF ordered list
// 1. 2. 3. instead of repeating the list's start number on every item.
// Read by FromADF. ADF records no marker style, so the reference
// rendering repeats the start number (remark-stringify with
// incrementListMarker off); use this where the Markdown is written and
// read by people, and where a list a document already spelled 1. 2. 3.
// has to survive the round trip unchanged.
func WithIncrementListMarkers() Option {
	return func(o *options) { o.incrementLists = true }
}

// WithPreserveLocalImages keeps an unresolved document-relative image
// reference (![alt](assets/x.png)) as external media carrying the path
// instead of dropping it (the remark-reference default). Read by ToADF.
// Use it for store-aware round-trips and diff normalization where a
// not-yet-uploaded local image must survive so a later push upload can
// resolve it; do NOT use it for the final Jira push encode, where an
// unresolved image should drop with an unresolved-asset diagnostic.
func WithPreserveLocalImages() Option {
	return func(o *options) { o.preserveLocalImages = true }
}

// WithImageDimsResolver supplies the resolver used to re-derive intrinsic
// media dimensions from downloaded asset files during encoding. Read by
// ToADF.
func WithImageDimsResolver(r convert.ImageDimsResolver) Option {
	return func(o *options) { o.resolveImageDims = r }
}

// WithAssetIDResolver supplies the asset-store lookup used to convert
// ![alt](assets/name) references back to ADF media nodes during encoding.
// Read by ToADF.
func WithAssetIDResolver(r convert.AssetIDResolver) Option {
	return func(o *options) { o.resolveAssetID = r }
}

// WithDocTransforms appends ADF document transforms applied, in order,
// after ToADF's conversion (e.g. bare issue-key expansion). Read by ToADF.
func WithDocTransforms(ts ...func(adf.Doc) adf.Doc) Option {
	return func(o *options) { o.docTransforms = append(o.docTransforms, ts...) }
}

// WithADFTransforms appends ADF document transforms applied, in order,
// BEFORE FromADF's conversion — the decode-side mirror of
// WithDocTransforms. Read by FromADF.
//
// It exists for the shapes a per-node decode hook cannot reach: an
// extension.Registration decodes one ADF node into one AST node, so a
// product construct that is structurally spread across nodes — a
// Confluence anchor macro sitting inside a heading's content, which has to
// leave that content and become an attribute of the heading itself —
// needs a pass over the whole document first. Rewriting the ADF into the
// shape the conversion already understands keeps such lowerings in the
// product addon (see confluence.RenderOptions) instead of the neutral
// core.
func WithADFTransforms(ts ...func(adf.Doc) adf.Doc) Option {
	return func(o *options) { o.adfTransforms = append(o.adfTransforms, ts...) }
}

// WithMediaAssets maps media ids to downloaded local files so file media
// renders as plain images (see convert.WithMediaAssets). Read by FromADF
// and the prettier-format mode of ToMarkdown.
func WithMediaAssets(assets map[string]convert.MediaAsset) Option {
	return func(o *options) { o.mediaAssets = assets }
}

// WithMediaAssetResolver answers the same question as WithMediaAssets one media
// id at a time, so a conversion only asks about the media it meets (see
// convert.WithMediaAssetResolver). Prefer it when the caller's collection is
// large, or when producing an entry costs something. Read by FromADF and the
// prettier-format mode of ToMarkdown.
func WithMediaAssetResolver(r convert.MediaAssetResolver) Option {
	return func(o *options) { o.resolveMediaAsset = r }
}

// WithPrintWidth sets the paragraph wrapping width. Pass 0 to disable
// wrapping. Read by ToMarkdown.
func WithPrintWidth(width int) Option {
	return func(o *options) { w := width; o.printWidth = &w }
}

// WithNoWrap disables paragraph wrapping, matching remark-stringify's
// default of preserving long lines. Read by ToMarkdown.
func WithNoWrap() Option {
	return func(o *options) { o.noWrap = true }
}

// WithoutSignificantSpaceEscapes writes a significant boundary space as
// the space itself instead of the "&#x20;" character reference. Read by
// ToMarkdown.
//
// THE OUTPUT IS DELIBERATELY NOT ROUND-TRIP FAITHFUL. The escape exists
// because Markdown strips whitespace at an inline boundary, so re-parsing
// this output loses exactly the space the escape would have carried, and
// re-rendering it gives a different document. Never use it for text that
// is written to a file or submitted to a remote — such text is read back,
// and what it loses is gone. It is for a comparison that renders both
// sides and throws the rendering away: a caller that must line a rendered
// document up against a working copy, which cannot spell a trailing space
// at all, gets the two sides into one spelling without a text
// substitution over the finished output (which would also hit the
// author's own "&#x20;" inside a code span, inside a fence, or behind a
// backslash, where the six characters are content and no escape at all).
//
// Only whitespace escapes are suppressed. The character references that
// keep an emphasis marker flankable — the alphanumeric neighbors of a
// marker that would otherwise read as intraword — stay, because dropping
// those changes what the text means rather than how a space survives.
func WithoutSignificantSpaceEscapes() Option {
	return func(o *options) { o.noSpaceEscapes = true }
}

// WithBlockSeparator sets the string written between consecutive
// top-level blocks (default "\n", a blank line; "" suppresses blank
// lines). Read by ToMarkdown.
func WithBlockSeparator(sep string) Option {
	return func(o *options) { s := sep; o.blockSep = &s }
}

// WithASTTransforms appends transforms run on the canonical pivot AST
// between convert.NormalizeFormat and rendering (each receives the document
// root and may edit it in place). Read by ToMarkdown in the
// prettier-format mode — the formatter's content-rewrite seam (e.g.
// re-pathing image destinations after an asset-store layout change).
func WithASTTransforms(ts ...func(ast.Node)) Option {
	return func(o *options) { o.astTransforms = append(o.astTransforms, ts...) }
}

// WithPrettierFormat selects the prettier md→md formatting mode. It is a
// purely RENDER-side flag with NO parse-side effect: the single
// FromMarkdown parse captures prettier's literal escapes as provenance on
// ast.Text.Raw (see markdown.PreservedEscapes) regardless of this flag, and
// splits frontmatter with the same FrontmatterProvider md→adf uses, so
// detection can never diverge between the two directions. ToMarkdown reads
// the flag to canonicalize the parsed tree with convert.NormalizeFormat
// (the total, format-leg canonicalizer: unlike the encode-side
// convert.Normalize it keeps the generic directives ADF cannot represent)
// and switch text serialization to prettier's rules. The prettier md→md
// formatter is thus ToMarkdown(FromMarkdown(md), WithPrettierFormat(),
// WithPrintWidth(w)) — the flag on the render call alone, with no need to
// pass it to FromMarkdown. Pair with WithPrintWidth for the wrap width
// (prettier's default is 80).
//
// The former parse-side frontmatter residual (a byte-exact splitter this
// installed on FromMarkdown, whose detection disagreed with the md→adf
// default on pathological delimiter shapes) is GONE: both directions now
// share one FrontmatterProvider. Prefer Pipeline.Format (a zero-value
// Pipeline works: (&adfast.Pipeline{}).Format(md)), which wires both
// halves for you.
func WithPrettierFormat() Option {
	return func(o *options) { o.prettier = true }
}

// WithDiagnostics registers a sink for non-fatal diagnostics, wired into
// whichever primitive emits: FromMarkdown (parse-recovered,
// malformed-frontmatter, depth-exceeded, span-marker-invalid), ToADF (colwidths-orphan,
// decisions-orphan, unresolved-asset, unsupported-code-language,
// unsupported-in-product, raw-node, depth-exceeded), and FromADF (decode-failed, unknown-node,
// unknown-mark, unknown-attr, raw-node). ToMarkdown's prettier-format
// mode runs the same canonicalization as ToADF, so the ToADF
// diagnostics (notably unsupported-code-language) also fire there.
// Without a sink, diagnostics are silently dropped.
func WithDiagnostics(sink func(convert.Diagnostic)) Option {
	return func(o *options) { o.diagnostics = sink }
}

// FrontmatterOutcome classifies what a FrontmatterProvider found at the
// head of a Markdown document.
type FrontmatterOutcome int

const (
	// FrontmatterAbsent reports no leading metadata block: the whole
	// source is body (front is "", rest is the input unchanged).
	FrontmatterAbsent FrontmatterOutcome = iota
	// FrontmatterFound reports a metadata block extracted losslessly:
	// front is the raw block (delimiters included) and rest the body.
	FrontmatterFound
	// FrontmatterMalformed reports that the document opens the metadata
	// convention (e.g. a leading "---" fence) but does not form a valid
	// block. The provider extracts nothing (front is "", rest is the input
	// unchanged), so the opening bytes are preserved as body, and
	// FromMarkdown emits a malformed-frontmatter diagnostic rather than
	// silently dropping the broken block.
	FrontmatterMalformed
)

// FrontmatterProvider extracts a leading document-metadata block from
// Markdown source under a caller-defined convention. It returns the raw
// front block (delimiters included), the remaining body, and an outcome:
//
//   - FrontmatterAbsent    — no metadata; front == "", rest == md.
//   - FrontmatterFound     — front is the verbatim block, rest the body.
//   - FrontmatterMalformed — the convention was opened but no valid block
//     formed; front == "", rest == md (kept as body).
//
// The SAME provider drives BOTH directions — the md→adf conversion and the
// prettier md→md formatter — so frontmatter detection can never diverge
// between them.
type FrontmatterProvider func(md string) (front, rest string, outcome FrontmatterOutcome)

// WithFrontmatterProvider replaces the default YAML-frontmatter handling
// with a caller-defined metadata convention (e.g. the "<!-- Space: X -->"
// HTML-comment headers used by some Confluence sync tools). Read by
// FromMarkdown in BOTH conversion directions: a FrontmatterFound block is
// kept verbatim as a leading ast.Frontmatter node (ToADF drops it; the
// renderer re-emits it) and only the body is parsed; a FrontmatterMalformed
// outcome keeps the whole source as body and emits a malformed-frontmatter
// diagnostic; FrontmatterAbsent parses the whole source as body.
func WithFrontmatterProvider(fp FrontmatterProvider) Option {
	return func(o *options) { o.frontmatter = fp }
}

// defaultFrontmatterProvider is the built-in YAML-fence convention used
// identically in both directions. A well-formed block opens with a "---\n"
// fence line at column 0 and closes with a "\n---\n" fence line; it is
// captured byte-for-byte (delimiters and trailing newline included) as
// FrontmatterFound.
//
// Pathological shapes are classified as follows (all keep their bytes as
// body — the difference is only whether a diagnostic fires):
//
//   - opens "---\n" but never closes with a "\n---\n" line (unterminated,
//     or the close line carries trailing text / lacks a trailing newline)
//     → FrontmatterMalformed.
//   - a leading "---" fence (whitespace-tolerant) followed by a later
//     "---" line that does not form a valid block (e.g. leading whitespace
//     before the fence, or trailing text on the OPEN fence like "---0")
//     → FrontmatterMalformed.
//   - a lone leading "---" with no second fence line → FrontmatterAbsent
//     (an ordinary thematic break, not an attempt at frontmatter).
//   - anything not starting with a "---" fence → FrontmatterAbsent.
func defaultFrontmatterProvider(md string) (front, rest string, outcome FrontmatterOutcome) {
	if body, ok := strings.CutPrefix(md, "---\n"); ok {
		if content, after, ok := strings.Cut(body, "\n---\n"); ok {
			front = "---\n" + content + "\n---\n"
			rest = strings.TrimPrefix(after, "\n")
			return front, rest, FrontmatterFound
		}
	}
	if opensFrontmatterConvention(md) {
		return "", md, FrontmatterMalformed
	}
	return "", md, FrontmatterAbsent
}

// opensFrontmatterConvention reports whether md looks like an attempt at a
// YAML frontmatter block: a leading "---" fence (tolerating surrounding
// whitespace) followed by a later "---" line. A lone leading "---" with no
// second fence is an ordinary thematic break, not a malformed block, so it
// stays absent. This is the signal that distinguishes a genuinely broken
// frontmatter block (worth a diagnostic) from plain body content.
func opensFrontmatterConvention(md string) bool {
	s := strings.TrimSpace(md)
	if !strings.HasPrefix(s, "---") {
		return false
	}
	_, _, ok := strings.Cut(s[3:], "\n---")
	return ok
}
