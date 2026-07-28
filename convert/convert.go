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
	resolveImageDims      ImageDimsResolver
	resolveAssetID        AssetIDResolver
	diagnostics           func(Diagnostic)
	mediaAssets           mediaAssetMap
	codeLanguages         map[string]bool
	unsupportedKinds      map[string]bool
	unsupportedProduct    string
	extensions            []extension.Registration
	preserveListTightness bool
	preserveLocalImages   bool
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

// WithPreserveListTightness stores the source list tightness on ADF list
// nodes (as the synthetic "tight" attribute) so that a later FromADF can
// reproduce tight lists without blank lines. Use where local file
// tightness must be preserved; do NOT use when the ADF will be compared
// against Jira-sourced documents, because Jira ADF lacks this attribute
// and would diverge.
func WithPreserveListTightness() Option {
	return func(c *config) { c.preserveListTightness = true }
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
			if !a.HasDim && (a.Width != 0 || a.Height != 0) {
				a.HasDim = true
			}
			m[id] = a
		}
		c.mediaAssets = m
	}
}
