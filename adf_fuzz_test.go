package adfast

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/pmarschik/adfast/adf"
)

// Seed corpus entries covering interesting structural patterns.
var fuzzSeeds = []string{
	"# Heading\n\nParagraph text.",
	"- item 1\n- item 2\n  - nested",
	"1. first\n2. second\n3. third",
	"```go\nfmt.Println(\"hello\")\n```",
	"> blockquote\n> second line",
	":::note\nBe careful\n:::",
	"| A | B |\n| --- | --- |\n| 1 | 2 |",
	// table column alignment (the synthetic never-wire carrier): every
	// alignment, and a column narrower than its own colons
	"| A | B | C | D |\n|:-|-:|:-:|-|\n| 1 | 2 | 3 | 4 |",
	"| a |\n|:-:|\n| bbbb |",
	"Some **bold** and _italic_ and ~~strike~~ text.",
	"Use `code` here and [link](https://example.com).",
	// escape sequences
	"\\`backtick\\` and \\_underscore\\_",
	// colon-after-backtick (the remark-directive trap)
	"\\`bisearch:partner-sync-opt-out\\`",
	// autolinks
	"See <https://example.com> for details.",
	// task list
	"- [x] done\n- [ ] todo",
	// hard break
	"line one\\\nline two",
	// empty and whitespace
	"",
	"   \n  \n  ",
	// YAML frontmatter
	"---\nstatus: Ready\n---\n\n# Title",
	// soft line break (word-wrap)
	"A long sentence that wraps\nacross two lines in the source.",
	// multiple blank lines
	"para one\n\n\n\npara two",
	// ordered list loose (blank lines between items)
	"1. first\n\n2. second\n\n3. third",
	// mixed marks
	"**bold with `code` inside**",
	// extended dialect: date/placeholder/emoji/annotation/fontSize
	"due :date[2026-07-15]{timestamp=\"1784073600000\"} soon",
	":placeholder[Type something]",
	":emoji{#abc shortName=\":team_logo:\"}",
	":annotation[noted **bold**]{#a1 annotationType=\"inlineComment\"}",
	":fontSize[fine print]{small}",
	// extended dialect: extension family + sync blocks
	":extension{key=\"k\" parameters='{\"a\":1}' type=\"t\"}",
	"::extension{key=\"k\" layout=\"wide\" type=\"t\"}",
	":::extension{key=\"k\" type=\"t\"}\nbody\n:::",
	"::::extension{key=\"k\" type=\"t\"}\n:::frame\none\n:::\n\n:::frame\ntwo\n:::\n::::",
	"::syncBlock{localId=\"l\" resourceId=\"r\"}",
	// extended dialect: layouts, wrappers, captions, block task items
	"::::section\n:::column{width=\"50\"}\nleft\n:::\n\n:::column{width=\"50\"}\nright\n:::\n::::",
	":::center\ncentered\n:::",
	"::::indent{2}\n:::end\nboth\n:::\n::::",
	":::breakout{wide}\n```\ncode\n```\n:::",
	":::fragment{localId=\"f1\"}\n| a |\n| --- |\n| b |\n:::",
	":::dataConsumer{sources=\"f1\"}\n::extension{key=\"k\" type=\"t\"}\n:::",
	":::media[alt]{type=\"external\" url=\"https://example.com/i.png\"}\ncaption **text**\n:::",
	"![alt](https://example.com/i.png \"caption\")",
	"- [ ] head\n\n  more blocks\n- [x] done",
}

func FuzzMarkdownToAdf(f *testing.F) {
	for _, seed := range fuzzSeeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, md string) {
		doc := mdToADF(md)
		// Must produce valid serializable ADF.
		if _, err := json.Marshal(doc); err != nil {
			t.Errorf("json.Marshal: %v", err)
		}
		// Type and version must be set.
		if doc.Type != "doc" {
			t.Errorf("doc.Type = %q, want doc", doc.Type)
		}
		if doc.Version != 1 {
			t.Errorf("doc.Version = %d, want 1", doc.Version)
		}
	})
}

func FuzzToMarkdown(f *testing.F) {
	// Seed with ADF JSON blobs.
	adfSeeds := []string{
		`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"hello"}]}]}`,
		`{"type":"doc","version":1,"content":[{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Title"}]}]}`,
		`{"type":"doc","version":1,"content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"item"}]}]}]}]}`,
		`{"type":"doc","version":1,"content":[{"type":"codeBlock","attrs":{"language":"go"},"content":[{"type":"text","text":"x := 1"}]}]}`,
		`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"bold","marks":[{"type":"strong"}]}]}]}`,
		`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"code","marks":[{"type":"code"}]}]}]}`,
		// plain text with literal backtick (no code mark) — the INFRA-813 case
		"{\"type\":\"doc\",\"version\":1,\"content\":[{\"type\":\"bulletList\",\"content\":[{\"type\":\"listItem\",\"content\":[{\"type\":\"paragraph\",\"content\":[{\"type\":\"text\",\"text\":\"`bisearch:partner-sync-opt-out`\"}]}]}]}]}",
		`{}`,
		`{"type":"doc"}`,
		`null`,
	}
	for _, seed := range adfSeeds {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(_ *testing.T, data []byte) {
		var v map[string]any
		if err := json.Unmarshal(data, &v); err != nil {
			return // skip non-JSON
		}
		// Must not panic.
		_ = adfToMD(v)
	})
}

// FuzzRoundTripIdempotent verifies that two successive ADF round-trips produce
// the same result. This is the invariant relied on by IssueChangedFromJira's
// field normalization.
// setextAmbiguousLineRe matches a content line followed by a
// marker-only "-"/"=" continuation line (a setext underline on re-parse).
// unterminatedAngleDestRe matches a rendered inline-link destination that
// opens an angle bracket but never closes it before the closing paren —
// goldmark and remark both fail to re-parse these as links.
var unterminatedAngleDestRe = regexp.MustCompile(`\]\(<[^>\n]*\)`)

// hasAdjacentCodeSpans reports whether any content array holds two
// consecutive code-marked text nodes with the same mark signature — their
// rendered backtick fences would abut and merge on re-parse.
func hasAdjacentCodeSpans(nodes []adf.Node) bool {
	for i := range nodes {
		if i+1 < len(nodes) && isCodeText(nodes[i]) && isCodeText(nodes[i+1]) && sameMarkSignature(nodes[i], nodes[i+1]) {
			return true
		}
		if hasAdjacentCodeSpans(adf.NodeContent(nodes[i])) {
			return true
		}
	}
	return false
}

var linkDigitDirectiveRe = regexp.MustCompile(`:\d|^:[A-Za-z]`)

// hasDigitDirectiveInLink reports a link-marked text containing ":<digit>"
// or starting with ":<letter>". A link label is written with remark's
// label escaping, which leaves colons alone, so a directive token in a
// rendered label re-parses as a text directive and splits or empties the
// link each round; remark degrades identically (probes: [0:0:0](),
// [:u:A]()).
//
// The directive-form wrappers (:u/:sub/:color/:bg/:fontSize/:annotation)
// used to be skipped here too. They are fixed rather than skipped: their
// labels escape every colon that could open a nested text directive (see
// markdown.writeColonEscapePrefix). Probes: ":u[0:0:0]", ":sup[:a:b]".
func hasDigitDirectiveInLink(nodes []adf.Node) bool {
	for i := range nodes {
		if text, ok := nodes[i].(*adf.Text); ok && linkDigitDirectiveRe.MatchString(text.Text) {
			for _, m := range text.Marks {
				if m.Kind() == "link" {
					return true
				}
			}
		}
		if hasDigitDirectiveInLink(adf.NodeContent(nodes[i])) {
			return true
		}
	}
	return false
}

// hasUnlinkedURLText reports a run of unlinked text whose JOINED value
// contains a URL/www literal (see the unlinked-URL skip): rendering
// fuses adjacent text nodes, so the literal may only exist across the
// seams. Code blocks are verbatim and stay out of it.
func hasUnlinkedURLText(nodes []adf.Node) bool {
	var run strings.Builder
	flush := func() bool {
		defer run.Reset()
		return urlLiteralRe.MatchString(run.String())
	}
	for i := range nodes {
		if _, isCode := nodes[i].(*adf.CodeBlock); isCode {
			continue
		}
		if text, ok := nodes[i].(*adf.Text); ok {
			if !adf.HasMark(text.Marks, "link") && !adf.HasMark(text.Marks, "code") {
				run.WriteString(text.Text)
				continue
			}
		}
		if flush() {
			return true
		}
		if hasUnlinkedURLText(adf.NodeContent(nodes[i])) {
			return true
		}
	}
	return flush()
}

// hasLinkUnderMarkDirective reports a text node carrying both a link
// mark and a label-wrapping mark directive (see the skip above).
func hasLinkUnderMarkDirective(nodes []adf.Node) bool {
	for i := range nodes {
		marks := adf.NodeMarks(nodes[i])
		if adf.HasMark(marks, "link") {
			for _, kind := range []string{"underline", "subsup", "textColor", "backgroundColor", "annotation", "fontSize"} {
				if adf.HasMark(marks, kind) {
					return true
				}
			}
		}
		if hasLinkUnderMarkDirective(adf.NodeContent(nodes[i])) {
			return true
		}
	}
	return false
}

// hasAngleLeadingHref reports a link href beginning with a literal "<":
// the renderer emits "](<...)" which re-parses as an angle-bracket
// destination (or fails to parse), changing the href.
func hasAngleLeadingHref(nodes []adf.Node) bool {
	for i := range nodes {
		for _, m := range adf.NodeMarks(nodes[i]) {
			if link, ok := m.(*adf.Link); ok {
				if link.Href != nil && strings.HasPrefix(*link.Href, "<") {
					return true
				}
			}
		}
		if hasAngleLeadingHref(adf.NodeContent(nodes[i])) {
			return true
		}
	}
	return false
}

// hasAdjacentQuotesInItem reports two consecutive blockquote siblings
// inside a list item — they render without a separating blank line and
// merge into one quote on re-parse.
func hasAdjacentQuotesInItem(nodes []adf.Node, inItem bool) bool {
	for i := range nodes {
		if inItem && i+1 < len(nodes) && nodes[i].Kind() == "blockquote" && nodes[i+1].Kind() == "blockquote" {
			return true
		}
		if hasAdjacentQuotesInItem(adf.NodeContent(nodes[i]), nodes[i].Kind() == "listItem") {
			return true
		}
	}
	return false
}

// hasControlText reports decoded control characters in any text value or
// string attribute (fence info strings land in the language attr).
func hasControlText(nodes []adf.Node) bool {
	for i := range nodes {
		if controlString(adf.NodeText(nodes[i])) {
			return true
		}
		for _, v := range adf.NodeAttrs(nodes[i]) {
			if sv, ok := v.(string); ok && controlString(sv) {
				return true
			}
		}
		if hasControlText(adf.NodeContent(nodes[i])) {
			return true
		}
	}
	return false
}

func controlString(s string) bool {
	for _, r := range s {
		if (r < 0x20 && r != '\n' && r != '\t' && r != '\r') || r == 0x7f {
			return true
		}
	}
	return false
}

// hasMergingDigitPunct reports adjacent text siblings where the first ends
// in a digit and the next begins with '.' or ')': on re-parse the texts
// merge into one node whose digit-run escape decision differs from the
// split rendering; remark is equally unstable (probe: "* [X] 0\n\n  )").
func hasMergingDigitPunct(nodes []adf.Node) bool {
	for i := range nodes {
		if i+1 < len(nodes) && nodes[i].Kind() == "text" && nodes[i+1].Kind() == "text" {
			a, b := adf.NodeText(nodes[i]), adf.NodeText(nodes[i+1])
			if a != "" && a[len(a)-1] >= '0' && a[len(a)-1] <= '9' && digitPunctLeadRe.MatchString(b) {
				return true
			}
		}
		if hasMergingDigitPunct(adf.NodeContent(nodes[i])) {
			return true
		}
	}
	return false
}

// hasCodeBoundaryBreak reports an em/strong text followed by a bare code
// text (ADF drops those marks on code) with a hard break later in the same
// paragraph: the mark re-inference across the code boundary interacts with
// the break and re-nests differently each round; remark renders identical
// bytes and is equally unstable (probe: "*0\x600\x60* \\<break>0").
func hasCodeBoundaryBreak(nodes []adf.Node) bool {
	for i := range nodes {
		if i+1 < len(nodes) && hasEmphasisMark(nodes[i]) && isCodeText(nodes[i+1]) && !hasEmphasisMark(nodes[i+1]) {
			for j := i + 2; j < len(nodes); j++ {
				if nodes[j].Kind() == "hardBreak" {
					return true
				}
			}
		}
		if hasCodeBoundaryBreak(adf.NodeContent(nodes[i])) {
			return true
		}
	}
	return false
}

func hasEmphasisMark(n adf.Node) bool {
	for _, m := range adf.NodeMarks(n) {
		if m.Kind() == "em" || m.Kind() == "strong" {
			return true
		}
	}
	return false
}

// hasTabText reports a tab character in any inline text value.
func hasTabText(nodes []adf.Node) bool {
	for i := range nodes {
		if strings.Contains(adf.NodeText(nodes[i]), "\t") {
			return true
		}
		if hasTabText(adf.NodeContent(nodes[i])) {
			return true
		}
	}
	return false
}

func isCodeText(n adf.Node) bool {
	text, ok := n.(*adf.Text)
	if !ok {
		return false
	}
	return adf.HasMark(text.Marks, "code")
}

func sameMarkSignature(a, b adf.Node) bool {
	return bytes.Equal(mustJSON(adf.NodeMarks(a)), mustJSON(adf.NodeMarks(b)))
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// digitDirectiveTokenRe matches a rendered ":<digit-led name>" followed by
// an attribute/label opener or an emphasis marker. Colon escaping protects
// letter-led names, but remark-directive also accepts digit-led ones,
// which re-parse as a directive and drop braces or shift emphasis
// boundaries between rounds (probes: ":0{}00{}", ":0-*0!*").
var digitDirectiveTokenRe = regexp.MustCompile(`:\d[A-Za-z0-9-]*[{\[_*~]`)

// emailTokenRe finds email-like tokens in rendered text; used with
// gfmEmailInvalid to skip goldmark/micromark parse-order divergences.
var emailTokenRe = regexp.MustCompile(`[a-zA-Z0-9.+_-]+@[a-zA-Z0-9._-]+`)

// gfmEmailInvalid reports an email-like token micromark would NOT linkify
// (no dot in the domain or a non-letter final character). goldmark's
// linkify matches these before attention resolution, so the two parsers
// give different inline structure (probe: "0+_@0._0_!_" — remark reads
// "_@0.0!_" as emphasis where goldmark's dead email match eats the "_").
func gfmEmailInvalid(md string) bool {
	for _, m := range emailTokenRe.FindAllString(md, -1) {
		at := strings.LastIndexByte(m, '@')
		labels := strings.Split(m[at+1:], ".")
		valid := len(labels) >= 2
		for _, l := range labels {
			if l == "" {
				valid = false
			}
		}
		last := m[len(m)-1]
		if (last < 'a' || last > 'z') && (last < 'A' || last > 'Z') {
			valid = false
		}
		// An '_' in the token can pair with another '_' (inside or after
		// the token) as emphasis in micromark, where attention wins over
		// the literal, while goldmark's linkify consumes the token first —
		// parser divergence (probes: "0+_@0.A!_", "0+_@_.A").
		if strings.Contains(m, "_") && strings.Count(md, "_") >= 2 {
			valid = false
		}
		if !valid {
			return true
		}
	}
	// The same attention-vs-literal hazard for URL literals
	// (probe: "*http://*.a" — remark keeps the emphasis, goldmark links).
	for _, m := range urlLiteralRe.FindAllString(md, -1) {
		if strings.Contains(m, "_") && strings.Count(md, "_") >= 2 {
			return true
		}
	}
	return false
}

// urlLiteralRe matches GFM literal-autolink URLs (a copy of the markdown
// package's linkify pattern; the path part optional).
var urlLiteralRe = regexp.MustCompile(
	"(?:(?:https?|ftp)://|www\\.)[-a-zA-Z0-9@:%._\\+~#=]{1,256}\\.[a-z]+(?::\\d+)?(?:[/#?][-a-zA-Z0-9@:%_+.~#$!?&/=\\(\\);,'\">\\^{}\\[\\]`]*)?",
)

// bareKnownDirectiveRe matches a known empty-content text directive token
// in rendered output (e.g. ":u" after an emphasis marker, where colon
// escaping doesn't apply): it re-parses as a mark with no content and
// vanishes; remark degrades identically (probe: "*:*u*!*").
var bareKnownDirectiveRe = regexp.MustCompile("[_*~[(\x60]:(?:u|sub|sup|media|mention|status|color|bg)(?:[^A-Za-z0-9-]|$)")

// leadingTabRe matches a line whose indentation contains a tab.
var leadingTabRe = regexp.MustCompile(`(?m)^ *\t`)

// markerTabRe matches a tab in the whitespace gap after a same-line
// list-marker chain (the same tab-stop column-math class as
// leadingTabRe).
var markerTabRe = regexp.MustCompile(`(?m)^[ \t]*(?:(?:[-*+]|\d+[.)])[ \t]*)+\t`)

// bareOrderedMarkerTextRe matches a line starting with an unescaped
// "N."/"N)" run NOT followed by a space — paragraph text whose leading
// digits+punctuation only get escaped once neighboring nodes merge on
// re-parse; remark is equally unstable (probe: "0:u)\\<break>0").
var bareOrderedMarkerTextRe = regexp.MustCompile(`(?m)^\d+[.)][^ \t\n]`)

// fenceInfoAmpRe matches a fence opener whose info string contains an
// ampersand: a literal "&#…;"/"&name;" in the language decodes as a
// character reference on re-parse; remark renders the identical bytes and
// is equally unstable (probe: "~~~\\&#0;").
var fenceInfoAmpRe = regexp.MustCompile("(?m)^\x60{3,}[^\n]*&")

// markerDirectiveRe matches a ':'-led token directly after an emphasis
// marker (see the skip above).
var markerDirectiveRe = regexp.MustCompile(`[_*~]:[A-Za-z]`)

// refDefLineRe matches a line that could re-parse as a link reference
// definition ("[label]: dest"), which produces no output; remark renders
// the identical bytes and is equally unstable (probe: "[` + "`" + `]:` + "`" + `]( )").
var refDefLineRe = regexp.MustCompile(`(?m)^[ \t]*(?:(?:[-*+]|\d+[.)]|>) )*\[[^\n]*\]:`)

// hexRefOnlyLineRe matches a line consisting only of character references.
var hexRefOnlyLineRe = regexp.MustCompile(`(?m)^(?:&#x[0-9A-Fa-f]+;)+$`)

// markerOnlyChainRe matches a line of bare list markers (any spacing):
// goldmark parses trailing "* *" as an empty nested list where micromark
// keeps literal text, and the wide-gap rendering re-parses as indented
// code — parser divergence (probe: "*\t\t0\n  * 0\n    * *").
var markerOnlyChainRe = regexp.MustCompile(`(?m)^[ \t]*(?:[-*+] +)+[-*+]?[ \t]*$|^[ \t]*[-*+] {2,}[-*+](?: |$)`)

// digitPunctLeadRe matches a text beginning with digits then '.'/')'.
var digitPunctLeadRe = regexp.MustCompile(`^\d*[.)]`)

// escapedColonURLRe: see the escaped-colon-before-URL skip.
var escapedColonURLRe = regexp.MustCompile(`\\:(?:www\.|https?://|ftp://)`)

// escapedPunctURLRe: see the escaped-punctuation-before-URL skip.
var escapedPunctURLRe = regexp.MustCompile(`\\[^A-Za-z0-9\s](?:www\.|https?://|ftp://)`)

// colonBeforeDirectiveRe: see the colon-before-directive skip — a
// mid-line colon run (any length ≥2, raw or escaped) fused onto a
// directive name; the leading [^\n:] keeps line-start leaf/container
// fences out.
var colonBeforeDirectiveRe = regexp.MustCompile(`[^\n:]:{2,}[A-Za-z0-9]`)

// urlFusedWithUnderscore reports a URL/www literal in rendered output
// directly preceded by '_' or sharing its whitespace-delimited token
// with one (goldmark's linkify pattern extends further than
// urlLiteralRe approximates, so the whole token is the hazard zone —
// see the fused-underscore skip).
func urlFusedWithUnderscore(first string) bool {
	for _, m := range urlLiteralRe.FindAllStringIndex(first, -1) {
		if m[0] > 0 && first[m[0]-1] == '_' {
			return true
		}
		token := first[m[0]:]
		if end := strings.IndexAny(token, " \t\n"); end >= 0 {
			token = token[:end]
		}
		if strings.Contains(token, "_") {
			return true
		}
	}
	return false
}

// tabLeadLabelRe matches a '[' whose leading whitespace contains a tab
// (indented-code column math inside the label).
var tabLeadLabelRe = regexp.MustCompile(`\[[ \t]*\t`)

// blockLeadLabelRe matches a '[' followed by block-forming syntax,
// optionally space-indented — up to 3 leading spaces still block-parse
// (see the block-syntax-in-label skip; probe: ":u[ # &A]").
var blockLeadLabelRe = regexp.MustCompile(`\[ {0,3}(?:#{1,6}[ \t]|\d+[.)][ \t]|[-*+][ \t]|>[ \t]?)`)

var setextAmbiguousLineRe = regexp.MustCompile(`(?m)^[ \t]*\S.*\n[ \t]*(-+|=+)[ \t]*$`)

// markerLineRe matches a line beginning with one or more list markers and
// captures the full marker prefix (its length is the content column).
var markerLineRe = regexp.MustCompile(`^[ \t]*(?:(?:[-*+>]|\d+[.)]) )+`)

// markerStartRe matches a line that begins another list marker.
var markerStartRe = regexp.MustCompile(`^[ \t]*(?:[-*+>]|\d+[.)])([ \t]|$)`)

// zeroStartMarkerRe matches an indented marker line that cannot interrupt
// a paragraph — a zero-start ordered marker, or an empty item of any kind
// (CommonMark: empty list items never interrupt) — so the line lazily
// continues the previous one on re-parse.
var zeroStartMarkerRe = regexp.MustCompile(`^[ \t]+(?:(?:0|[2-9]\d*|1\d+)[.)]([ \t]|$)|\d+[.)]$|[-*+]$)`)

// hasLazyChainContinuation reports render shapes that lazily continue the
// previous paragraph line on re-parse: an indented continuation after a
// same-line marker chain, or an indented zero-start ordered marker after a
// content line. remark renders the identical bytes for both (see the ewyh
// bead probes).
func hasLazyChainContinuation(md string) bool {
	lines := strings.Split(md, "\n")
	for i := 0; i+1 < len(lines); i++ {
		line := lines[i]
		next := lines[i+1]
		if strings.TrimSpace(line) == "" {
			continue
		}
		if zeroStartMarkerRe.MatchString(next) {
			return true
		}
		prefix := markerLineRe.FindString(line)
		if prefix == "" {
			continue
		}
		// Walk the paragraph continuation lines that follow the marker
		// chain: any line shallower than the chain's content column lazily
		// continues the deeper paragraph on re-parse; remark renders the
		// same bytes (see the ewyh bead probes).
		for j := i + 1; j < len(lines); j++ {
			cont := lines[j]
			trimmed := strings.TrimLeft(cont, " \t")
			if trimmed == "" {
				continue // blank lines don't end the item's continuation scope
			}
			if markerStartRe.MatchString(cont) {
				break
			}
			if len(cont)-len(trimmed) < len(prefix) {
				return true
			}
		}
	}
	return false
}

// emptyQuoteLineRe matches a line consisting only of blockquote markers
// (optionally list-indented).
var emptyQuoteLineRe = regexp.MustCompile(`^[ \t]*(?:(?:[-*+]|\d+[.)]) )*(?:> ?)+$`)

// quoteContinuationRe matches a line that continues a blockquote
// (optionally list-indented) with content after the markers.
var quoteContinuationRe = regexp.MustCompile(`^[ \t]*(?:(?:[-*+]|\d+[.)]) )*> ?`)

// hasAdjacentEmptyQuotes reports two consecutive marker-only blockquote
// lines (adjacent empty quotes merge on re-parse).
func hasAdjacentEmptyQuotes(md string) bool {
	prev := false
	for line := range strings.SplitSeq(md, "\n") {
		cur := emptyQuoteLineRe.MatchString(line)
		if cur && prev {
			return true
		}
		prev = cur
	}
	return false
}

// indentedEmptyQuoteRe matches a marker-only quote line nested inside a
// list item (indented, or on a line with list markers).
var indentedEmptyQuoteRe = regexp.MustCompile(`^[ \t]+(?:> ?)+$|^[ \t]*(?:(?:[-*+]|\d+[.)]) )+(?:> ?)+$`)

// hasListNestedEmptyQuoteLine reports an empty quote line inside a list
// item. goldmark ends the item there and re-opens the rest of the quote at
// top level, while remark (CommonMark-correct) keeps one quote — a known
// goldmark divergence tracked on the ewyh bead.
func hasListNestedEmptyQuoteLine(md string) bool {
	for line := range strings.SplitSeq(md, "\n") {
		if indentedEmptyQuoteRe.MatchString(line) {
			return true
		}
	}
	return false
}

// wrappedBlockLeadRe matches a line that could start block syntax when it
// lands at a line start (directives, lists, quotes, headings, setext or
// thematic lines, table rows, fences, ordered markers).
var wrappedBlockLeadRe = regexp.MustCompile("^(?:[-+*#>=|:~\x60]|\\d+[.)])")

// hasWrappedBlockSyntaxLine reports a paragraph continuation line that
// begins with block-forming syntax. Prose wrapping can move such a word to
// a line start where it re-parses as a block; the reference pipeline (remark +
// prettier wrap) wraps without re-escaping and is equally unstable
// (probes: ":0 ::000…", "000… -\\|"). Canonical blocks always follow a
// blank line, so a matching line directly after content can only come from
// wrapping (escaped break continuations never match — their lead is
// escaped).
func hasWrappedBlockSyntaxLine(md string) bool {
	inFence := false
	prevContent := false
	for line := range strings.SplitSeq(md, "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			prevContent = false
			continue
		}
		if inFence {
			continue
		}
		if prevContent && wrappedBlockLeadRe.MatchString(trimmed) {
			return true
		}
		prevContent = strings.TrimSpace(line) != ""
	}
	return false
}

// hasSameBulletSiblingLists reports two consecutive blank-line-separated
// blocks that both start with the SAME top-level bullet marker: remark's
// single useDifferentMarker flag produces this when a break-safe flip
// collides with alternation (probe: "+ \n*\n+ ***"), and the lists merge
// on re-parse; remark renders the identical bytes.
func hasSameBulletSiblingLists(md string) bool {
	prev := byte(0)
	for block := range strings.SplitSeq(md, "\n\n") {
		if block == "" {
			continue
		}
		c := block[0]
		isBullet := (c == '-' || c == '*' || c == '+') && (len(block) == 1 || block[1] == ' ' || block[1] == '\n')
		if !isBullet {
			prev = 0
			continue
		}
		if c == prev {
			return true
		}
		prev = c
	}
	return false
}

// hasTrailingBreakBackslash reports a line ending in a single backslash
// (a rendered hard break) whose next line is blank or missing — the break
// has no continuation and re-parses as a literal backslash.
func hasTrailingBreakBackslash(md string) bool {
	lines := strings.Split(md, "\n")
	for i, line := range lines {
		backslashBreak := strings.HasSuffix(line, "\\") && !strings.HasSuffix(line, "\\\\")
		spacesBreak := strings.HasSuffix(line, "  ") && strings.TrimSpace(line) != ""
		if !backslashBreak && !spacesBreak {
			continue
		}
		if i+1 >= len(lines) || strings.TrimSpace(lines[i+1]) == "" {
			return true
		}
	}
	return false
}

// hasTrailingEmptyQuoteLine reports a marker-only blockquote line at the
// END of its quote (the next line does not continue the quote): the empty
// sibling/trailing quote merges into the previous one and drops on
// re-parse. An empty quote line BETWEEN two quote content lines is a
// stable paragraph separator and does not match.
func hasTrailingEmptyQuoteLine(md string) bool {
	lines := strings.Split(md, "\n")
	for i, line := range lines {
		if !emptyQuoteLineRe.MatchString(line) {
			continue
		}
		if i+1 >= len(lines) || !quoteContinuationRe.MatchString(lines[i+1]) {
			return true
		}
	}
	return false
}

// skipRawInput reports raw input classes that are out of scope for the
// round-trip invariant, before any parsing happens.
func skipRawInput(md string) (reason string, skip bool) {
	// Invalid UTF-8 cannot occur in Jira content (the ADF JSON layer
	// guarantees valid strings).
	if !utf8.ValidString(md) {
		return "invalid UTF-8 out of scope", true
	}
	// Control characters cannot occur in Jira content and even remark is
	// not round-trip stable on them (emphasis flanking classifies them
	// as word characters while rendering cannot escape them).
	for _, r := range md {
		if (r < 0x20 && r != '\n' && r != '\t' && r != '\r') || r == 0x7f {
			return "control characters out of scope", true
		}
	}
	// Leading tabs interact with goldmark's tab-stop expansion
	// differently than micromark's (nested list/indented-code column
	// math); rendered output never emits leading tabs, so the input
	// class is out of scope. A tab in the marker gap of a list-marker
	// chain is the same column-math class (probe: "*\t\t0\n  * 0\n    * 0)").
	normalized := strings.ReplaceAll(strings.ReplaceAll(md, "\r\n", "\n"), "\r", "\n")
	if leadingTabRe.MatchString(normalized) || markerTabRe.MatchString(normalized) {
		return "tab-indented block structure out of scope", true
	}
	return "", false
}

// skipDocClasses reports parsed-document classes (checked before rendering)
// that are out of scope for the round-trip invariant.
func skipDocClasses(doc adf.Doc) (reason string, skip bool) {
	// Character references can smuggle control characters past the raw
	// input check ("&#1;" decodes to \x01) — same out-of-scope class.
	if hasControlText(doc.Content) {
		return "control characters (via character references) out of scope", true
	}
	// Adjacent code spans with identical mark sets render their backtick
	// fences back to back ("``a```b`"), merging the runs on re-parse;
	// remark renders the identical bytes and is equally unstable
	// (probe: ``*0`0``*`0`*).
	if hasAdjacentCodeSpans(doc.Content) {
		return "adjacent code spans; remark is equally unstable", true
	}
	// Sibling blockquotes in a list item render adjacent and merge on
	// re-parse; remark renders the identical bytes and is equally
	// unstable (probe: "* >0\n\n  >0").
	if hasAdjacentQuotesInItem(doc.Content, false) {
		return "adjacent blockquotes in list item; remark is equally unstable", true
	}
	if hasMergingDigitPunct(doc.Content) {
		return "digit+punct texts merge on re-parse; remark is equally unstable", true
	}
	if hasCodeBoundaryBreak(doc.Content) {
		return "mark inference across code boundary with hard break; remark is equally unstable", true
	}
	// A URL literal inside a text node WITHOUT a link mark: round one's
	// parse did not linkify it (a construct boundary split the token in
	// the source), but the render fuses it back together, so the
	// re-parse linkifies and the mark structure flips — the
	// attention-vs-literal hazard again (probe: "*0 http://0.*a 0**" →
	// "_0 http://0.a 0_" → "_0 <http://0.a> 0_").
	if hasUnlinkedURLText(doc.Content) {
		return "unlinked URL literal in text; re-parse linkifies what a construct boundary split", true
	}
	// A media alt containing markdown-active characters: the alt renders
	// as plain text inside the ![…] / [label] context (brackets and
	// backslashes escaped, the rest verbatim), but goldmark re-parses
	// alt content as INLINE NODES where mdast keeps alt as a plain
	// string — emphasis markers, code ticks, autolinks, character
	// references, and directive-forming ':' tokens all change the alt on
	// the next round (parser divergence; probes: "![:0[:]0](http://)",
	// "![0*0[0\*]](http://)").
	if hasUnsafeMediaAlt(doc.Content) {
		return "markdown-active media alt; goldmark parses alt inlines where mdast keeps a string", true
	}
	return "", false
}

// altUnsafeRe matches alt content that is markdown-active in label
// context (see the skip above).
var altUnsafeRe = regexp.MustCompile("[*_\x60~<&]|:[A-Za-z0-9]")

// unsafeAlt reports alt text the label round trip cannot hold shape-
// stable: markdown-active characters, or boundary whitespace (label
// whitespace normalizes across parses; probe: "::media[  ]").
func unsafeAlt(alt string) bool {
	if altUnsafeRe.MatchString(alt) {
		return true
	}
	return alt != "" && strings.TrimSpace(alt) != alt
}

// hasUnsafeMediaAlt reports a media/mediaInline alt holding
// markdown-active content (see the skip above).
func hasUnsafeMediaAlt(nodes []adf.Node) bool {
	for i := range nodes {
		switch n := nodes[i].(type) {
		case *adf.Media:
			if unsafeAlt(n.Alt) {
				return true
			}
		case *adf.MediaInline:
			if unsafeAlt(n.Alt) {
				return true
			}
		}
		if hasUnsafeMediaAlt(adf.NodeContent(nodes[i])) {
			return true
		}
	}
	return false
}

// skipDocLinkClasses reports link-shaped document classes (checked after
// rendering, but on the parsed document) that are out of scope.
func skipDocLinkClasses(doc adf.Doc) (reason string, skip bool) {
	// A link href starting with a literal "<" renders as "](<...)" and
	// re-parses as an angle destination; remark renders the identical
	// bytes and is equally unstable (probes: [0](\<), [0](\<>)).
	if hasAngleLeadingHref(doc.Content) {
		return "angle-leading link href; remark is equally unstable", true
	}
	if hasDigitDirectiveInLink(doc.Content) {
		return "digit-led directive token in link label; remark is equally unstable", true
	}
	// A link inside a mark-directive label: the rendered "[text](url)"
	// carries ']' into the :u/:sub/:color/… label, which ends at the
	// FIRST ']' (goldmark-directive does not balance label brackets
	// where micromark does) — parser divergence (probe: "[:u[0]]()").
	if hasLinkUnderMarkDirective(doc.Content) {
		return "link inside mark-directive label; goldmark labels do not nest brackets", true
	}
	return "", false
}

// skipRenderedClasses reports first-render shapes that are out of scope for
// the round-trip invariant.
func skipRenderedClasses(first string) (reason string, skip bool) {
	if reason, skip = skipRenderedStructureClasses(first); skip {
		return reason, skip
	}
	return skipRenderedTokenClasses(first)
}

// skipRenderedStructureClasses reports out-of-scope first-render shapes
// rooted in block structure (setext, quotes, lists, fences, definitions).
func skipRenderedStructureClasses(first string) (reason string, skip bool) {
	// An empty nested list rendered directly under an item's text line
	// forms a setext underline ("- 0\n  -" re-parses as "- ## 0").
	// remark renders the identical shape and is equally unstable (see
	// the ewyh bead probes), so the input class is out of scope.
	if setextAmbiguousLineRe.MatchString(first) {
		return "setext-forming empty nested list; remark is equally unstable", true
	}
	// A rendered document starting with a thematic break re-parses as
	// frontmatter and is stripped — the remark reference pipeline does the same
	// (lenient frontmatter stripping), so the class is out of scope.
	if strings.HasPrefix(first, "---\n") {
		return "leading thematic break reads as frontmatter; the reference pipeline behaves identically", true
	}
	// An item continuation line after a same-line marker chain
	// ("- - 0\n  0") lazily continues the deeper paragraph on
	// re-parse; remark renders the identical shape and is equally
	// unstable (see the ewyh bead probes).
	if hasLazyChainContinuation(first) {
		return "continuation after marker chain; remark is equally unstable", true
	}
	// Adjacent empty blockquotes ("- >\n  >") merge into one quote on
	// re-parse; remark renders the identical bytes and is equally
	// unstable (see the ewyh bead probes).
	if hasAdjacentEmptyQuotes(first) {
		return "adjacent empty blockquotes; remark is equally unstable", true
	}
	// A quote's trailing marker-only line ("- > 0\n  >") contributes
	// nothing on re-parse and drops; remark renders the identical
	// bytes and is equally unstable (probe: "* >0\n\n  >").
	if hasTrailingEmptyQuoteLine(first) {
		return "trailing empty blockquote line; remark is equally unstable", true
	}
	// goldmark splits a quote at an empty quote line nested in a list
	// item ("- > -\n  >\n  > 0" re-parses the tail as a top-level
	// quote); remark keeps one quote. Known goldmark parser divergence
	// (ewyh bead), out of scope for the renderer.
	if hasListNestedEmptyQuoteLine(first) {
		return "empty quote line in list item; goldmark parser divergence", true
	}
	if gfmEmailInvalid(first) {
		return "invalid email-like token; goldmark parser divergence (first render matches remark)", true
	}
	if bareKnownDirectiveRe.MatchString(first) {
		return "bare known directive token after marker; remark is equally unstable", true
	}
	// A ':' right after an emphasis marker starts a directive whose
	// name can absorb the closing marker on re-parse ("_:A_-0" reads
	// as directive "A_-0"); remark renders the identical bytes and is
	// equally unstable.
	if markerDirectiveRe.MatchString(first) {
		return "directive after emphasis marker; remark is equally unstable", true
	}
	if bareOrderedMarkerTextRe.MatchString(first) {
		return "bare ordered-marker text at line start; remark is equally unstable", true
	}
	if fenceInfoAmpRe.MatchString(first) {
		return "ampersand in fence info; remark is equally unstable", true
	}
	if refDefLineRe.MatchString(first) {
		return "link-reference-definition shaped line; remark is equally unstable", true
	}
	return "", false
}

// skipRenderedTokenClasses reports out-of-scope first-render shapes rooted
// in inline tokens (character references, directives, wraps, spaces).
// skipRenderedURLClasses reports rendered token fusions around URL
// literals and directive tokens (split from skipRenderedTokenClasses
// for complexity budgets).
func skipRenderedURLClasses(first string) (reason string, skip bool) {
	// An escaped colon directly before a URL/www literal re-linkifies
	// on re-parse (the escape removed the directive reading); remark
	// is equally unstable (probe: ":www.0.a").
	if escapedColonURLRe.MatchString(first) {
		return "escaped colon before URL literal; remark is equally unstable", true
	}
	// Any other rendered backslash escape directly before a www/URL
	// literal is the same class: GFM autolink literals may follow the
	// escaped punctuation (cmark-gfm allows '_', '*', '~', '(' before
	// "www."), so the second parse linkifies what the first round —
	// where the token was still fused into a directive name — did not.
	// remark renders the identical escape bytes and re-linkifies the
	// same way (probe: ":0_www.0.a" → ":0\_www.0.a").
	if escapedPunctURLRe.MatchString(first) {
		return "escaped punctuation before URL literal; remark is equally unstable", true
	}
	// A colon (raw or escaped) directly before a rendered text directive:
	// the literal ':' triggers the preceded-by-colon guard (remark
	// directive rule), so the directive cannot re-parse and flattens to
	// text on the next round; remark renders the identical bytes and is
	// equally unstable (probes: ":*:media[0]00*" → "\::media[0]{…}",
	// "0:*:media[0]*" → "0::media[0]{…}").
	if colonBeforeDirectiveRe.MatchString(first) {
		return "colon before text directive; remark is equally unstable", true
	}
	// A '_' emphasis marker fused onto a URL/www literal (either side):
	// '_' is inside GFM's extended-autolink charset, so the re-parse
	// linkifies through the marker and the emphasis reading flips — the
	// same attention-vs-literal hazard as the raw-input '_' URL class
	// (probes: " *http://0.*a**" → "_http://0.a_",
	// "*0 http://*0.a**" → "_0 http://0.a_").
	if urlFusedWithUnderscore(first) {
		return "underscore emphasis fused to URL literal; goldmark linkify consumes the marker", true
	}
	return "", false
}

func skipRenderedTokenClasses(first string) (reason string, skip bool) {
	// A line holding only encoded whitespace comes from wrapping at a
	// dropped construct's residue and rejoins differently on re-parse.
	if hexRefOnlyLineRe.MatchString(first) {
		return "encoded-whitespace-only line; wrap artifact", true
	}
	if markerOnlyChainRe.MatchString(first) {
		return "marker-only list chain line; goldmark parser divergence", true
	}
	if reason, skip := skipRenderedURLClasses(first); skip {
		return reason, skip
	}
	// A directive label starting with block syntax (tab-indented code,
	// heading, list, quote) block-parses inside the label — goldmark
	// parses label content as blocks where remark treats labels as
	// inline-only, and raw-segment constructs keep escapes undecoded —
	// parser divergence (probes: ":u[\t&A]", ":u[# 00&A]").
	if tabLeadLabelRe.MatchString(first) || blockLeadLabelRe.MatchString(first) {
		return "block syntax in directive label; goldmark parser divergence", true
	}
	// An '@' whose neighbor was flanking-hex-encoded flips its escape
	// decision between rounds (the escape looks at the AST neighbor,
	// the re-parse sees the reference); remark diverges structurally on
	// these inputs and is not stable either (probe: "**0@*0*~!~*").
	if strings.Contains(first, "@&#x") || strings.Contains(first, ":&#x") || strings.Contains(first, "&&#x") {
		return "escapable punctuation adjoining a character reference; remark is equally unstable", true
	}
	// Wrapping can start a continuation line with block syntax, which
	// re-parses as a block; the reference pipeline wraps identically without
	// re-escaping and is equally unstable.
	if hasWrappedBlockSyntaxLine(first) {
		return "wrapped line starts block syntax; the reference pipeline is equally unstable", true
	}
	// A hard break at the end of a block renders as a trailing
	// backslash with no continuation line, which re-parses as literal
	// text; remark renders the identical bytes (probes: "[\\\r]()",
	// "0[\\\r]()").
	if hasTrailingBreakBackslash(first) {
		return "trailing hard break; remark is equally unstable", true
	}
	if hasSameBulletSiblingLists(first) {
		return "same-bullet sibling lists; remark is equally unstable", true
	}
	// An empty paragraph (e.g. a known text directive with no content,
	// ":u") renders as bare blank lines that vanish on re-parse; remark
	// renders the identical bytes (probe: ":u\r\r0").
	if strings.HasPrefix(first, "\n") || strings.Contains(first, "\n\n\n") || strings.Contains(first, "\n\n:::") {
		return "empty paragraph renders as blank lines; remark is equally unstable", true
	}
	// A digit-led ":name{…}"/":name[…]" token in plain text re-parses
	// as a text directive and sheds its braces; the reference pipeline
	// degrades it identically (probe: ":0{}00{}").
	if digitDirectiveTokenRe.MatchString(first) {
		return "digit-led directive token in text; the reference pipeline is equally unstable", true
	}
	// The three interior-space skip classes that used to live here —
	// dropped empty links leaving "x  y", the same run with one space
	// boundary-encoded, and the same at a line start — are fixed rather
	// than skipped: adf.NormalizeTextNewlines now collapses a space run
	// across the junction of two adjacent same-mark text nodes, which is
	// exactly what a dropped construct leaves behind. Probes: "x []() y",
	// "[]() []() 0", "*0aaa[0 :u ]*".
	// A link destination that is (or starts with) a literal "<" renders
	// as "](<...)" — an unterminated angle destination that fails to
	// re-parse as a link; remark renders the identical bytes and is
	// equally unstable (probe: [0](\\<)).
	if unterminatedAngleDestRe.MatchString(first) {
		return "unterminated angle link destination; remark is equally unstable", true
	}
	return "", false
}

func FuzzRoundTripIdempotent(f *testing.F) {
	for _, seed := range fuzzSeeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, md string) {
		if reason, skip := skipRawInput(md); skip {
			t.Skip(reason)
		}
		doc := mdToADF(md)
		if reason, skip := skipDocClasses(doc); skip {
			t.Skip(reason)
		}
		first0 := adfToMD(doc)
		// A tab inside prose that the wrapper broke at becomes a newline
		// and collapses to a space on re-parse; the reference pipeline (prettier
		// wrap) renders identical bytes and is equally unstable.
		if hasTabText(doc.Content) && !strings.Contains(first0, "\t") {
			t.Skip("wrapped tab in prose; the reference pipeline is equally unstable")
		}
		if reason, skip := skipDocLinkClasses(doc); skip {
			t.Skip(reason)
		}
		first := first0
		if reason, skip := skipRenderedClasses(first); skip {
			t.Skip(reason)
		}
		second := adfToMD(mdToADF(first))
		if first != second {
			t.Errorf("round-trip not idempotent for %q:\nfirst:  %q\nsecond: %q", md, first, second)
		}
	})
}
