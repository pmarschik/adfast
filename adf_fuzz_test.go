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
	// GFM footnotes: the pair, a reference under marks, a definition the
	// source nested, and the label shapes that are NOT footnotes
	"a[^1]\n\n[^1]: note",
	"a[^one] b[^two] c[^one]\n\n[^two]: second\n\n[^one]: first",
	"**a[^1]** and `b`[^1]\n\n[^1]: x",
	"a[^1]\n\n> [^1]: quoted\n\n- [^2]: listed\n\nb[^2]",
	"a[^1]\n\n[^1]: one\n\n    two\n\n[^1]: duplicate",
	"a[^a b] c[^]\n\n[^a b]: not a footnote\n\n[^]: neither",
	// escape sequences
	"\\`backtick\\` and \\_underscore\\_",
	// A character reference in text must survive as text: written bare it
	// decodes on the next parse.
	"a \\&#169; b\n\nAT&T and \\&amp; and \\&#xA9;\n",
	// A tight list item holding a paragraph immediately followed by a GFM
	// table (no source blank) forces an internal gap on render; the item
	// must separate from its successor too, or the render is a fixpoint
	// only after two passes.
	"- a\n  | x | y |\n  | - | - |\n  | 1 | 2 |\n- b\n",
	// colon-after-backtick (the remark-directive trap)
	"\\`bisearch:partner-sync-opt-out\\`",
	// autolinks
	"See <https://example.com> for details.",
	// The formatter's emphasis delimiter landing inside an autolink
	// literal's host: "_www._.A" holds a "www." literal whose host is
	// "www._.A", so before the host rule the re-parse ate the emphasis and
	// invented a link (see TestEmphasisDelimiterDoesNotBecomeAHost).
	"*www.*.A",
	// The same delimiter, landing in a domain that is nothing BUT
	// punctuation: "_www.._" holds a literal whose domain is ".._", which
	// trims to empty. A gate that trimmed the whole literal instead of the
	// domain saw the clean host "www" and linked it.
	"*www..*",
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
	// strike ends at a code span, and unmarked text follows immediately:
	// the mark must not leak onto that trailing text.
	"~0`0`~!",
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
// markdown.escapesColon). Probes: ":u[0:0:0]", ":sup[:a:b]".
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

// urlLiteralRe matches GFM literal-autolink URLs. It APPROXIMATES the markdown
// package's linkify pattern rather than copying it, and the direction of the
// approximation is what matters: this drives a fuzz SKIP, so matching more than
// the parser does hides inputs, while matching less only costs a skip that was
// never needed. Keep it no wider than the real pattern.
//
// The scheme set is `https?://` and `www.` — the GFM set the parser is
// configured to. It carried `ftp` while goldmark still linkified a bare
// `ftp://`; the parser no longer does, so `ftp` here would skip inputs the
// fuzzer should see.
//
// The TLD part is deliberately narrower than the real one, which takes digits
// and `_`. Widening it would widen a skip.
var urlLiteralRe = regexp.MustCompile(
	"(?:https?://|www\\.)[-a-zA-Z0-9@:%._\\+~#=]{1,256}\\.[a-z]+(?::\\d+)?(?:[/#?][-a-zA-Z0-9@:%_+.~#$!?&/=\\(\\);,'\">\\^{}\\[\\]`]*)?",
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

// urlLiteralOpenerPat spells a www literal's opening bytes THE WAY THE
// RENDERER WRITES THEM, which is not the way a source author writes them.
// The two classes below key on a rendered escape sitting in front of a
// literal, so they have to read the literal out of RENDERED bytes — and a
// row of the remark unsafe table lands inside the opener itself:
// dotAfterWwwEscapes writes a backslash on a '.' that follows a 'w', so the
// same literal reaches the output as "www." or as "www\.".
//
// Both spellings link on the next parse, because the decoded-text linkify
// (relinkifyTexts, mirroring mdast-util-gfm-autolink-literal's transform)
// reads the text the escape produced and never sees the backslash. Keying on
// the unescaped spelling alone is therefore not a narrower skip, it is a DEAD
// one: it stops matching the moment the renderer decides to escape the dot.
// That is what happened to both classes below — each was written with the
// probe in its skip comment, and neither probe has matched since the dot rule
// landed, so the class silently covered nothing.
//
// IT IS CASE-INSENSITIVE BECAUSE THE RECOGNIZER IS. Both GFM autolink-literal
// halves match "www." without regard to case, and the unsafe row that
// escapes the dot is written `before: '[Ww]'`, so ":wwW.0" renders to
// "\:wwW\.0" and re-links exactly as ":www.0" does. A lowercase-only pattern
// leaves that shape uncovered; the fuzzer reaches it in about 77 seconds.
//
// IT NO LONGER SPELLS A SCHEMED "http://"/"https://" OPENER (storysmith-md-
// h97u). A www literal's href always differs from its text, so it never
// takes the ADF leg's Bare branch and keeps writing [label](url) regardless;
// an http(s) literal whose text equals its href does take that branch, and
// since the ADF leg now writes it bare like the format leg does, escaping
// punctuation in front of one no longer changes whether it links on the next
// parse — probed directly: "a\_http://0", "a_http://0" and "a\_HTTP://0.0"
// are all now stable round trips, where they used to dodge this class only
// because the old angle rendering put a '<' between the escape and the
// scheme. Keeping "https?://" in this pattern would skip those as if they
// still diverged.
const urlLiteralOpenerPat = `(?i:www\\?\.)`

// escapedColonURLRe: see the escaped-colon-before-URL skip.
var escapedColonURLRe = regexp.MustCompile(`\\:` + urlLiteralOpenerPat)

// escapedPunctURLRe: see the escaped-punctuation-before-URL skip.
var escapedPunctURLRe = regexp.MustCompile(`\\[^A-Za-z0-9\s]` + urlLiteralOpenerPat)

// escapedSchemeColonRe finds the ':'-before-'/' escape the remark unsafe
// table writes inside a scheme (colonBeforeSlashEscapes). It is only HALF of
// the escaped-scheme-colon skip: whether that escape costs a round trip
// depends on the host behind it, and a dotless one is stable with the escape
// intact — "https\://x" and "_http\://x_" both re-render unchanged, because
// mdast-util-gfm-autolink-literal's isCorrectDomain demands two dot-separated
// segments. escapedSchemeColonIgnored is the other half.
//
// IT SPELLS THE UNSAFE ROW, `before: '[ps]'`, NOT THE SCHEME. The escape
// appears iff the byte in front of the colon is a p or an s, so
// "*HTTp://*0.0**" renders "_HTTp\://0.0_" while "*fttp://*0.0**" renders
// "_fttp://0.0_" with no escape at all — a scheme-shaped `https?\\://` misses
// the first of those.
//
// IT IS CASE-INSENSITIVE BECAUSE THE RENDERER'S ROW IS, and unlike
// urlLiteralOpenerPat's `(?i:)` next door that is not a widening for
// tidiness's sake: it is load-bearing, and it was measured as load-bearing
// only after colonBeforeSlashEscapes started folding the byte it reads. The
// pair below is the same tree one letter's case apart, rendered and
// re-rendered, with what this gate and the skip it drives say about each:
//
//	"*HTTp://*0.0**" -> "_HTTp\://0.0_" -> "_<HTTp://0.0>_"  covered
//	"*httP://*0.0**" -> "_httP\://0.0_" -> "_<httP://0.0>_"  UNCOVERED
//
// Both are unstable, both are unstable for the identical reason (the decoded
// -text linkify never sees the backslash), and a lowercase-only gate skips
// the first and hands the second to the fuzzer as a fresh crasher. Widening
// is what makes the gate describe the renderer it is named after. Narrow it
// back and TestUppercaseSchemeColonIsALiveFuzzFailureNotASkipClass's
// dotted-host row fails with exactly that pair as its evidence.
//
// Note what does NOT belong in this class and never did: the UNESCAPED
// uppercase spelling. That was a renderer defect — the literal went out bare
// and linkified on re-parse — and it was fixed in the renderer rather than
// gated here, which is the only reason the escape exists for this pattern to
// find.
var escapedSchemeColonRe = regexp.MustCompile(`(?i)[ps]\\://`)

// escapedSchemeColonIgnored reports a rendered "http\://" whose backslash the
// next parse ignores — the escape is written to keep a scheme out of a link
// and the document comes back the same with it deleted, which is the whole of
// the defect.
//
// IT ASKS THE PARSER RATHER THAN A PATTERN, because the host rule here is not
// one. urlliteral.go spells out why — isCorrectDomain and micromark's domain
// production are scanners over the host's dot-separated segments, they REJECT
// an address rather than shortening one, and a character class written to
// imitate either simply matches a shorter host. A pattern keyed on two
// alphanumeric-anchored segments looks right and still misses "_http\://.0_",
// whose host ".0" has an EMPTY first segment that isCorrectDomain accepts;
// the fuzzer finds that shape in about two seconds.
//
// The escape is not decoded here either, for the same reason: the question is
// what the parse does with the bytes, and only the parse knows.
func escapedSchemeColonIgnored(first string) bool {
	var base []byte
	for _, loc := range escapedSchemeColonRe.FindAllStringIndex(first, -1) {
		// loc ends at the '/' pair, so the backslash is four bytes back.
		bs := loc[1] - len(`\://`)
		if base == nil {
			base = skipCompareJSON(mdToADF(first))
		}
		if bytes.Equal(skipCompareJSON(mdToADF(first[:bs]+first[bs+1:])), base) {
			return true
		}
	}
	return false
}

// skipCompareJSON is the comparison form for two documents in a skip
// predicate. adfJSON, the assertion helper next door, wants a *testing.T.
func skipCompareJSON(doc adf.Doc) []byte {
	data, err := json.Marshal(doc)
	if err != nil {
		return nil
	}
	return data
}

// wwwDotPunctRe matches a rendered "www." host literal where
// dotAfterWwwEscapes did not escape the dot — the byte after it is outside
// the after-class ('-', '.', or a word char) — yet the byte is one of a
// narrower set the GFM www-autolink host grammar still accepts as the start
// of a second label, so a later dot completes a linkable host on re-parse.
// dotAfterWwwEscapes reads exactly one byte of lookahead (see its doc
// comment); it cannot see that a second, later dot still forms a host, so no
// escape is written and the next parse links what the first left bare.
//
// remark's unsafe table has the identical one-byte-wide row (`before: '[Ww]'`,
// `after: '[\-.\w]'`) and degrades on the identical bytes. Probed as a
// POST-LOSS tree (the mdast built by hand, stringified, at the same
// unified/remark-parse/remark-gfm/remark-stringify versions
// TestSchemeColonIsEscapedAndStableInEitherCase cites):
//
//	stringify(em[text "www.#."])  ->  "*www.#.*\n"
//	parse("www.#.")               ->  link http://www.#
//	stringify(that)               ->  "*[www.#](http://www.#).*\n"
//
// That is adfast's own pair exactly (probe: "*www.#.*"). THE CHARACTER SET IS
// DELIBERATELY NARROW: each byte below was checked against the reference
// individually, and only kept where the reference relinks on the identical
// bytes too. Other punctuation between the dots (probe: "*www.!.*" ->
// "_www.!._\n", stable in both passes) already round-trips here and in the
// reference — adding it to this class would skip a shape neither side gets
// wrong.
var wwwDotPunctRe = regexp.MustCompile(`(?i:www)\.[#%+=]\.`)

// wwwEscapedDotRe finds a rendered "www\." — the escaped half of the dot rule
// above, where dotAfterWwwEscapes DID write the backslash (the next byte was
// '-', '.', or a word char). wwwEscapedDotIgnored is the oracle that decides
// whether that escape costs a round trip.
var wwwEscapedDotRe = regexp.MustCompile(`(?i:www)\\\.`)

// wwwEscapedDotIgnored reports a rendered "www\." whose backslash the next
// parse ignores: the decoded-text linkify (relinkifyTexts, mirroring
// mdast-util-gfm-autolink-literal's transform) reads the text the escape
// produced, never sees the backslash, and still finds a valid host behind it.
//
// IT ASKS THE PARSER RATHER THAN A PATTERN, for the same reason
// escapedSchemeColonIgnored does: whether the escaped dot "costs" anything
// depends on what completes the host after it, which only the real domain
// scanner can answer. "www\.-." is stable — "-" alone is not a valid second
// label, so the escape never had anything to lose — while "www\..0" is not:
// a second dot and the alnum segment "0" complete a host the escape cannot
// hide (probe: "*www..*0**" -> "_www\\..0_\n" -> "_[www..0](http://www..0)_\n").
// "www\...0" degrades the same way with one more dot (probe: "*www...*0**").
//
// Measured against the reference on the same post-loss construction:
//
//	stringify(em[text "www..0"])  ->  "*www\.·.0*\n" (literally "*www\..0*\n")
//	parse("www\..0")              ->  link http://www..0
//	stringify(that)                -> "*[www..0](http://www..0)*\n"
//
// which is the identical pair one delimiter apart.
func wwwEscapedDotIgnored(first string) bool {
	var base []byte
	for _, loc := range wwwEscapedDotRe.FindAllStringIndex(first, -1) {
		// loc ends at the dot, so the backslash is one byte back.
		bs := loc[1] - len(`\.`)
		if base == nil {
			base = skipCompareJSON(mdToADF(first))
		}
		if bytes.Equal(skipCompareJSON(mdToADF(first[:bs]+first[bs+1:])), base) {
			return true
		}
	}
	return false
}

// colonBeforeDirectiveRe: see the colon-before-directive skip — a
// mid-line colon run (any length ≥2, raw or escaped) fused onto a
// directive name; the leading [^\n:] keeps line-start leaf/container
// fences out.
var colonBeforeDirectiveRe = regexp.MustCompile(`[^\n:]:{2,}[A-Za-z0-9]`)

// urlFusedSchemeHostRe matches a scheme-prefixed literal with a DOTLESS
// host ("http://0"), which goldmark's linkify extension really does link
// in production — see markdown/barelinkform_test.go's "a dotless host"
// row, and markdown/urlliteral.go's urlLiteralHost, whose dotless
// alternative is what that production behavior is built from. The host
// and path character classes below are copied from urlLiteralRe so the
// two patterns agree on everything except the dot requirement.
//
// THIS REGEXP IS LOCAL TO urlFusedWithUnderscore, deliberately not folded
// into urlLiteralRe: several OTHER skip classes in this file (the
// escaped-scheme-colon class, the www-dot-punctuation class, …) scan with
// urlLiteralRe too, and widening it to accept a dotless host would widen
// their match sets along with this one — urlLiteralRe's own doc comment
// states why it stays dotted-only (it mirrors the DECODED-TEXT
// recognizer, which demands a dot). Scoping the widening to a second,
// narrower regexp used only here keeps those other classes unchanged.
// There is no "www." counterpart: the "www." prefix supplies the dot the
// decoded-text recognizer demands, so a dotless www literal is not a
// production shape (see urlLiteralWWWAnchoredRe's doc comment).
var urlFusedSchemeHostRe = regexp.MustCompile(
	`https?://[a-zA-Z0-9][-a-zA-Z0-9@:%._\+~#=]{0,255}(?:[/#?][-a-zA-Z0-9@:%_+.~#$!?&/=\(\);,'">\^{}\[\]` + "`" + `]*)?`,
)

// fusedUnderscoreHazard reports the fused-underscore hazard for one match
// starting at first[start:]: a '_' directly before it, or a '_' sharing its
// whitespace-delimited token (goldmark's linkify pattern extends further
// than the test-side regexes approximate, so the whole token is the hazard
// zone). When checkEscapes is true, a '_' the source escaped — inert text,
// not an emphasis delimiter — does not count; see urlFusedSchemeHostRe's
// call below for why only that path needs the distinction.
func fusedUnderscoreHazard(first string, start int, checkEscapes bool) bool {
	if start > 0 && first[start-1] == '_' && (!checkEscapes || !escapedByteAt(first, start-1)) {
		return true
	}
	token := first[start:]
	if end := strings.IndexAny(token, " \t\n"); end >= 0 {
		token = token[:end]
	}
	for i := range token {
		if token[i] == '_' && (!checkEscapes || !escapedByteAt(token, i)) {
			return true
		}
	}
	return false
}

// escapedByteAt reports whether the byte at index i in s is a literal
// character produced by a backslash escape immediately in front of it.
// It walks back over the full run of consecutive '\' bytes ending at
// i-1: an odd-length run means the last backslash in the run escapes the
// byte at i (escaped — e.g. a run of 1 or 3 backslashes before '_'), an
// even-length run means every backslash in the run pairs off escaping
// the backslash before it, leaving the byte at i bare (not escaped —
// e.g. a run of 2 or 4). A fixed two-byte lookback only distinguishes 0
// from 1 backslash and gets a 3-backslash run wrong; this has no depth
// limit.
func escapedByteAt(s string, i int) bool {
	n := 0
	for i-1-n >= 0 && s[i-1-n] == '\\' {
		n++
	}
	return n%2 == 1
}

// urlFusedWithUnderscore reports a URL/www literal in rendered output
// fused to a '_' (see fusedUnderscoreHazard). It scans with urlLiteralRe
// (the dotted host, scheme or "www." prefixed) and, separately,
// urlFusedSchemeHostRe (the dotless, scheme-prefixed host) so a dotless
// literal like "http://0" is recognized as the same hazard class a dotted
// one already is.
//
// THE DOTLESS PASS NEEDS TWO GUARDS urlLiteralRe's PASS NEVER DID:
//
//   - a match sitting inside an ALREADY-FORMED link's destination
//     ("[www..0](http://www..0)") is not a bare literal at all; the
//     token-from-match-start scan would otherwise walk past the
//     destination's closing ')' into the surrounding markdown (an
//     emphasis marker closing the link's own label) and misread that
//     marker as fused to the URL. urlLiteralRe's matches rarely reach
//     this, because its TLD requirement (a trailing "\.[a-z]+") excludes
//     most hrefs this harness constructs by hand; the dotless pattern has
//     no such requirement, so it reaches every href in sight and needs
//     the guard urlLiteralRe could go without.
//   - an underscore the SOURCE escaped ("a\_http://0") is inert text, not
//     an emphasis delimiter, and the renderer reproduces the escape on
//     every subsequent round (probe: "a\_http://0" and its unescaped
//     sibling "a_http://0" both render "a\\_http://0\n" and stay there) —
//     a stable shape, not the hazard this class describes. Because
//     urlLiteralRe demands a dot, a dotted host glued to an escaped
//     underscore never reaches this function today, so the guard was
//     never needed on that path.
func urlFusedWithUnderscore(first string) bool {
	for _, m := range urlLiteralRe.FindAllStringIndex(first, -1) {
		if fusedUnderscoreHazard(first, m[0], false) {
			return true
		}
	}
	// The dotless pass has no TLD requirement, so it also matches inside
	// a code span, a fenced code block, and a link title — three
	// contexts where goldmark's linkify extension never runs at all, so
	// a match there is not the hazard this class describes: the text is
	// inert literal content (a code span's and a fenced block's content
	// render verbatim, see formatCodeSpan and renderCodeBlock), or, for
	// a title, advisory text re-emitted as its own escaped unit, never
	// offered to linkify as prose either way. inertSpanRanges finds
	// those three WITHOUT reparsing — a byte-range scan over first
	// itself — so the match positions below stay first's own offsets
	// and escapedByteAt keeps seeing the actual escaped bytes the
	// renderer wrote, not a decoded stand-in that would make its
	// backslash-run parity meaningless. See fusedUnderscoreHazard's doc
	// comment for why this guard is scoped to the dotless pass only: a
	// dotted host glued to an escaped underscore never reaches this
	// pass's sibling above.
	inert := inertSpanRanges(first)
	for _, m := range urlFusedSchemeHostRe.FindAllStringIndex(first, -1) {
		if m[0] >= 2 && first[m[0]-2:m[0]] == "](" {
			continue
		}
		if inInertRange(inert, m[0]) {
			continue
		}
		if fusedUnderscoreHazard(first, m[0], true) {
			return true
		}
	}
	return false
}

// byteSpan is a half-open [start, end) byte range into a rendered string.
type byteSpan struct{ start, end int }

// inInertRange reports whether pos falls inside any of ranges.
func inInertRange(ranges []byteSpan, pos int) bool {
	for _, r := range ranges {
		if pos >= r.start && pos < r.end {
			return true
		}
	}
	return false
}

// inertSpanRanges finds the byte ranges of a rendered markdown string that
// hold verbatim or re-escaped-as-a-unit content: a code span's body, a
// fenced code block's body, and a link title. All three are written by
// this renderer as symmetric delimiter pairs (formatCodeSpan,
// renderCodeBlock, titleSegment), so they are findable by scanning first
// itself instead of correlating it back to a freshly re-parsed tree
// (first has no source positions to correlate against).
func inertSpanRanges(first string) []byteSpan {
	ranges := backtickFenceRanges(first)
	for _, m := range linkTitleRe.FindAllStringIndex(first, -1) {
		ranges = append(ranges, byteSpan{start: m[0], end: m[1]})
	}
	return ranges
}

// backtickFenceRanges pairs up runs of backtick characters by length,
// left to right, nearest-open-of-that-length first — the same rule that
// tells a code span's or a fenced code block's closing run apart from an
// unrelated backtick run elsewhere: formatCodeSpan picks the shortest
// fence length that does not already occur inside the content, and
// renderCodeBlock's fence is always the SAME length on both sides it
// writes, so this renderer's own output never closes a run with one of a
// different length. The returned ranges span from the start of the
// opening run to the end of the matching closing run (fences and
// content together), which is wide enough to exclude any match that
// starts inside, the only thing callers need.
func backtickFenceRanges(first string) []byteSpan {
	var ranges []byteSpan
	open := map[int]int{}
	for i := 0; i < len(first); {
		if first[i] != '`' {
			i++
			continue
		}
		j := i
		for j < len(first) && first[j] == '`' {
			j++
		}
		n := j - i
		if start, ok := open[n]; ok {
			ranges = append(ranges, byteSpan{start: start, end: j})
			delete(open, n)
		} else {
			open[n] = i
		}
		i = j
	}
	return ranges
}

// linkTitleRe matches a link or image title in this renderer's only
// non-prettier form (titleSegment: ` "escaped"` directly before the
// destination's closing ')') — a leading space or tab, a double-quoted
// span (escaped '"' and '\' allowed inside via the alternation, per
// remarkTitle), then the closing paren.
var linkTitleRe = regexp.MustCompile(`[ \t]"(?:[^"\\]|\\.)*"\)`)

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
// content line. remark renders the identical bytes for both (measured
// against remark-stringify), so neither shape is a renderer bug — see
// skipRenderedStructureClasses, which carries the probe input for the
// marker-chain case.
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
		// same bytes (measured against remark-stringify).
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
// goldmark parser divergence, not a renderer bug
// (probe: "- > -\n  >\n  > 0").
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
	// remark renders the identical shape and is equally unstable
	// (measured against remark-stringify), so the input class is out of
	// scope.
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
	// unstable (measured against remark-stringify).
	if hasLazyChainContinuation(first) {
		return "continuation after marker chain; remark is equally unstable", true
	}
	// Adjacent empty blockquotes ("- >\n  >") merge into one quote on
	// re-parse; remark renders the identical bytes and is equally
	// unstable (measured against remark-stringify).
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
	// quote); remark keeps one quote. A known goldmark parser
	// divergence, out of scope for the renderer.
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
	// is equally unstable (probes: ":www.0.a" → "\:www\.0.a",
	// ":www.0" → "\:www\.0", ":wwW.0" → "\:wwW\.0").
	//
	// THE REFERENCE IS UNSTABLE BYTE FOR BYTE, not merely "also wrong".
	// Measured on remark-parse 11.0.0 + remark-gfm + remark-directive
	// 3.0.0 + remark-stringify 11.0.0 by stringifying the hand-built
	// mdast this round trip produces — text ":www" then text ".0", which
	// is what a text directive with no ADF form degrades to — so no parse
	// step could reinterpret the string first:
	//
	//	stringify([text ":www", text ".0"])  ->  "\:www\.0\n"
	//	parse("\:www\.0")                    ->  text ":" + link http://www.0
	//	stringify(that)                      ->  ":[www.0](http://www.0)\n"
	//
	// Those are adfast's two renders exactly. The escape cannot hold in
	// either implementation: micromark's tokenizer does not linkify
	// "www\.0" from the raw source, and mdast-util-gfm-autolink-literal's
	// post-parse transform then reads the DECODED text, where the
	// backslash is already gone. A character reference does not help
	// either — "\:www&#46;0" and "&#58;www.0" both come back as the same
	// link from remark and from here — so the plain text ":www.0" has no
	// spelling in this dialect that survives a round trip.
	if escapedColonURLRe.MatchString(first) {
		return "escaped colon before URL literal; remark is equally unstable", true
	}
	// Any other rendered backslash escape directly before a www literal
	// is the same class: GFM autolink literals may follow the escaped
	// punctuation (cmark-gfm allows '_', '*', '~', '(' before "www."), so
	// the second parse linkifies what the first round — where the token
	// was still fused into a directive name — did not. remark renders the
	// identical escape bytes and re-linkifies the same way (probe:
	// ":0_www.0.a" → ":0\_www\.0.a"). An http(s) scheme literal does not
	// reach this pattern (urlLiteralOpenerPat is www-only); see its doc
	// comment for why the scheme form stopped needing this skip.
	if escapedPunctURLRe.MatchString(first) {
		return "escaped punctuation before URL literal; remark is equally unstable", true
	}
	// The same escape one byte deeper: INSIDE the scheme rather than in
	// front of the literal. colonBeforeSlashEscapes writes the remark
	// unsafe table's ':'-before-'/' row, so an emphasized "http://0.0"
	// goes out as "_http\://0.0_"; the decoded-text transform reads
	// "http://0.0" and links it, and the second round writes the angle
	// autolink. urlliteral.go states the reference half of this already —
	// "https\://ex.com" comes back from the reference as
	// "<https://ex.com>" — and the reference is unstable on the emphasized
	// shape in the same two steps, measured on the same install:
	//
	//	stringify(emphasis[text "http://0.0"])  ->  "*http\://0.0*\n"
	//	parse(that)                             ->  emphasis[link http://0.0]
	//	stringify(that)                         ->  "*<http://0.0>*\n"
	//
	// adfast writes the identical pair under its own emphasis delimiter
	// ("_http\://0.0_" then "_<http://0.0>_"). Probes: "*http://*0.0**",
	// "*http://.*0**" for the empty leading host segment, and
	// "*HTTp://*0.0**" for the unsafe row's lowercase-last-letter rule.
	if escapedSchemeColonIgnored(first) {
		return "escaped colon inside URL scheme; remark is equally unstable", true
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
	// An unescaped "www." host followed later by a second dot: the
	// after-class dotAfterWwwEscapes checks is one byte wide and cannot see
	// the second dot, so no escape is written and the host links on
	// re-parse; remark's identical one-byte-wide unsafe row is equally
	// unstable (probe: "*www.#.*").
	if wwwDotPunctRe.MatchString(first) {
		return "www host with a second dot past an unescaped byte; remark is equally unstable", true
	}
	// The escaped half of the same rule: the dot was escaped, but the
	// decoded-text linkify reads past the backslash anyway and a later dot
	// still completes a host; remark is equally unstable (probe: "*www..*0**").
	if wwwEscapedDotIgnored(first) {
		return "escaped www host dot is ignored; remark is equally unstable", true
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
	if reason, skip := skipRenderedLabelClasses(first); skip {
		return reason, skip
	}
	if reason, skip := skipRenderedLayoutClasses(first); skip {
		return reason, skip
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

// skipRenderedLabelClasses covers the rendered tokens whose re-parse
// diverges because of what sits inside a directive label or next to a
// character reference.
func skipRenderedLabelClasses(first string) (reason string, skip bool) {
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
	return "", false
}

// skipRenderedLayoutClasses covers the rendered shapes the line layout
// itself creates — wrapped lines, trailing breaks, and blank runs — where
// the reference pipeline is equally unstable.
func skipRenderedLayoutClasses(first string) (reason string, skip bool) {
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
