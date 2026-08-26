package confluence

import (
	"encoding/json"

	"github.com/pmarschik/adfast/adf"
)

// Legacy content: the wrapper Confluence stores for a construct that
// ADF's own content model forbids.
//
// ADF gives listItem the content model
//
//	(paragraph | mediaSingle | codeBlock)
//	(paragraph | bulletList | orderedList | mediaSingle | codeBlock | taskList)*
//
// so a blockquote or a table inside a list item is not representable.
// Confluence accepts the submission anyway and rewrites it on save: the
// offending subtree becomes a bodiless extension
//
//	{"type": "extension", "attrs": {
//	  "extensionType": "com.atlassian.confluence.migration",
//	  "extensionKey": "legacy-content",
//	  "parameters": {"cxhtml": "<the original storage format>",
//	                 "nestedContent": {"type": "doc", "version": 1, …}}}}
//
// measured on a live page (read-only, 2026-08-26): a bodiless extension
// nested inside a listItem, replacing the blockquote the content model
// would not allow there; parameters held exactly cxhtml and
// nestedContent, and nestedContent was a JSON object — {"type": "doc",
// "version": 1, "content": [...]}, not an escaped string.
//
// The page still RENDERS, because the extension carries the cxhtml. The
// damage is on the read: without this pass the node decodes through the
// generic ::extension directive, carrying a screenful of escaped HTML
// and JSON, so the remote side of a comparison never matches the local
// document and no push ever settles it.
//
// nestedContent is preferred over cxhtml because it is already ADF: it
// needs no storage-format parser, and it is the same document the
// submission carried.
//
// This is an expansion and not a conversion. It replaces one node with
// the content that node was standing in for, and it removes nothing:
// anything the payload cannot supply — an absent, unparsable, empty, or
// non-version-1 nestedContent — leaves the extension exactly where it
// was, so the fallback is the behavior that shipped before this pass.

// legacyContentType and legacyContentKey identify the wrapper.
const (
	legacyContentType = "com.atlassian.confluence.migration"
	legacyContentKey  = "legacy-content"
)

// maxLegacyNesting caps how many wrappers deep the expansion follows. A
// nestedContent payload is a whole document and may in principle hold a
// wrapper of its own, so the pass recurses; the cap is what keeps a
// self-referential payload from growing the stack without bound. The
// decoder's own maxDecodeDepth does not bound this, because each
// nestedContent is a fresh decode starting at depth zero. At the cap the
// innermost wrapper is left in place, which is the same fallback every
// other decline takes.
const maxLegacyNesting = 8

// ExpandLegacyContent replaces every Confluence legacy-content extension
// with the ADF document its nestedContent parameter carries, in the
// position the extension held. A document with no such extension is
// returned unchanged, sharing its subtrees with the input.
//
// RenderOptions installs it as an ADF transform, first in the list so
// the lifts that follow see the expanded content; supply it directly
// (adfast.WithADFTransforms(confluence.ExpandLegacyContent)) when
// composing options by hand.
func ExpandLegacyContent(doc adf.Doc) adf.Doc {
	return expandLegacyContentDepth(doc, 0)
}

// expandLegacyContentDepth is ExpandLegacyContent's recursive worker. The
// name deliberately does not differ from ExpandLegacyContent by
// capitalization alone — revive's confusing-naming check flags that within
// a single file — so it carries the extra "Depth" to name what it adds
// over the exported wrapper: the recursion budget.
func expandLegacyContentDepth(doc adf.Doc, depth int) adf.Doc {
	if depth >= maxLegacyNesting {
		return doc
	}
	return adf.Transform(doc, func(n adf.Node) ([]adf.Node, bool) {
		ext, ok := n.(*adf.Extension)
		if !ok || ext.ExtensionType != legacyContentType || ext.ExtensionKey != legacyContentKey {
			return nil, false
		}
		nested, ok := nestedContentDoc(ext.Parameters)
		if !ok || len(nested.Content) == 0 {
			// Nothing usable in the payload: keep the extension, and let it
			// decode through the generic ::extension directive as before.
			// An empty replacement would delete the content the wrapper
			// stands for, which is worse than showing the wrapper.
			return nil, false
		}
		return expandLegacyContentDepth(nested, depth+1).Content, true
	})
}

// nestedContentDoc reads the nestedContent parameter as an ADF document.
//
// The measured page (see the package comment) carries it as a JSON
// object. The escaped-string form below is defensive and unmeasured on
// the wire — cxhtml, the sibling parameter, does carry escaped JSON, so
// nestedContent doing the same on some page is plausible even though no
// observed case exercises it; accepting it costs one type switch case.
//
// The type and version are checked on the RAW map: adf.DecodeDoc
// normalizes every document it decodes to type "doc" version 1 and
// discards what the input said, so a check on the decoded Doc would
// pass anything. Version 1 is the only version adfast models, and a
// future version could spell a node differently enough that decoding it
// as version 1 would be a silent misreading rather than a decode
// failure.
func nestedContentDoc(parameters any) (adf.Doc, bool) {
	params, ok := parameters.(map[string]any)
	if !ok {
		return adf.Doc{}, false
	}
	var raw map[string]any
	switch v := params["nestedContent"].(type) {
	case map[string]any:
		raw = v
	case string:
		if json.Unmarshal([]byte(v), &raw) != nil {
			return adf.Doc{}, false
		}
	default:
		return adf.Doc{}, false
	}
	if t, ok := raw["type"].(string); !ok || t != "doc" {
		return adf.Doc{}, false
	}
	if version, isNum := raw["version"].(float64); !isNum || version != 1 {
		return adf.Doc{}, false
	}
	return adf.DecodeDoc(raw)
}
