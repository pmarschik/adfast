package convert

import (
	"fmt"
	"slices"

	"github.com/pmarschik/adfast/adf"
)

// Diagnostic codes: every code a Diagnostic emitted anywhere in the
// pipeline can carry, collected here (next to the Diagnostic type's
// conversion-side home) as one stable vocabulary for sinks and lint
// rules. The decode-codec codes are re-exported from the adf package,
// where they originate.
const (
	// CodeColwidthsOrphan reports a ::colwidths directive with no
	// following table to attach its widths to; the directive is dropped.
	// Emitted by ToADF.
	CodeColwidthsOrphan = "colwidths-orphan"
	// CodeDecisionsOrphan reports a ::decisions directive with no plain
	// bullet list on the following line to mark as a decisionList; the
	// directive is dropped. Emitted by ToADF.
	CodeDecisionsOrphan = "decisions-orphan"
	// CodeParseRecovered reports that the markdown parser panicked and
	// the source was re-parsed in a normalized form (tabs expanded,
	// backticks escaped, or as plain text). Emitted by the facade parse.
	CodeParseRecovered = "parse-recovered"
	// CodeMalformedFrontmatter reports that a document opened the
	// frontmatter convention (e.g. a leading "---" fence) but did not form
	// a valid block; the opening bytes are kept as body rather than
	// silently dropped. Emitted by the facade parse.
	CodeMalformedFrontmatter = "malformed-frontmatter"
	// CodeSpanMarkerInvalid reports a table span marker (a cell containing
	// only ">" or "^") in a position where its merge cannot apply — a ">"
	// with no content cell to its right in the row, or a "^" with no
	// spanning cell above its visual column; the marker is kept as literal
	// cell text. Emitted by the facade parse.
	CodeSpanMarkerInvalid = "span-marker-invalid"
	// CodeUnresolvedAsset reports a picture the payload cannot address:
	// an asset reference the configured store could not map back to a
	// media id, or a media directive that names no source at all. ADF
	// addresses an attachment by id, so with no id nothing on the page can
	// find the file — and the spellings of a reference lose different
	// things:
	//
	//   - ![alt](assets/…): there is no node for the picture, so it is
	//     kept as external media carrying the path under
	//     WithPreserveLocalImages, and otherwise the PICTURE drops while
	//     the label stays — as a link to the destination, which is the
	//     enclosing one when the image sits inside a link. The document
	//     is never emptied by it.
	//   - ::media[alt]{path=…} (and the :::media caption form): the media
	//     node stays, because the directive is explicit and carries the
	//     alt text and the caption, but it is written with an empty id and
	//     the path has no ADF field to travel in. A path spelled beside an
	//     explicit id is not reported: the id addresses the attachment, so
	//     nothing is lost.
	//   - ::media[alt]{} (and the :::media caption form): a directive with
	//     no id, no url and no path names nothing to begin with, so this
	//     one is not a loss the encode caused — it is an unrenderable node
	//     the encode would otherwise have shipped in silence, with the
	//     author's line looking well-formed and the pushed page showing a
	//     gap. The node still ships, for the same reason as above. The
	//     inline :media[…] spelling is not covered; its vocabulary has no
	//     path, so a path on it is ignored rather than unresolvable.
	//
	// Emitted by ToADF. The md→md formatter has no counterpart: it keeps
	// the reference exactly as written.
	//
	// Whether the loss is permanent depends on WHICH of the shapes above
	// it was, and for the first two on WHEN the encode ran rather than on
	// what the document says: an asset uploads and the next encode finds
	// its id. A consumer with an upload flow should report those two only
	// from the encode that actually ships. The sourceless directive is the
	// exception — no upload resolves it, because nothing was named to
	// upload — and its message says so, so a consumer can route the two
	// apart on the text without a second code.
	CodeUnresolvedAsset = "unresolved-asset"
	// CodeUnsupportedCodeLanguage reports a fenced code block whose
	// language tag is not in the WithCodeLanguages set; the language
	// encodes verbatim anyway (Jira renders unknown languages as plain
	// text). Emitted by ToADF, only when a set is configured.
	CodeUnsupportedCodeLanguage = "unsupported-code-language"
	// CodeUnsupportedInProduct reports that the produced ADF document
	// uses a node or mark kind the target product does not render, per
	// the product-neutral set supplied via WithUnsupportedKinds. One
	// diagnostic fires per distinct offending kind; conversion output is
	// unchanged (no node or mark is dropped or altered — this is a
	// pure authoring-side signal). The consumer decides severity (e.g.
	// treat it as a blocking error before a Jira-targeted push). Emitted
	// by ToADF, only when a set is configured. See WithUnsupportedKinds.
	CodeUnsupportedInProduct = "unsupported-in-product"
	// CodeHeadingAnchorDropped reports a heading's {#id} anchor dropped
	// because the target product has no anchor construct to lower it to
	// (see WithoutHeadingAnchors). One diagnostic fires per dropped
	// anchor, naming the id, because each one is a link target the
	// rendered page will not have. The heading text is unaffected.
	// Emitted by ToADF, only when the option is set.
	CodeHeadingAnchorDropped = "heading-anchor-dropped"
	// CodeInlineImageDegraded reports an inline ![alt](url) with an
	// absolute http(s) URL rewritten as a link, because ADF has no
	// inline image that can carry one: mediaInline addresses an
	// uploaded attachment by id and has no external variant, unlike
	// block media (type "external" + url). The alt text becomes the
	// link label and the image URL its href, so the content stays
	// visible and the round trip is stable; only the "render this
	// inline" intent is lost. An inline image the asset store resolves
	// to a media id is unaffected — it becomes a real mediaInline.
	//
	// Inside a link — a linked image mid-sentence, such as a badge that
	// links to a build — the ENCLOSING destination takes the href
	// instead, because that is the one the reader means to click, and
	// the IMAGE URL is what leaves the document. The message says which
	// way round it went. Only one of the two can survive: the degraded
	// form is one text node with one link mark. The block form of the
	// same document ("[![alt](img)](href)" alone in a paragraph) loses
	// neither — it becomes media with a link mark and raises nothing.
	//
	// Emitted by ToADF, one per degraded image.
	CodeInlineImageDegraded = "inline-image-degraded"
	// CodeFootnoteFlattened reports a GFM footnote flattened because ADF
	// has no footnote construct: the reference becomes its number as
	// superscript text, and the definition moves into an ordered list
	// behind a rule at the end of the document (see the footnote section
	// of the README). Nothing is lost from the page, but the reference is
	// no longer a link to its definition — ADF has no anchor to link to —
	// and the md → ADF → md round trip returns the flattened form, not
	// the footnote. One diagnostic fires per definition, naming its
	// label and its number. Emitted by ToADF.
	CodeFootnoteFlattened = "footnote-flattened"
	// CodeUnusedDefinitionDropped reports a link reference definition
	// ("[label]: url") that nothing in the document references, dropped
	// because ADF has no definition construct to hold it. A definition
	// something DOES reference is not reported: its destination travels
	// to every use as an ordinary link href, so the page is unchanged and
	// only the write-it-once source form is lost. An unreferenced one has
	// nowhere for its destination to go, so the URL leaves the document —
	// the one lossy half, and the reason this code exists. One diagnostic
	// fires per unused definition, naming its label and destination.
	// Emitted by ToADF.
	CodeUnusedDefinitionDropped = "unused-definition-dropped"
	// CodeLinkDestinationDropped reports a link whose label kept no node
	// able to carry the mark, so its destination left the document: an
	// ADF link is a MARK, and a mark needs a node that can hold one. Two
	// shapes reach it, and the message says which:
	//
	//   - The label converted to nothing at all — "[![]()](https://home/)",
	//     an image with neither alt text nor a destination to name it
	//     after.
	//   - The label converted to an inline leaf that carries no marks:
	//     "[:mention[Jane]{#1}](https://home/)", and the same for :status,
	//     :emoji, :date, :placeholder, :extension and :media. These are
	//     real ADF nodes, so the label is visible on the page — only the
	//     destination is gone. adfast does NOT attach the mark to them
	//     instead: no Atlassian payload has ever been observed to mark
	//     one of these leaves (measured over 110MB of recorded Jira
	//     documents: every marks key sat on a text node, none on the
	//     7,453 bare inline leaves), so emitting one would ship a shape
	//     the product is likely to strip on write and would move the loss
	//     one hop later, past the point adfast can still report it. The
	//     author's remedy is to put text inside the link beside the leaf.
	//
	// A label that carries the destination in PART — an unlinked mention
	// between two linked words — is not reported: the href is still in
	// the document and still clickable, so nothing was dropped. Nor is a
	// linked image the asset store cannot place, as long as it has a
	// label: the picture drops (CodeUnresolvedAsset) but the label
	// carries the enclosing destination as its mark. A link with no
	// destination to lose ("[]()") is not reported either. One diagnostic
	// fires per emptied link, naming the href. Emitted by ToADF.
	//
	// A linked image whose picture DOES convert is not this: the
	// destination becomes the link mark on the media node (block and
	// store-resolvable inline forms, nothing lost) or the href of the
	// degraded link (external inline form, see
	// CodeInlineImageDegraded).
	CodeLinkDestinationDropped = "link-destination-dropped"
	// CodeListItemContent reports a block inside a list item that ADF's
	// listItem content model does not allow. The pinned schema oracle
	// (docs/adf-coverage.md:122) gives the model as
	//
	//	(paragraph | bulletList | orderedList | taskList | mediaSingle |
	//	 codeBlock | unsupportedBlock | extension)+
	//
	// so a blockquote, a table, a heading, a rule, a panel or a
	// mediaGroup in a list item is not representable, and markdown says
	// all of them. An extension IS representable there — the model above
	// includes it — so it never raises this diagnostic.
	//
	// Conversion output is UNCHANGED — the document encodes exactly as
	// written, and the submission succeeds. What changes is what the
	// product STORES: Confluence accepts the push, renders the page
	// correctly, and rewrites the offending subtree into a bodiless
	// com.atlassian.confluence.migration / legacy-content extension
	// (measured on a live page, 2026-08-26; see
	// confluence.ExpandLegacyContent, which reads that wrapper back).
	// adfast does not restructure the author's document to avoid it:
	// lifting the block out of the item changes what the document says,
	// and re-nesting it changes the structure the author chose.
	//
	// One diagnostic fires per DISTINCT offending kind — Diagnostic
	// carries no position, so a second identical sentence would locate
	// nothing.
	//
	// The consumer decides severity AND WHEN TO SAY IT: this describes
	// the document, not the push, so it fires on every encode. A consumer
	// that encodes only to compare should stay quiet (storysmith's
	// PagePlanner.noteLosses is the reference: it warns only when the
	// encoded body is the one being published). Emitted by ToADF, only
	// when a diagnostics sink is configured.
	CodeListItemContent = "list-item-content"
	// CodeBeforeEncodeFailed reports a BeforeEncode hook error downgraded
	// to a diagnostic by the infallible facade conversion.
	CodeBeforeEncodeFailed = "before-encode-failed"
	// CodeRawNode reports an unknown ADF node (adf.RawNode) reaching the
	// markdown projection; its content was projected through its first
	// child or the node was dropped. Emitted by FromADF.
	CodeRawNode = "raw-node"
	// CodeDecodeFailed reports a value that could not be decoded into an
	// ADF document at all (nil, an unsupported Go type, or malformed
	// JSON); the conversion produces empty output.
	CodeDecodeFailed = "decode-failed"
	// CodeFontSizeDropped reports a retired fontSize construct dropped to
	// plain text: no Atlassian product supports the fontSize mark (Jira
	// REST rejects it with INVALID_INPUT; Confluence strips it on save),
	// so adfast never produces it. On encode a :fontSize[text]{size}
	// directive unwraps to its inline text (no mark emitted); on decode a
	// legacy fontSize ADF mark decodes to bare text. The text is always
	// preserved; only the size annotation is lost. Emitted by ToADF,
	// Normalize (the prettier formatter), and FromADF.
	CodeFontSizeDropped = "fontsize-dropped"
	// CodeJQLDegraded reports a ::jql directive that could not become a
	// JQL-datasource blockCard because the directive did not carry both
	// of the attributes ADF addresses the datasource by: cloudId
	// (parameters.cloudId) and datasource (datasource.id). ADF has no
	// bare-query card, and adfast cannot invent a cloud id, so the
	// QUERY DEGRADES TO A PLAIN PARAGRAPH: the query text stays in the
	// document and only the "render this as a live table" intent is
	// lost. The md → ADF → md round trip therefore returns the prose,
	// not the directive.
	//
	// A directive with an EMPTY query ("::jql[]") has no text to keep,
	// so it drops outright — the one case where the diagnostic is the
	// only trace left, and the reason it fires even when nothing is
	// salvageable. The message says which of the two happened.
	//
	// One diagnostic fires per degraded directive. Emitted by ToADF and
	// by Normalize (the prettier formatter).
	CodeJQLDegraded = "jql-degraded"
	// CodeSmartLinkDegraded reports a ::linkCard or ::linkEmbed directive
	// that could not become its ADF card because the label resolved to no
	// URL — a label with no text, or a SmartLinks resolver that maps the
	// key to the empty string. ADF addresses both blockCard and embedCard
	// by url and neither has a URL-less variant, so the LABEL DEGRADES TO
	// A PLAIN PARAGRAPH: the text the author wrote stays in the document
	// and only the "render this as a card" intent is lost. The md → ADF →
	// md round trip therefore returns that text, not the directive.
	//
	// A directive whose label has NO text ("::linkCard[]") has nothing to
	// keep, so it drops outright — the one case where the diagnostic is
	// the only trace left, and the reason it fires even when nothing is
	// salvageable. The message says which of the two happened, and which
	// of the two directives it was.
	//
	// One diagnostic fires per degraded directive. Emitted by ToADF and
	// by Normalize (the prettier formatter).
	CodeSmartLinkDegraded = "smartlink-degraded"

	// CodeUnknownNode re-exports adf.CodeUnknownNode: an ADF node type
	// the typed model does not know, kept losslessly as a RawNode.
	CodeUnknownNode = adf.CodeUnknownNode
	// CodeUnknownMark re-exports adf.CodeUnknownMark: an ADF mark type
	// the typed model does not know, kept losslessly as a RawMark.
	CodeUnknownMark = adf.CodeUnknownMark
	// CodeUnknownAttr re-exports adf.CodeUnknownAttr: an attribute a
	// known kind's typed fields do not model, kept in Extra.
	CodeUnknownAttr = adf.CodeUnknownAttr
	// CodeDepthExceeded re-exports adf.CodeDepthExceeded: input nested
	// deeper than a recursion cap; deeper content is truncated. Emitted
	// by the ADF decode codec and the facade markdown parse.
	CodeDepthExceeded = adf.CodeDepthExceeded
)

// codes is the inventory of the vocabulary above, in the declaration order
// of the constants — the const block is the reading order of the doc
// comments, and keeping the two aligned makes an omission visible in the
// diff rather than only in the test.
//
// diagnostics_test.go re-derives this list by parsing the source of this
// package (and of adf, for the re-exports) and fails if the two disagree,
// so a code added to the const block without a line here breaks the build
// of the package that OWNS the vocabulary, instead of silently shrinking
// the inventory a consumer trusted to be complete.
var codes = []string{
	CodeColwidthsOrphan,
	CodeDecisionsOrphan,
	CodeParseRecovered,
	CodeMalformedFrontmatter,
	CodeSpanMarkerInvalid,
	CodeUnresolvedAsset,
	CodeUnsupportedCodeLanguage,
	CodeUnsupportedInProduct,
	CodeHeadingAnchorDropped,
	CodeInlineImageDegraded,
	CodeFootnoteFlattened,
	CodeUnusedDefinitionDropped,
	CodeLinkDestinationDropped,
	CodeListItemContent,
	CodeBeforeEncodeFailed,
	CodeRawNode,
	CodeDecodeFailed,
	CodeFontSizeDropped,
	CodeJQLDegraded,
	CodeSmartLinkDegraded,

	CodeUnknownNode,
	CodeUnknownMark,
	CodeUnknownAttr,
	CodeDepthExceeded,
}

// Codes returns every diagnostic code a Diagnostic produced anywhere in
// this library can carry, including the codes re-exported from adf. The
// order is the declaration order of the Code* constants, and the returned
// slice is a fresh copy, so a caller may sort or filter it in place.
//
// It exists for the consumer that must handle EVERY code — classify each
// one as a real loss, a retry-later, or an informational notice, or route
// it to a severity — and wants the compiler and its test suite to prove it
// missed none. Key that table by the Code* CONSTANTS rather than by the
// string literals: a renamed code is then a compile error on the consumer's
// side, and a code adfast ADDS is a missing key this inventory reveals:
//
//	for _, code := range convert.Codes() {
//		if _, ok := severityOf[code]; !ok {
//			t.Errorf("adfast diagnostic code %q is unclassified", code)
//		}
//	}
//
// Such a test is meant to fail on an adfast upgrade that adds a code —
// that is the point of it — so put the check in a test and not on a
// request path. The inventory carries no severity of its own: what a code
// MEANS for a document is in the doc comment on its constant, and what to
// DO about it is the consumer's policy (see CodeUnsupportedInProduct and
// CodeListItemContent, which say so explicitly).
func Codes() []string { return slices.Clone(codes) }

// fontSizeDroppedMessage is the shared message for CodeFontSizeDropped,
// emitted identically on every path that retires a fontSize construct.
const fontSizeDroppedMessage = "fontSize dropped: no Atlassian product supports it (text kept, size lost)"

// jqlDegradedMessage is the shared message for CodeJQLDegraded, emitted
// identically by ToADF and by Normalize. It names the query, so a reader
// can find the directive the report is about, and says which of the two
// outcomes happened — an empty query has no text to keep.
func jqlDegradedMessage(query string) string {
	const why = "::jql needs both cloudId and datasource to become an ADF datasource card"
	if query == "" {
		return why + "; the directive carries no query either, so it drops entirely"
	}
	return fmt.Sprintf("%s; query %q kept as plain text (live table lost)", why, query)
}

// smartLinkDegradedMessage is the shared message for
// CodeSmartLinkDegraded, emitted identically by ToADF and by Normalize.
// It names the directive and its label, so a reader can find the line the
// report is about, and says which of the two outcomes happened — a label
// with no text has nothing to keep.
func smartLinkDegradedMessage(kind, label string) string {
	why := fmt.Sprintf("::%s needs a label that resolves to a URL to become an ADF card", kind)
	if label == "" {
		return why + "; the directive carries no label text either, so it drops entirely"
	}
	return fmt.Sprintf("%s; label %q kept as plain text (card lost)", why, label)
}
