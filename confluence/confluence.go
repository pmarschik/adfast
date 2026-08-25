// Package confluence provides the Confluence-specific ADF addons for
// github.com/pmarschik/adfast: page smart links matching Confluence
// Cloud's URL conventions, the code block macro language set, named
// directives for the core Confluence macros (see Macros), and a repair
// for what Confluence's ADF page read loses (see RepairReadBack).
package confluence

import (
	"regexp"
	"strings"

	"github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

// pageURLRe matches Confluence Cloud page view URLs
// (https://site.atlassian.net/wiki/spaces/KEY/pages/123456789/Page+Title)
// and captures the space key and the numeric page id. The trailing
// title slug is optional — Confluence resolves the URL without it.
var pageURLRe = regexp.MustCompile(`/wiki/spaces/([^/]+)/pages/(\d+)\b`)

// pageKeyRe matches a label that is exactly a page key (SPACE/pageID).
// The space part must not contain whitespace or slashes.
var pageKeyRe = regexp.MustCompile(`^([A-Za-z0-9~][A-Za-z0-9~:._-]*)/(\d+)$`)

// SmartLinks returns the Confluence URL-scheme resolver: page view URLs
// map to a "SPACE/pageID" key (e.g. "DOCS/123456789"), and such keys
// expand back to page URLs under baseURL (key expansion is disabled
// when baseURL is empty).
//
// The key deliberately carries the space key and page id, NOT the page
// title: the title slug in a Confluence URL is mutable display text
// (the URL resolves without it), while space key + page id are stable
// and reconstruct the URL exactly — the same role Jira issue keys play
// for jira.SmartLinks. Keys derived from URLs whose slug encodes a
// title therefore stay valid when the page is renamed. The separator
// is "/" (mirroring the URL's spaces/KEY/pages/ID order) because a ":"
// inside a directive label (::linkCard[DOCS:123]) reads as directive
// syntax and would require escaping.
func SmartLinks(baseURL string) convert.SmartLinks {
	sl := convert.SmartLinks{
		KeyFromURL: func(url string) (string, bool) {
			if m := pageURLRe.FindStringSubmatch(url); m != nil {
				return m[1] + "/" + m[2], true
			}
			return "", false
		},
	}
	if baseURL != "" {
		base := strings.TrimRight(baseURL, "/")
		sl.URLForKey = func(key string) (string, bool) {
			m := pageKeyRe.FindStringSubmatch(key)
			if m == nil {
				return "", false
			}
			return base + "/wiki/spaces/" + m[1] + "/pages/" + m[2], true
		}
	}
	return sl
}

// UnsupportedKinds lists the ADF node and mark kinds Confluence Cloud
// does not preserve, sourced from docs/adf-availability.json (entries
// with confluence == "no"). Confluence silently strips or downgrades
// unsupported kinds on save, so a live round-trip is a reliable oracle:
// a full probe (2026-07-22, every node + mark written to a Confluence
// page and read back) found the fontSize mark STRIPPED on save and
// blockTaskItem DOWNGRADED to a plain taskItem (its block body flattened
// to inline). Only blockTaskItem is flagged here:
//   - blockTaskItem: DOWNGRADED to a plain taskItem — the distinct kind
//     is not preserved.
//
// fontSize is NOT listed: Confluence strips the mark, but adfast retires
// it entirely (it never produces a fontSize mark — the directive drops to
// plain text with a fontsize-dropped diagnostic), so an
// unsupported-in-product check for it is moot.
//
// Everything else survived, including the previously-inconclusive
// dataConsumer and fragment marks, plus multiBodiedExtension,
// extensionFrame, syncBlock, and bodiedSyncBlock. The deprecated
// confluence-schema.ts is not used (its omissions are evidence-by-
// omission only). Add a kind here only when a live round-trip confirms
// Confluence strips or downgrades it.
var UnsupportedKinds = []string{"blockTaskItem"}

// MarkdownOptions bundles the Confluence behavior for
// adfast.FromMarkdown: smart-link recognition for page URLs (a link
// whose text equals the derived SPACE/pageID key encodes as an
// inlineCard; bare ::linkCard[SPACE/pageID] labels expand under
// baseURL), the code-block language check against CodeLanguages (an
// "unsupported-code-language" diagnostic when a diagnostics sink is
// configured; conversion is unchanged), and the product-availability
// check against UnsupportedKinds (blockTaskItem; see UnsupportedKinds).
// It also lowers the two constructs ADF has no attribute for, which is
// what makes the encoded document wire-safe: a heading's "{#id}" suffix
// becomes the anchor macro Confluence stores (see LowerAnchors), and a
// table's column alignment becomes the alignment block mark on the
// blocks in each aligned column (see adf.LowerTableAlign).
//
// The facade shares one option type, so these compose with RenderOptions
// and pass to any primitive or to adfast.WithPipelineOptions; each
// primitive reads the subset that applies to it.
func MarkdownOptions(baseURL string) []adfast.Option {
	return []adfast.Option{
		adfast.WithSmartLinks(SmartLinks(baseURL)),
		adfast.WithCodeLanguages(CodeLanguages),
		adfast.WithUnsupportedKinds("confluence", UnsupportedKinds),
		adfast.WithExtensions(Macros()),
		adfast.WithDocTransforms(LowerAnchors, adf.LowerTableAlign),
	}
}

// RenderOptions bundles the Confluence behavior for the decode direction
// (adfast.FromADF): inline and block smart-link cards pointing at
// Confluence pages label with the SPACE/pageID key, the core macros
// decode to their sugared directives (see Macros), a heading's anchor
// macro lifts back to its "{#id}" suffix (see LiftAnchors), and a table
// column whose cells all carry the same alignment mark lifts back to a
// GFM delimiter row (see adf.LiftTableAlign). See MarkdownOptions for
// the encode side.
func RenderOptions() []adfast.Option {
	return []adfast.Option{
		adfast.WithSmartLinks(SmartLinks("")),
		adfast.WithExtensions(Macros()),
		adfast.WithADFTransforms(LiftAnchors, adf.LiftTableAlign),
	}
}
