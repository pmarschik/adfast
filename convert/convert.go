// Package convert transforms between the pivot Markdown AST (ast) and the
// ADF document model (adf), in both directions: ToADF flattens nested
// inline mark wrappers into ADF's flat mark arrays, FromADF rebuilds the
// nested wrappers. The types here (SmartLinks, MediaAsset, the resolver
// funcs, Diagnostic) parameterize those conversions.
//
// The known dialect kinds convert through the extension contract: ToADF
// dispatches extension.Node values to their EncodeADF methods, FromADF
// dispatches ADF nodes to the registered decode hooks (the dialect set
// by default, plus WithExtensions). Only the generic directive fallback,
// the mark machinery (which constructs the typed mark kinds), and the
// cross-sibling ::colwidths and ::decisions applications remain
// structural here.
//
// The audience is the root adfast facade and consumers assembling custom
// pipelines around the pivot AST; most users should stay on the root
// facade. The surface is stable alongside the root package.
package convert

import (
	"strings"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/extension"
)

// SmartLinks teaches the conversion about a host product's URL scheme so
// links and Atlassian smart-link cards convert automatically. The zero
// value disables all automatic link handling; either func may be nil.
type SmartLinks struct {
	// KeyFromURL derives the short display key for a URL the host
	// product owns (e.g. a Jira issue URL → "ABC-123"). It labels
	// inlineCards/blockCards when rendering markdown, and a markdown
	// link whose text equals the derived key encodes back as an
	// inlineCard.
	KeyFromURL func(url string) (key string, ok bool)
	// URLForKey expands a bare key label (::linkCard[ABC-123]) to its
	// URL when encoding markdown to ADF.
	URLForKey func(key string) (url string, ok bool)
}

// LinkResolver rewrites ordinary labeled-link destinations at the product
// boundary. Encode maps a Markdown href to the href stored in ADF; Decode maps
// that ADF href back to the stable Markdown form. The zero value is inert and
// either direction may be omitted.
type LinkResolver struct {
	Encode func(href string) (resolved string, ok bool)
	Decode func(href string) (resolved string, ok bool)
}

// FileCard is the inline file card a link publishes as: an attachment of the
// host document, named by its media id and by the collection it hangs off.
type FileCard struct {
	// ID is the media id of the attachment the card shows.
	ID string
	// Collection is the media collection the attachment lives in. Confluence
	// needs it — a card without one renders as nothing at all.
	Collection string
}

// FileCardLink is the link a card reads back as. An empty Label falls back to
// the card's own alt text, and then to the last segment of Href.
type FileCardLink struct {
	Href  string
	Label string
}

// FileCards decides which ordinary labeled links are inline file cards, in
// both directions. The zero value is inert and either func may be nil.
//
// It exists because the two forms mean the same thing to a reader and not to
// the wire: Confluence writes a mediaInline card for a file somebody drops on
// a page, and a labeled link for one somebody typed. A product that publishes
// its own attachments wants the first form on the way out and the second on
// the way back, and only the product knows the media id an href stands for.
type FileCards struct {
	// Card answers for a link href that names an attachment of the host
	// document. Read by ToADF, after LinkResolver.Encode — the href it sees is
	// the one the ADF stores.
	Card func(href string) (FileCard, bool)
	// Link answers with the link a card reads back as. Read by FromADF, before
	// LinkResolver.Decode, so returning the ADF-side href is enough when a
	// resolver already maps that href home.
	Link func(id string) (FileCardLink, bool)
}

// ImageDimsResolver resolves a local image path (relative to the markdown
// file) to its intrinsic pixel dimensions.
type ImageDimsResolver func(path string) (width, height int, ok bool)

// AssetIDResolver maps a referenced asset path (relative to the markdown
// file) back to its media id via the asset store.
type AssetIDResolver func(path string) (id string, ok bool)

// Diagnostic reports a non-fatal conversion issue: input that was
// accepted but could not be fully honored (dropped constructs, recovered
// parses, unknown ADF kinds). Code is a stable identifier suitable for
// lint rules. It is an alias of adf.Diagnostic, the type's home — the
// decode codec emits the same shape.
type Diagnostic = adf.Diagnostic

// MediaAsset describes a downloaded attachment for image-form rendering.
// It is an alias of extension.MediaAsset, the type's canonical home (the
// decode contract hands the same shape to extension hooks) — like
// Diagnostic, the alias keeps this package's option surface
// self-contained without forking the type.
type MediaAsset = extension.MediaAsset

type config struct {
	smartLinks            SmartLinks
	linkResolver          LinkResolver
	fileCards             FileCards
	resolveImageDims      ImageDimsResolver
	resolveAssetID        AssetIDResolver
	diagnostics           func(Diagnostic)
	mediaAssets           mediaAssetMap
	resolveMediaAsset     MediaAssetResolver
	codeLanguages         map[string]bool
	unsupportedKinds      map[string]bool
	unsupportedProduct    string
	noAnchorsProduct      string
	extensions            []extension.Registration
	noHeadingAnchors      bool
	preserveListTightness bool
	preserveLocalImages   bool
	incrementListMarkers  bool
}

// Option configures the ToADF and FromADF conversions; each option
// documents which direction reads it.
type Option func(*config)

// WithSmartLinks teaches both conversion directions about a host
// product's URL scheme (see SmartLinks). In ToADF, links whose text
// equals the derived key encode as inlineCards and bare
// ::linkCard/::linkEmbed key labels expand to URLs; in FromADF,
// inlineCards, ::linkCard, and ::linkEmbed labels use the short key
// derived by sl.KeyFromURL instead of the full URL.
func WithSmartLinks(sl SmartLinks) Option {
	return func(c *config) { c.smartLinks = sl }
}

// WithLinkResolver rewrites ordinary link-mark hrefs in both conversion
// directions (see LinkResolver). Smart-link cards and media nodes are not
// affected.
func WithLinkResolver(r LinkResolver) Option {
	return func(c *config) { c.linkResolver = r }
}

// WithFileCards publishes the links a host product owns as inline file cards
// and reads them back as those links (see FileCards). Read by ToADF and
// FromADF. Links the resolver does not answer for, smart-link cards, and block
// media are not affected.
func WithFileCards(f FileCards) Option {
	return func(c *config) { c.fileCards = f }
}

// WithPreserveListTightness stores the source list tightness on ADF list
// nodes (as the synthetic "tight" attribute) so that a later FromADF can
// reproduce tight lists without blank lines. Use where local file
// tightness must be preserved; do NOT use when the ADF will be compared
// against Jira-sourced documents, because Jira ADF lacks this attribute
// and would diverge.
func WithPreserveListTightness() Option {
	return func(c *config) { c.preserveListTightness = true }
}

// WithIncrementListMarkers renumbers the items of an ADF ordered list
// 1. 2. 3. instead of repeating the list's start number on every item.
// Read by FromADF. ADF records no marker style, so the reference
// rendering repeats the start number (remark-stringify with
// incrementListMarker off); use this where the markdown is written and
// read by people, and where an ordered list a document already spelled
// 1. 2. 3. must survive the round trip unchanged.
func WithIncrementListMarkers() Option {
	return func(c *config) { c.incrementListMarkers = true }
}

// WithPreserveLocalImages keeps a document-relative image reference
// (![alt](assets/x.png)) as external media carrying the path when the
// asset store cannot resolve it to an uploaded media id, instead of
// dropping it (the remark-reference default). Use it for store-aware
// round-trips and diff normalization where a not-yet-uploaded local image
// must survive — a later push upload then resolves it to file media. Do
// NOT use it for the final Jira push encode: an image left unresolved
// there should drop with an unresolved-asset diagnostic rather than send
// invalid external media to Jira.
func WithPreserveLocalImages() Option {
	return func(c *config) { c.preserveLocalImages = true }
}

// WithImageDimsResolver supplies the resolver used to re-derive intrinsic
// media dimensions from downloaded asset files during encoding
// (dimensions that match the file are omitted from adf: image titles).
// Read by ToADF.
func WithImageDimsResolver(r ImageDimsResolver) Option {
	return func(c *config) { c.resolveImageDims = r }
}

// WithAssetIDResolver supplies the asset-store lookup used to convert
// ![alt](assets/name) references back to ADF media nodes during
// encoding. Read by ToADF.
func WithAssetIDResolver(r AssetIDResolver) Option {
	return func(c *config) { c.resolveAssetID = r }
}

// WithCodeLanguages declares the code-block languages the host product
// supports: ToADF emits an "unsupported-code-language" diagnostic for
// every fenced code block whose language tag is not in the set (see
// CodeUnsupportedCodeLanguage). Conversion behavior is unchanged — the
// language encodes verbatim either way; a code block without a language
// never reports. Matching is case-insensitive on the fence info string's
// first word; no alias normalization is applied, so the set should list
// every accepted alias. An empty set disables the check. Read by ToADF.
func WithCodeLanguages(langs []string) Option {
	return func(c *config) {
		if len(langs) == 0 {
			return
		}
		set := make(map[string]bool, len(langs))
		for _, lang := range langs {
			set[strings.ToLower(lang)] = true
		}
		c.codeLanguages = set
	}
}

// WithUnsupportedKinds declares the ADF node/mark kinds the target
// product does not render, so ToADF can flag a document that uses one.
// After the document is produced, ToADF walks it over both nodes and
// marks and emits one "unsupported-in-product" diagnostic (see
// CodeUnsupportedInProduct) per DISTINCT offending kind, naming the
// product and the kind (e.g. `extension is not available in jira`).
//
// The mechanism is product-neutral: the caller owns which kinds and the
// product label (the jira/confluence submodules supply their sets,
// derived from docs/adf-availability.json). Kinds match the ADF
// node/mark Kind() strings as they appear in the produced document —
// the same type strings used in the availability dataset.
//
// Conversion behavior is UNCHANGED — no node or mark is dropped or
// altered; this is a pure authoring-side signal and the consumer decides
// severity. An empty set (or no diagnostics sink) disables the check.
// Read by ToADF.
func WithUnsupportedKinds(product string, kinds []string) Option {
	return func(c *config) {
		if len(kinds) == 0 {
			return
		}
		set := make(map[string]bool, len(kinds))
		for _, k := range kinds {
			set[k] = true
		}
		c.unsupportedProduct = product
		c.unsupportedKinds = set
	}
}

// WithoutHeadingAnchors declares that the target product has no heading
// anchor construct, so ToADF drops every heading's {#id} anchor and
// reports each one as a "heading-anchor-dropped" diagnostic naming the
// product (see CodeHeadingAnchorDropped).
//
// The anchor is a synthetic attribute with no wire form of its own (see
// adf.Heading.Anchor and adf.IsWireSafe): SOMETHING has to resolve it
// before submission, either by lowering it to the product's own construct
// (confluence.LowerAnchors) or by dropping it here. This is the second
// half of that pair, and unlike WithUnsupportedKinds it does change the
// output — the alternative is a document the product rejects or silently
// mangles.
//
// The mechanism is product-neutral: the caller owns the label and the
// judgement that the product lacks anchors (jira.MarkdownOptions supplies
// both). Read by ToADF.
func WithoutHeadingAnchors(product string) Option {
	return func(c *config) {
		c.noHeadingAnchors = true
		c.noAnchorsProduct = product
	}
}

// WithDiagnostics registers a sink for non-fatal conversion diagnostics,
// e.g. a ::colwidths directive with no following table
// ("colwidths-orphan") in ToADF, or an unknown ADF node reaching the
// markdown projection ("raw-node") in FromADF. Without a sink,
// diagnostics are silently dropped. Read by ToADF and FromADF.
func WithDiagnostics(sink func(Diagnostic)) Option {
	return func(c *config) { c.diagnostics = sink }
}

// WithExtensions registers additional AST extension kinds (see the
// extension package) on top of the default dialect set. Read by FromADF,
// whose decode dispatch tries the user-supplied hooks BEFORE the
// dialect's (user registrations override, see the extension package's
// conflict policy); ToADF needs no registry because encoding dispatches
// on the extension.Node interface itself. Registrations must be complete
// bundles and free of duplicate names within the supplied set
// (extension.ValidateSet); a violation panics at conversion time.
func WithExtensions(regs ...extension.Registration) Option {
	return func(c *config) { c.extensions = append(c.extensions, regs...) }
}

// WithMediaAssets maps media ids to downloaded local files so file media
// renders as plain images when the local asset carries every ADF
// property (see the dialect package's media decoding). Read by FromADF.
//
// HasDim semantics: an asset with HasDim set is taken at its word. For
// callers that never set HasDim (the historical struct had no such
// field), it is derived: a nonzero Width or Height implies known
// dimensions (HasDim true), both zero means not a parseable image
// (HasDim false).
func WithMediaAssets(assets map[string]MediaAsset) Option {
	return func(c *config) {
		m := make(mediaAssetMap, len(assets))
		for id, a := range assets {
			m[id] = withDerivedDims(a)
		}
		c.mediaAssets = m
	}
}

// WithMediaAssetResolver supplies the same knowledge as WithMediaAssets one
// media id at a time (see MediaAssetResolver): the conversion asks about the
// media it meets instead of being handed a whole collection, which is what a
// caller wants when its collection is large or when producing an entry costs
// something. Read by FromADF and the prettier-format mode of ToMarkdown.
//
// Both options may be set: the map is consulted first and the resolver answers
// for ids it does not cover. Resolver replies get the same HasDim treatment
// WithMediaAssets documents.
func WithMediaAssetResolver(r MediaAssetResolver) Option {
	return func(c *config) { c.resolveMediaAsset = r }
}
