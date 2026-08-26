package convert

import "github.com/pmarschik/adfast/adf"

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
	// CodeUnresolvedAsset reports an ![alt](assets/…) reference the
	// configured asset store could not map back to a media id; the image
	// is kept as external media. Emitted by ToADF.
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

// fontSizeDroppedMessage is the shared message for CodeFontSizeDropped,
// emitted identically on every path that retires a fontSize construct.
const fontSizeDroppedMessage = "fontSize dropped: no Atlassian product supports it (text kept, size lost)"
