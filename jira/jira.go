// Package jira provides the Jira-specific ADF addons for
// github.com/pmarschik/adfast: issue-key smart links and document
// transforms matching Jira's link conventions.
package jira

import (
	"regexp"
	"strings"

	"github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

var (
	jiraBrowseKeyReTransform = regexp.MustCompile(`/browse/([A-Z][A-Z0-9]+-\d+)\b`)
	bareIssueKeyRe           = regexp.MustCompile(`\b([A-Z][A-Z0-9]+-\d+)\b`)
	trailingPunctRe          = regexp.MustCompile(`^([.,;:!?\]\)]+)`)
)

// ConvertIssueLinksToInlineCards converts text nodes with Jira browse links
// into inlineCard nodes.
func ConvertIssueLinksToInlineCards(doc adf.Doc) adf.Doc {
	return adf.Transform(doc, func(n adf.Node) ([]adf.Node, bool) {
		if text, ok := n.(*adf.Text); ok {
			if replaced, ok := convertLinkNode(text); ok {
				return replaced, true
			}
		}
		return nil, false
	})
}

// convertLinkNode tries to convert a text node with a Jira browse link to
// inlineCard nodes. Returns the replacement nodes and true if converted.
func convertLinkNode(child *adf.Text) ([]adf.Node, bool) {
	linkMark, ok := adf.FindMark[*adf.Link](child.Marks)
	if !ok {
		return nil, false
	}
	if linkMark.Href == nil || *linkMark.Href == "" {
		return nil, false
	}
	href := *linkMark.Href
	m := jiraBrowseKeyReTransform.FindStringSubmatch(href)
	if m == nil {
		return nil, false
	}
	key := m[1]
	suffixRe := regexp.MustCompile(`^` + regexp.QuoteMeta(key) + `([\]\)\.\,;:]*)$`)
	sm := suffixRe.FindStringSubmatch(child.Text)
	if sm == nil {
		return nil, false
	}
	result := []adf.Node{&adf.InlineCard{URL: &href}}
	if suffix := sm[1]; suffix != "" {
		result = append(result, &adf.Text{Text: suffix})
	}
	return result, true
}

// ExpandMode selects how bare issue keys in markdown text expand to
// inlineCards during encoding. The underlying strings are stable
// configuration identifiers.
type ExpandMode string

const (
	// ExpandAuto expands bare issue keys found in plain text (the
	// default behavior when a base URL is configured).
	ExpandAuto ExpandMode = "auto"
	// ExpandAll expands bare issue keys everywhere auto does; it exists
	// as a distinct configuration value and currently behaves like
	// ExpandAuto.
	ExpandAll ExpandMode = "all"
	// ExpandExplicit disables bare-key expansion: only explicit links
	// and smart-link directives become cards.
	ExpandExplicit ExpandMode = "explicit"
)

// ExpandBareIssueKeys converts bare issue keys in plain text nodes into
// inlineCard nodes. ExpandExplicit returns the document unchanged.
func ExpandBareIssueKeys(doc adf.Doc, baseURL string, mode ExpandMode) adf.Doc {
	if mode == ExpandExplicit {
		return doc
	}
	normalizedURL := strings.TrimRight(baseURL, "/")
	return adf.Transform(doc, func(n adf.Node) ([]adf.Node, bool) {
		// Only expand bare text nodes (not already links or inline code).
		if textNode, ok := n.(*adf.Text); ok &&
			!adf.HasMark(textNode.Marks, "link") && !adf.HasMark(textNode.Marks, "code") {
			return expandKeysInText(textNode.Text, normalizedURL)
		}
		// Don't expand inside code blocks: keep them and prune.
		if _, ok := n.(*adf.CodeBlock); ok || n.Kind() == "code" {
			return []adf.Node{n}, true
		}
		return nil, false
	})
}

// expandKeysInText splits one text value around its bare issue keys into
// a replacement run (text, inlineCard, trailing punctuation, …); ok is
// false when the text holds no key.
func expandKeysInText(text, baseURL string) ([]adf.Node, bool) {
	matches := bareIssueKeyRe.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return nil, false
	}
	var out []adf.Node
	lastIndex := 0
	for _, loc := range matches {
		key := text[loc[2]:loc[3]]
		matchIndex := loc[0]

		// Add text before match
		if matchIndex > lastIndex {
			out = append(out, &adf.Text{Text: text[lastIndex:matchIndex]})
		}

		// Check for trailing punctuation
		afterMatch := text[loc[1]:]
		punct := ""
		if pm := trailingPunctRe.FindStringSubmatch(afterMatch); pm != nil {
			punct = pm[1]
		}

		// Add inline card
		url := baseURL + "/browse/" + key
		out = append(out, &adf.InlineCard{URL: &url})

		if punct != "" {
			out = append(out, &adf.Text{Text: punct})
		}

		lastIndex = loc[1] + len(punct)
	}

	// Add remaining text
	if lastIndex < len(text) {
		out = append(out, &adf.Text{Text: text[lastIndex:]})
	}
	return out, true
}

// browseKeyRe matches Jira issue browse URLs and captures the issue key.
var browseKeyRe = regexp.MustCompile(`/browse/([A-Z][A-Z0-9]+-\d+)\b`)

// bareKeyLabelRe matches a label that is exactly a bare issue key.
var bareKeyLabelRe = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d+$`)

// SmartLinks returns the Jira URL-scheme resolver: issue browse URLs map
// to their issue key, and bare keys expand under baseURL (key expansion
// is disabled when baseURL is empty).
func SmartLinks(baseURL string) convert.SmartLinks {
	sl := convert.SmartLinks{
		KeyFromURL: func(url string) (string, bool) {
			if m := browseKeyRe.FindStringSubmatch(url); m != nil {
				return m[1], true
			}
			return "", false
		},
	}
	if baseURL != "" {
		base := strings.TrimRight(baseURL, "/")
		sl.URLForKey = func(key string) (string, bool) {
			if !bareKeyLabelRe.MatchString(key) {
				return "", false
			}
			return base + "/browse/" + key, true
		}
	}
	return sl
}

// UnsupportedKinds lists the ADF kinds a Jira-targeted authoring flow
// should flag. Wired into MarkdownOptions via adfast.WithUnsupportedKinds
// so a Jira-targeted encode emits an "unsupported-in-product" diagnostic
// naming each offending kind; the consumer decides severity.
//
// Scope: RENDER-CONFIRMED non-support only. Jira's documentation is
// non-exhaustive (404-by-omission is unreliable) and its REST endpoint
// accepts most of the shared ADF schema, so a full live probe
// (2026-07-22, every node + mark written to a Jira issue description and
// inspected in the browser) is the oracle. It showed Jira actually
// renders the overwhelming majority of kinds documentation omits —
// including layoutSection/layoutColumn, blockCard/embedCard, status,
// taskList/decisionList, the extension family, syncBlock, and the
// alignment/indentation/breakout/annotation/fragment/dataConsumer marks,
// all first-class or degraded-but-present. Only three kinds are confirmed
// unavailable and flagged here:
//   - placeholder: rendered as an empty <span> (content DROPPED);
//   - multiBodiedExtension, extensionFrame: rejected outright by the Jira
//     REST endpoint (INVALID_INPUT) — not in Jira's ADF schema, so a push
//     carrying them fails entirely.
//
// fontSize is NOT listed: Jira rejects the mark, but adfast retires it
// entirely (it never produces a fontSize mark — the directive drops to
// plain text with a fontsize-dropped diagnostic), so an
// unsupported-in-product check for it is moot.
//
// Docs-by-omission entries Jira actually renders are deliberately NOT
// flagged (they would false-positive). Add a kind here only when a live
// Jira render confirms it is dropped/shown as an unsupported-content
// block, or the REST endpoint rejects it. Kind strings are the ADF type
// strings, which equal the adf.Node/Mark Kind() values the document walk
// matches against.
var UnsupportedKinds = []string{
	"placeholder",          // Jira live render 2026-07-22: content dropped (empty span)
	"multiBodiedExtension", // Jira REST 2026-07-22: INVALID_INPUT (not in Jira schema)
	"extensionFrame",       // Jira REST 2026-07-22: rejected with its multiBodiedExtension parent
}

// MarkdownOptions bundles the Jira behavior for the encode direction
// (adfast.ToADF): smart-link recognition, issue-link→inlineCard
// conversion, the code-block language check against CodeLanguages (an
// "unsupported-code-language" diagnostic when a diagnostics sink is
// configured; conversion is unchanged), the product-availability check
// against UnsupportedKinds (an "unsupported-in-product" diagnostic for
// each Confluence-only construct, again diagnostic-only), heading-anchor
// dropping (Jira has no anchor construct at all — a "## Title {#id}"
// suffix cannot survive, so the anchor drops with a
// "heading-anchor-dropped" diagnostic and the heading text is kept), and
// — when keyExpansion is not ExpandExplicit and baseURL is set — bare
// issue-key expansion (ExpandAuto or ExpandAll).
//
// The facade shares one option type, so these compose with RenderOptions
// and pass to any primitive or to adfast.WithPipelineOptions; each
// primitive reads the subset that applies to it.
func MarkdownOptions(baseURL string, keyExpansion ExpandMode) []adfast.Option {
	opts := []adfast.Option{
		adfast.WithSmartLinks(SmartLinks(baseURL)),
		adfast.WithDocTransforms(ConvertIssueLinksToInlineCards),
		adfast.WithCodeLanguages(CodeLanguages),
		adfast.WithUnsupportedKinds("jira", UnsupportedKinds),
		adfast.WithoutHeadingAnchors("jira"),
	}
	if keyExpansion != ExpandExplicit && baseURL != "" {
		opts = append(opts, adfast.WithDocTransforms(func(d adf.Doc) adf.Doc {
			return ExpandBareIssueKeys(d, baseURL, keyExpansion)
		}))
	}
	return opts
}

// RenderOptions bundles the Jira behavior for the decode direction
// (adfast.FromADF): inline and block smart-link cards label with the bare
// issue key. See MarkdownOptions for the encode side.
func RenderOptions() []adfast.Option {
	return []adfast.Option{adfast.WithSmartLinks(SmartLinks(""))}
}
