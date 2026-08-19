package adfast

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

// ---------------------------------------------------------------------------
// Extended ADF coverage: dates, placeholders, emoji fallbacks, captions,
// blockTaskItems, layouts, the extension family, synced blocks, and the
// mark mappings (annotation, fontSize, alignment, indentation, breakout,
// border, dataConsumer, fragment).
// ---------------------------------------------------------------------------

// assertAdfMdAdf renders doc, optionally checks the exact markdown (or
// substrings), re-encodes the markdown, and requires semantic ADF
// equality plus markdown-level stability.
func assertAdfMdAdf(t *testing.T, docIn adf.Doc, exact string, contains []string) {
	t.Helper()
	md := adfToMD(docIn)
	if exact != "" && md != exact {
		t.Fatalf("ToMarkdown:\n got: %q\nwant: %q", md, exact)
	}
	for _, s := range contains {
		if !strings.Contains(md, s) {
			t.Fatalf("ToMarkdown %q does not contain %q", md, s)
		}
	}
	docOut := mdToADF(md)
	assertSameADF(t, docIn, docOut)
	md2 := adfToMD(docOut)
	if md2 != md {
		t.Fatalf("md unstable:\nfirst:  %q\nsecond: %q", md, md2)
	}
}

// assertSameADF compares two docs through their canonical JSON.
func assertSameADF(t *testing.T, want, got adf.Doc) {
	t.Helper()
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wantJSON, gotJSON) {
		t.Fatalf("ADF diverged:\n got: %s\nwant: %s", gotJSON, wantJSON)
	}
}

// assertMdStable asserts FromMarkdown→ToMarkdown is idempotent for md.
func assertMdStable(t *testing.T, md string) {
	t.Helper()
	first := adfToMD(mdToADF(md))
	second := adfToMD(mdToADF(first))
	if first != second {
		t.Fatalf("round-trip unstable for %q:\nfirst:  %q\nsecond: %q", md, first, second)
	}
}

func dateTS(day string) string {
	d, err := time.Parse(time.DateOnly, day)
	if err != nil {
		panic(err)
	}
	return strconv.FormatInt(d.UnixMilli(), 10)
}

func TestExtended_DateAndPlaceholder(t *testing.T) {
	ts := dateTS("2026-07-15")
	assertAdfMdAdf(t,
		doc(p(txt("due "), &adf.Date{Timestamp: ts})),
		"due :date[2026-07-15]{timestamp=\""+ts+"\"}\n", nil)
	assertAdfMdAdf(t,
		doc(p(&adf.Date{Timestamp: ts, LocalID: "d1"})),
		":date[2026-07-15]{localId=\"d1\" timestamp=\""+ts+"\"}\n", nil)
	// The label alone (no timestamp attr) is parsed as a UTC day.
	got := mdToADF(":date[2026-07-15]")
	assertSameADF(t, doc(p(&adf.Date{Timestamp: ts})), got)
	// An unparsable timestamp keeps the attr but derives no label.
	weird := adfToMD(doc(p(&adf.Date{Timestamp: "not-a-number"})))
	if weird != ":date{timestamp=\"not-a-number\"}\n" {
		t.Fatalf("unparsable timestamp: %q", weird)
	}
	assertMdStable(t, weird)

	assertAdfMdAdf(t,
		doc(p(&adf.Placeholder{Text: "Type something…"})),
		":placeholder[Type something…]\n", nil)
	assertAdfMdAdf(t,
		doc(p(&adf.Placeholder{Text: "fill in", LocalID: "p1"})),
		":placeholder[fill in]{localId=\"p1\"}\n", nil)
}

func TestExtended_EmojiFallbacks(t *testing.T) {
	// text present: unchanged text projection (deliberately lossy).
	if got := adfToMD(doc(p(&adf.Emoji{Text: ptrOf("👍"), ShortName: ":thumbsup:"}))); got != "👍\n" {
		t.Fatalf("text-present emoji: %q", got)
	}
	// text absent, toolkit shortname: unicode restored.
	if got := adfToMD(doc(p(&adf.Emoji{ShortName: ":grinning:"}))); got != "😀\n" {
		t.Fatalf("shortname emoji: %q", got)
	}
	// text absent, custom emoji: the :emoji directive survives markdown.
	assertAdfMdAdf(t,
		doc(p(&adf.Emoji{ShortName: ":team_logo:", ID: "atlassian-abc"})),
		":emoji{#atlassian-abc shortName=\":team_logo:\"}\n", nil)
}

func TestExtended_AnnotationMark(t *testing.T) {
	assertAdfMdAdf(t,
		doc(p(txt("plain "), txt("noted", &adf.Annotation{ID: "a1", AnnotationType: "inlineComment"}))),
		"plain :annotation[noted]{#a1 annotationType=\"inlineComment\"}\n", nil)
	// Formatting inside the annotated span: strong groups outside, the
	// annotation wraps the leaf.
	assertAdfMdAdf(t,
		doc(p(txt("b", &adf.Strong{}, &adf.Annotation{ID: "a2", AnnotationType: "inlineComment"}))),
		"**:annotation[b]{#a2 annotationType=\"inlineComment\"}**\n", nil)
	// Overlapping comments degrade to the outermost anchor: directive
	// labels cannot nest brackets, so only the first annotation mark
	// projects (documented lossiness).
	overlapped := adfToMD(doc(p(txt("x",
		&adf.Annotation{ID: "outer", AnnotationType: "inlineComment"},
		&adf.Annotation{ID: "inner", AnnotationType: "inlineComment"},
	))))
	if overlapped != ":annotation[x]{#outer annotationType=\"inlineComment\"}\n" {
		t.Fatalf("overlapping annotations: %q", overlapped)
	}
	assertSameADF(t,
		doc(p(txt("x", &adf.Annotation{ID: "outer", AnnotationType: "inlineComment"}))),
		mdToADF(overlapped))
	// The annotationType defaults to inlineComment on encode.
	got := mdToADF(":annotation[hi]{#a3}")
	assertSameADF(t, doc(p(txt("hi", &adf.Annotation{ID: "a3", AnnotationType: "inlineComment"}))), got)
}

func TestExtended_FontSizeMark(t *testing.T) {
	// fontSize is RETIRED: no Atlassian product supports the mark, so
	// adfast never produces one. The directive still parses, but its
	// content unwraps to plain text on encode (and a legacy fontSize ADF
	// mark decodes to bare text), each with a fontsize-dropped diagnostic.

	// Encode: the directive drops the mark, keeps the text, and reports.
	var encDiags []convert.Diagnostic
	got := mdToADF(":fontSize[fine print]{small}",
		WithDiagnostics(func(d convert.Diagnostic) { encDiags = append(encDiags, d) }))
	assertSameADF(t, doc(p(txt("fine print"))), got)
	if len(encDiags) != 1 || encDiags[0].Code != convert.CodeFontSizeDropped {
		t.Fatalf("encode diagnostics: %+v", encDiags)
	}

	// The named form parses the same; still no mark.
	assertSameADF(t, doc(p(txt("x"))), mdToADF(":fontSize[x]{size=\"small\"}"))
	// Ambiguous bare payload already degraded to unstyled text.
	assertSameADF(t, doc(p(txt("x"))), mdToADF(":fontSize[x]{a b}"))

	// Decode: a legacy fontSize ADF mark decodes to bare text and reports.
	var decDiags []convert.Diagnostic
	md := adfToMD(doc(p(txt("fine print", &adf.FontSize{Size: "small"}))),
		WithDiagnostics(func(d convert.Diagnostic) { decDiags = append(decDiags, d) }))
	if md != "fine print\n" {
		t.Fatalf("decode fontSize to bare text: %q", md)
	}
	if len(decDiags) != 1 || decDiags[0].Code != convert.CodeFontSizeDropped {
		t.Fatalf("decode diagnostics: %+v", decDiags)
	}
}

func TestExtended_BlockMarkWrappers(t *testing.T) {
	assertAdfMdAdf(t,
		doc(&adf.Paragraph{Content: []adf.Node{txt("centered")}, Marks: []adf.Mark{&adf.Alignment{Align: "center"}}}),
		":::center\ncentered\n:::\n", nil)
	assertAdfMdAdf(t,
		doc(&adf.Heading{Level: 2, Content: []adf.Node{txt("h")}, Marks: []adf.Mark{&adf.Alignment{Align: "end"}}}),
		":::end\n## h\n:::\n", nil)
	assertAdfMdAdf(t,
		doc(&adf.Paragraph{Content: []adf.Node{txt("in")}, Marks: []adf.Mark{&adf.Indentation{Level: 2}}}),
		":::indent{2}\nin\n:::\n", nil)
	assertAdfMdAdf(t,
		doc(&adf.CodeBlock{Content: []adf.Node{txt("wide code")}, Marks: []adf.Mark{&adf.Breakout{Mode: "wide"}}}),
		":::breakout{wide}\n```\nwide code\n```\n:::\n", nil)
	assertAdfMdAdf(t,
		doc(&adf.CodeBlock{Content: []adf.Node{txt("c")}, Marks: []adf.Mark{&adf.Breakout{Mode: "full-width", Width: ptrOf(1200.0)}}}),
		":::breakout{full-width width=\"1200\"}\n```\nc\n```\n:::\n", nil)
	// Nested wrappers on one paragraph: the ADF mark order maps
	// inside-out (first mark innermost).
	assertAdfMdAdf(t,
		doc(&adf.Paragraph{
			Content: []adf.Node{txt("both")},
			Marks:   []adf.Mark{&adf.Alignment{Align: "center"}, &adf.Indentation{Level: 3}},
		}),
		"::::indent{3}\n:::center\nboth\n:::\n::::\n", nil)
}

func TestExtended_FragmentAndDataConsumer(t *testing.T) {
	table := func() *adf.Table {
		return &adf.Table{
			Marks: []adf.Mark{&adf.Fragment{LocalID: "frag-1", Name: "Metrics"}},
			Content: []adf.Node{
				&adf.TableRow{Content: []adf.Node{
					&adf.TableHeader{Colwidth: []float64{100}, Content: []adf.Node{p(txt("a"))}},
					&adf.TableHeader{Colwidth: []float64{200}, Content: []adf.Node{p(txt("b"))}},
				}},
			},
		}
	}
	assertAdfMdAdf(t, doc(table()), "", []string{
		":::fragment{localId=\"frag-1\" name=\"Metrics\"}",
		"::colwidths[100,200]",
		"| a | b |",
	})
	assertAdfMdAdf(t,
		doc(&adf.Extension{
			ExtensionType: "com.example.charts",
			ExtensionKey:  "chart",
			Marks:         []adf.Mark{&adf.DataConsumer{Sources: []string{"frag-1", "frag-2"}}},
		}),
		":::dataConsumer{sources=\"frag-1,frag-2\"}\n::extension{key=\"chart\" type=\"com.example.charts\"}\n:::\n", nil)
}

func TestExtended_ExtensionFamily(t *testing.T) {
	assertAdfMdAdf(t,
		doc(p(txt("v: "), &adf.InlineExtension{
			ExtensionType: "com.example",
			ExtensionKey:  "version",
			LocalID:       "L1",
		})),
		"v: :extension{key=\"version\" localId=\"L1\" type=\"com.example\"}\n", nil)
	assertAdfMdAdf(t,
		doc(&adf.Extension{
			ExtensionType: "com.example",
			ExtensionKey:  "toc",
			Layout:        "wide",
			Parameters:    map[string]any{"maxLevel": 3.0},
		}),
		"::extension{key=\"toc\" layout=\"wide\" parameters='{\"maxLevel\":3}' type=\"com.example\"}\n", nil)
	assertAdfMdAdf(t,
		doc(&adf.BodiedExtension{
			ExtensionType: "com.example",
			ExtensionKey:  "note",
			Content:       []adf.Node{p(txt("body"))},
		}),
		":::extension{key=\"note\" type=\"com.example\"}\nbody\n:::\n", nil)
	assertAdfMdAdf(t,
		doc(&adf.MultiBodiedExtension{
			ExtensionType: "com.example",
			ExtensionKey:  "tabs",
			Content: []adf.Node{
				&adf.ExtensionFrame{Content: []adf.Node{p(txt("one"))}},
				&adf.ExtensionFrame{Content: []adf.Node{p(txt("two"))}},
			},
		}),
		"::::extension{key=\"tabs\" type=\"com.example\"}\n:::frame\none\n:::\n\n:::frame\ntwo\n:::\n::::\n", nil)
	// A frameless multiBodiedExtension keeps its identity via multi.
	assertAdfMdAdf(t,
		doc(&adf.MultiBodiedExtension{ExtensionType: "com.example", ExtensionKey: "tabs"}),
		":::extension{key=\"tabs\" multi type=\"com.example\"}\n:::\n", nil)
}

func TestExtended_NastyExtensionParameters(t *testing.T) {
	params := map[string]any{
		"quote":   `he said "hi" & 'bye'`,
		"unicode": "täb\tnew\nline ☕",
		"nested":  map[string]any{"list": []any{1.0, "two", true, nil}},
		"empty":   "",
	}
	original := doc(&adf.Extension{
		ExtensionType: "com.example",
		ExtensionKey:  "nasty",
		Parameters:    params,
	})
	md := adfToMD(original)
	assertSameADF(t, original, mdToADF(md))
	assertMdStable(t, md)
	// Non-object parameters survive too (parameters is arbitrary JSON).
	arr := doc(&adf.Extension{
		ExtensionType: "com.example",
		ExtensionKey:  "arr",
		Parameters:    []any{"a", map[string]any{"b": 2.0}},
	})
	md = adfToMD(arr)
	assertSameADF(t, arr, mdToADF(md))
	// A hand-written non-JSON payload degrades to a JSON string value.
	got := mdToADF(`::extension{key="k" type="t" parameters="garbage"}`)
	assertSameADF(t, doc(&adf.Extension{ExtensionType: "t", ExtensionKey: "k", Parameters: "garbage"}), got)
}

func TestExtended_SyncBlocks(t *testing.T) {
	assertAdfMdAdf(t,
		doc(&adf.SyncBlock{ResourceID: "ari:cloud:sync/1", LocalID: "s1"}),
		"::syncBlock{localId=\"s1\" resourceId=\"ari:cloud:sync/1\"}\n", nil)
	assertAdfMdAdf(t,
		doc(&adf.BodiedSyncBlock{
			ResourceID: "ari:cloud:sync/1",
			LocalID:    "s1",
			Content:    []adf.Node{p(txt("source"))},
		}),
		":::syncBlock{localId=\"s1\" resourceId=\"ari:cloud:sync/1\"}\nsource\n:::\n", nil)
}

func TestExtended_Layouts(t *testing.T) {
	assertAdfMdAdf(t,
		doc(&adf.LayoutSection{Content: []adf.Node{
			&adf.LayoutColumn{Width: ptrOf(50.0), Content: []adf.Node{p(txt("left"))}},
			&adf.LayoutColumn{Width: ptrOf(50.0), Content: []adf.Node{p(txt("right"))}},
		}}),
		"::::section\n:::column{width=\"50\"}\nleft\n:::\n\n:::column{width=\"50\"}\nright\n:::\n::::\n", nil)
	assertAdfMdAdf(t,
		doc(&adf.LayoutSection{
			ColumnRuleStyle: "solid",
			Content: []adf.Node{
				&adf.LayoutColumn{Width: ptrOf(33.33), VAlign: "top", LocalID: "c1", Content: []adf.Node{p(txt("x"))}},
			},
		}),
		"::::section{columnRuleStyle=\"solid\"}\n:::column{localId=\"c1\" valign=\"top\" width=\"33.33\"}\nx\n:::\n::::\n", nil)
	// A breakout-marked layoutSection wraps outside the section.
	assertAdfMdAdf(t,
		doc(&adf.LayoutSection{
			Marks: []adf.Mark{&adf.Breakout{Mode: "wide"}},
			Content: []adf.Node{
				&adf.LayoutColumn{Width: ptrOf(100.0), Content: []adf.Node{p(txt("w"))}},
			},
		}),
		":::::breakout{wide}\n::::section\n:::column{width=\"100\"}\nw\n:::\n::::\n:::::\n", nil)
}

func TestExtended_BlockTaskItems(t *testing.T) {
	assertAdfMdAdf(t,
		doc(&adf.TaskList{LocalID: ptrOf(""), Content: []adf.Node{
			&adf.BlockTaskItem{LocalID: ptrOf(""), State: "TODO", Content: []adf.Node{
				p(txt("first")),
				p(txt("second paragraph")),
			}},
			&adf.TaskItem{LocalID: ptrOf(""), State: "DONE", Content: []adf.Node{txt("plain")}},
		}}),
		"- [ ] first\n\n  second paragraph\n- [x] plain\n", nil)
	// Nested blocks (a bullet list) ride under the checkbox item.
	assertAdfMdAdf(t,
		doc(&adf.TaskList{LocalID: ptrOf(""), Content: []adf.Node{
			&adf.BlockTaskItem{LocalID: ptrOf(""), State: "TODO", Content: []adf.Node{
				p(txt("head")),
				&adf.BulletList{Content: []adf.Node{li(p(txt("sub")))}},
			}},
		}}),
		"- [ ] head\n  - sub\n", nil)
}

func TestExtended_MediaCaptions(t *testing.T) {
	// Plain-text caption on an image-expressible media: the image title.
	assertAdfMdAdf(t,
		doc(&adf.MediaSingle{
			Layout: ptrOf("center"),
			Content: []adf.Node{
				&adf.Media{Type: "external", URL: "https://example.com/i.png", Alt: "alt"},
				&adf.Caption{Content: []adf.Node{txt("A caption")}},
			},
		}),
		"![alt](https://example.com/i.png \"A caption\")\n", nil)
	// A formatted caption keeps the :::media container form.
	assertAdfMdAdf(t,
		doc(&adf.MediaSingle{
			Layout: ptrOf("center"),
			Content: []adf.Node{
				&adf.Media{Type: "external", URL: "https://example.com/i.png", Alt: "alt"},
				&adf.Caption{Content: []adf.Node{txt("see "), txt("bold", &adf.Strong{})}},
			},
		}),
		":::media[alt]{layout=\"center\" type=\"external\" url=\"https://example.com/i.png\"}\nsee **bold**\n:::\n", nil)
	// A caption on media too rich for the image form keeps :::media too.
	assertAdfMdAdf(t,
		doc(&adf.MediaSingle{
			Layout: ptrOf("wide"),
			Content: []adf.Node{
				&adf.Media{Type: "external", URL: "https://example.com/i.png"},
				&adf.Caption{Content: []adf.Node{txt("plain")}},
			},
		}),
		":::media{layout=\"wide\" type=\"external\" url=\"https://example.com/i.png\"}\nplain\n:::\n", nil)
	// Multi-paragraph caption bodies join with hard breaks.
	md := ":::media[alt]{type=\"external\" url=\"https://example.com/i.png\"}\nline one\n\nline two\n:::\n"
	got := mdToADF(md)
	want := doc(&adf.MediaSingle{Content: []adf.Node{
		&adf.Media{Type: "external", URL: "https://example.com/i.png", Alt: "alt"},
		&adf.Caption{Content: []adf.Node{txt("line one"), &adf.HardBreak{}, txt("line two")}},
	}})
	assertSameADF(t, want, got)
}

func TestExtended_MediaBorder(t *testing.T) {
	assertAdfMdAdf(t,
		doc(&adf.MediaSingle{
			Layout: ptrOf("center"),
			Content: []adf.Node{&adf.Media{
				Type: "external", URL: "https://example.com/i.png", Alt: "a",
				Marks: []adf.Mark{&adf.Border{Color: "#091e42", Size: 2}},
			}},
		}),
		"::media[a]{borderColor=\"#091e42\" borderSize=\"2\" layout=\"center\" type=\"external\" url=\"https://example.com/i.png\"}\n", nil)
}

func TestExtended_UnknownDirectivesStillDegrade(t *testing.T) {
	// New names must not swallow other directives: an unknown container
	// keeps dissolving into its content.
	got := mdToADF(":::mystery\nhello\n:::")
	assertSameADF(t, doc(p(txt("hello"))), got)
	// A bare prose ":date" is consumed by the claimed name — the same
	// degradation the historical claimed names (:mention/:status/:u)
	// show. The space run the dropped construct leaves across the two
	// text nodes collapses (adf.NormalizeTextNewlines): markdown cannot
	// write it, so keeping it would break the round trip. The renderer
	// escapes colons on the way out, so ADF-sourced text never re-parses
	// as a directive.
	got = mdToADF("a :date in prose")
	assertSameADF(t, doc(p(txt("a "), txt("in prose"))), got)
	if md := adfToMD(doc(p(txt("a :date in prose")))); !strings.Contains(md, "\\:date") {
		t.Fatalf("prose colon not escaped: %q", md)
	}
}

func TestExtended_DecodeWireShapes(t *testing.T) {
	// Wire-level decode of the new kinds stays lossless.
	raw := `{"type":"doc","version":1,"content":[
		{"type":"layoutSection","content":[
			{"type":"layoutColumn","attrs":{"width":50},"content":[
				{"type":"paragraph","content":[{"type":"text","text":"x","marks":[
					{"type":"annotation","attrs":{"id":"a1","annotationType":"inlineComment"}}
				]}]}
			]}
		]},
		{"type":"extension","attrs":{"extensionType":"t","extensionKey":"k","parameters":{"z":1,"a":[true,null]}}},
		{"type":"date","attrs":{"timestamp":"1752537600000"}}
	]}`
	var input map[string]any
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		t.Fatal(err)
	}
	decoded, ok := adf.DecodeDoc(input)
	if !ok {
		t.Fatal("DecodeDoc failed")
	}
	if _, isTyped := decoded.Content[0].(*adf.LayoutSection); !isTyped {
		t.Fatalf("layoutSection not typed: %T", decoded.Content[0])
	}
	if _, isTyped := decoded.Content[1].(*adf.Extension); !isTyped {
		t.Fatalf("extension not typed: %T", decoded.Content[1])
	}
	out, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	var roundTripped map[string]any
	if err := json.Unmarshal(out, &roundTripped); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(input, roundTripped) {
		t.Fatalf("wire round trip diverged:\n got: %s", out)
	}
}
