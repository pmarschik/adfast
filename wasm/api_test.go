package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"strings"
	"testing"
	"unicode/utf16"

	adfast "github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/dialect"
)

// sliceUTF16 is the test's independent oracle for the offset contract: it
// indexes src the way JavaScript does (UTF-16 code units), NOT the way Go
// does (bytes). Every span assertion goes through it, so a byte/UTF-16
// mix-up in api.go shows up as garbled text rather than passing quietly.
func sliceUTF16(t *testing.T, src string, start, end int) string {
	t.Helper()
	u := utf16.Encode([]rune(src))
	if start < 0 || end > len(u) || start > end {
		t.Fatalf("span [%d,%d) out of range for a %d code unit source", start, end, len(u))
	}
	return string(utf16.Decode(u[start:end]))
}

type wantSpan struct {
	attrs map[string]string
	name  string
	text  string // the source the span must select, sliced as UTF-16
	level int
}

// scanSpanCase is one source document plus the spans ScanSpans must
// report for it, in document order.
type scanSpanCase struct {
	name string
	md   string
	want []wantSpan
}

// scanSpanCases is the corpus TestScanSpans runs. It sits beside the
// runner rather than inside it so that neither the table nor the
// assertions grow past what one screen shows.
func scanSpanCases() []scanSpanCase {
	return []scanSpanCase{{
		name: "no directives",
		md:   "# Heading\n\nJust prose with a : colon.\n",
		want: nil,
	}, {
		name: "leaf directive",
		md:   "::media[shot.png]{#abc width=\"512\"}\n",
		want: []wantSpan{{
			level: LevelLeaf, name: "media",
			text:  "::media[shot.png]{#abc width=\"512\"}",
			attrs: map[string]string{"id": "abc", "width": "512"},
		}},
	}, {
		name: "container with a label and attributes",
		md:   ":::note[A **label**]{title=\"Hi\"}\nbody\n:::\n",
		want: []wantSpan{{
			level: LevelContainer, name: "note",
			text:  ":::note[A **label**]{title=\"Hi\"}\nbody\n:::",
			attrs: map[string]string{"title": "Hi"},
		}},
	}, {
		name: "nested containers resolve to their own closing fences",
		md:   "::::outer\n:::inner\nbody\n:::\ntail\n::::\n",
		want: []wantSpan{{
			level: LevelContainer, name: "outer",
			text:  "::::outer\n:::inner\nbody\n:::\ntail\n::::",
			attrs: map[string]string{},
		}, {
			level: LevelContainer, name: "inner",
			text:  ":::inner\nbody\n:::",
			attrs: map[string]string{},
		}},
	}, {
		// A container the user has not finished typing emits NO CloseFence
		// at all; the extent runs to the end of the input.
		name: "unclosed container runs to the end of the source",
		md:   ":::info\nstill typing\n",
		want: []wantSpan{{
			level: LevelContainer, name: "info",
			text:  ":::info\nstill typing\n",
			attrs: map[string]string{},
		}},
	}, {
		name: "unclosed inner container is clamped to its parent",
		md:   "::::outer\n:::inner\nbody\n::::\nafter\n",
		want: []wantSpan{{
			level: LevelContainer, name: "outer",
			text:  "::::outer\n:::inner\nbody\n::::",
			attrs: map[string]string{},
		}, {
			level: LevelContainer, name: "inner",
			text:  ":::inner\nbody\n::::",
			attrs: map[string]string{},
		}},
	}, {
		name: "text directives",
		md:   "Feed :status[Done]{color=\"green\"} on :date[2026-04-12].\n",
		want: []wantSpan{{
			level: LevelText, name: "status",
			text:  ":status[Done]{color=\"green\"}",
			attrs: map[string]string{"color": "green"},
		}, {
			level: LevelText, name: "date",
			text:  ":date[2026-04-12]",
			attrs: map[string]string{},
		}},
	}, {
		// The offsets that break if anything reports bytes: every
		// character before the directives is multi-byte in UTF-8, and the
		// bee is a surrogate pair in UTF-16.
		name: "non-ASCII and emoji before inline directives",
		md:   "Héllo :status[Ready]{color=\"green\"} 🐝 :date[2026-04-12] end\n",
		want: []wantSpan{{
			level: LevelText, name: "status",
			text:  ":status[Ready]{color=\"green\"}",
			attrs: map[string]string{"color": "green"},
		}, {
			level: LevelText, name: "date",
			text:  ":date[2026-04-12]",
			attrs: map[string]string{},
		}},
	}, {
		name: "directives inside a container body and its label",
		md:   ":::info[Für 🐝]\nSee :status[Läuft]{color=\"blue\"} today.\n:::\n",
		want: []wantSpan{{
			level: LevelContainer, name: "info",
			text:  ":::info[Für 🐝]\nSee :status[Läuft]{color=\"blue\"} today.\n:::",
			attrs: map[string]string{},
		}, {
			level: LevelText, name: "status",
			text:  ":status[Läuft]{color=\"blue\"}",
			attrs: map[string]string{"color": "blue"},
		}},
	}}
}

func TestScanSpans(t *testing.T) {
	for _, tt := range scanSpanCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := ScanSpans(tt.md)
			if len(got) != len(tt.want) {
				t.Fatalf("ScanSpans(%q) returned %d spans, want %d: %+v", tt.md, len(got), len(tt.want), got)
			}
			for i, w := range tt.want {
				g := got[i]
				if g.Level != w.level || g.Name != w.name {
					t.Errorf("span %d: got level=%d name=%q, want level=%d name=%q", i, g.Level, g.Name, w.level, w.name)
				}
				if text := sliceUTF16(t, tt.md, g.Start, g.End); text != w.text {
					t.Errorf("span %d selects %q, want %q", i, text, w.text)
				}
				if !maps.Equal(g.Attrs, w.attrs) {
					t.Errorf("span %d attrs = %v, want %v", i, g.Attrs, w.attrs)
				}
			}
		})
	}
}

// TestScanSpans_OffsetsAreUTF16CodeUnits pins the documented contract
// numerically. Byte offsets for the same source are 7..36 and 42..59; if
// these ever come back as bytes, every CodeMirror decoration after the
// first non-ASCII character silently shifts.
func TestScanSpans_OffsetsAreUTF16CodeUnits(t *testing.T) {
	md := "Héllo :status[Ready]{color=\"green\"} 🐝 :date[2026-04-12] end\n"
	got := ScanSpans(md)
	want := [][2]int{{6, 35}, {39, 56}}
	if len(got) != len(want) {
		t.Fatalf("got %d spans, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Start != w[0] || got[i].End != w[1] {
			t.Errorf("span %d = [%d,%d), want [%d,%d)", i, got[i].Start, got[i].End, w[0], w[1])
		}
	}
}

// TestScanSpans_AttrsAreNeverNil keeps `span.attrs.color` safe to write on
// the JavaScript side without a guard.
func TestScanSpans_AttrsAreNeverNil(t *testing.T) {
	for _, s := range ScanSpans(":::info\nx\n:::\n\n::media[a.png]\n\na :date[2026-01-01] b\n") {
		if s.Attrs == nil {
			t.Errorf("span %+v has nil attrs", s)
		}
	}
	var v []Span
	if err := json.Unmarshal([]byte(mustJSON(t, ScanSpans("::media[a.png]\n"))), &v); err != nil {
		t.Fatalf("round-trip through JSON: %v", err)
	}
	if v[0].Attrs == nil {
		t.Error("attrs decoded as null; want an empty object")
	}
}

// TestScanSpans_Corpus runs the whole remark-reference directive corpus
// through ScanSpans and asserts the structural invariant every consumer
// relies on: the span selects source that actually opens the directive it
// names, at the colon count its level implies.
func TestScanSpans_Corpus(t *testing.T) {
	data, err := os.ReadFile("../testdata/directive_fixtures.json")
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}
	var fixtures struct {
		Markdown []struct {
			Md string `json:"md"`
		} `json:"markdown"`
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatalf("unmarshal fixtures: %v", err)
	}
	if len(fixtures.Markdown) == 0 {
		t.Fatal("fixtures empty")
	}
	total := 0
	for _, f := range fixtures.Markdown {
		spans := ScanSpans(f.Md)
		last := -1
		for _, s := range spans {
			total++
			if s.Start < last {
				t.Errorf("%q: spans are not in document order at %+v", f.Md, s)
			}
			last = s.Start
			text := sliceUTF16(t, f.Md, s.Start, s.End)
			// A container may open with MORE than three colons
			// (:::: nests a ::: inside it), so the level fixes the
			// minimum colon run, not the exact one.
			colons := len(text) - len(strings.TrimLeft(text, ":"))
			if colons != s.Level && (s.Level != LevelContainer || colons < LevelContainer) {
				t.Errorf("%q: span %+v opens with %d colons, which does not match level %d",
					f.Md, s, colons, s.Level)
			}
			if !strings.HasPrefix(text[colons:], s.Name) {
				t.Errorf("%q: span %+v selects %q, which does not name %q",
					f.Md, s, text, s.Name)
			}
		}
	}
	if total == 0 {
		t.Fatal("the corpus produced no spans at all")
	}
	t.Logf("scanned %d directive spans across %d corpus documents", total, len(fixtures.Markdown))
}

// TestCatalog derives the expected entries from dialect.Registrations()
// the same way api.go does, but independently: the point is not to prove
// the two loops agree, it is to prove the catalog is a FUNCTION of the
// registrations — every registered (name, level) shows up exactly once,
// nothing that is not registered shows up at all, and the last
// registration wins where two claim the same pair (as markdown.Parse's
// promotion index does).
func TestCatalog(t *testing.T) {
	type key struct {
		name  string
		level int
	}
	want := map[key]CatalogEntry{}
	for _, reg := range dialect.Registrations() {
		for _, m := range []struct {
			names map[string]bool
			level int
		}{
			{namesOf(reg.Texts), LevelText},
			{namesOf(reg.Leaves), LevelLeaf},
			{namesOf(reg.Containers), LevelContainer},
		} {
			for name := range m.names {
				want[key{name, m.level}] = CatalogEntry{
					Name: name, Level: m.level, Kind: reg.Kind, DecodedByCore: reg.DecodedByCore,
				}
			}
		}
	}
	if len(want) == 0 {
		t.Fatal("the dialect registered nothing at all")
	}

	got := Catalog()
	seen := map[key]bool{}
	for _, e := range got {
		k := key{e.Name, e.Level}
		if seen[k] {
			t.Errorf("%q at level %d appears more than once", e.Name, e.Level)
		}
		seen[k] = true
		w, ok := want[k]
		if !ok {
			t.Errorf("catalog lists %q at level %d, which no registration claims", e.Name, e.Level)
			continue
		}
		if e != w {
			t.Errorf("catalog entry for %q at level %d = %+v, want %+v", e.Name, e.Level, e, w)
		}
	}
	for k := range want {
		if !seen[k] {
			t.Errorf("registered directive %q at level %d is missing from the catalog", k.name, k.level)
		}
	}

	for i := 1; i < len(got); i++ {
		prev, cur := got[i-1], got[i]
		if prev.Name > cur.Name || (prev.Name == cur.Name && prev.Level >= cur.Level) {
			t.Errorf("catalog is not sorted by (name, level): %+v precedes %+v", prev, cur)
		}
	}
	t.Logf("catalog lists %d directive registrations", len(got))
}

// TestCatalog_Pins nails the two properties the derivation could satisfy
// vacuously: a name registered at several levels keeps a per-level kind,
// and decodedByCore is carried through rather than hard-coded false.
func TestCatalog_Pins(t *testing.T) {
	byKey := map[string]CatalogEntry{}
	for _, e := range Catalog() {
		byKey[fmt.Sprintf("%s/%d", e.Name, e.Level)] = e
	}
	tests := []struct {
		key  string
		want CatalogEntry
	}{
		{"media/1", CatalogEntry{Name: "media", Level: LevelText, Kind: "mediaInline"}},
		{"media/2", CatalogEntry{Name: "media", Level: LevelLeaf, Kind: "media"}},
		{"media/3", CatalogEntry{Name: "media", Level: LevelContainer, Kind: "media"}},
		{"info/3", CatalogEntry{Name: "info", Level: LevelContainer, Kind: "panel"}},
		{"warning/3", CatalogEntry{Name: "warning", Level: LevelContainer, Kind: "panel"}},
		{"colwidths/2", CatalogEntry{Name: "colwidths", Level: LevelLeaf, Kind: "colwidths", DecodedByCore: true}},
		{"decisions/2", CatalogEntry{Name: "decisions", Level: LevelLeaf, Kind: "decisions", DecodedByCore: true}},
		{"u/1", CatalogEntry{Name: "u", Level: LevelText, Kind: "underline"}},
	}
	for _, tt := range tests {
		got, ok := byKey[tt.key]
		if !ok {
			t.Errorf("%s is missing from the catalog", tt.key)
			continue
		}
		if got != tt.want {
			t.Errorf("%s = %+v, want %+v", tt.key, got, tt.want)
		}
	}
}

func TestBridgeCatalog(t *testing.T) {
	out, err := bridgeCatalog()
	if err != nil {
		t.Fatalf("bridgeCatalog: %v", err)
	}
	var entries []struct {
		Name          string `json:"name"`
		Kind          string `json:"kind"`
		Level         int    `json:"level"`
		DecodedByCore bool   `json:"decodedByCore"`
	}
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if len(entries) != len(Catalog()) {
		t.Errorf("bridge returned %d entries, Catalog has %d", len(entries), len(Catalog()))
	}
	for _, e := range entries {
		if e.Name == "" || e.Kind == "" {
			t.Errorf("entry %+v has an empty name or kind", e)
		}
		if e.Level < LevelText || e.Level > LevelContainer {
			t.Errorf("entry %+v has an out-of-range level", e)
		}
	}
}

// namesOf collects the keys of a registration map whose value type differs
// per level, so the table above can treat all three levels alike.
func namesOf[V any](m map[string]V) map[string]bool {
	out := make(map[string]bool, len(m))
	for name := range m {
		out[name] = true
	}
	return out
}

func TestOptions_EncodeAndRender(t *testing.T) {
	tests := []struct {
		name    string
		opts    Options
		wantErr string
	}{
		{name: "absent product", opts: Options{}},
		{name: "confluence", opts: Options{Product: ProductConfluence, BaseURL: "https://hive.example.org/wiki"}},
		{name: "jira with a default expand mode", opts: Options{Product: ProductJira, BaseURL: "https://hive.example.org"}},
		{name: "jira explicit", opts: Options{Product: ProductJira, BaseURL: "https://hive.example.org", ExpandMode: "explicit"}},
		{name: "jira all", opts: Options{Product: ProductJira, ExpandMode: "all"}},
		{name: "unknown product", opts: Options{Product: "bitbucket"}, wantErr: `unknown product "bitbucket"`},
		{name: "unknown expand mode", opts: Options{Product: ProductJira, ExpandMode: "sometimes"}, wantErr: `unknown expandMode "sometimes"`},
		{name: "expand mode without jira is ignored", opts: Options{ExpandMode: "sometimes"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, dir := range []struct {
				fn    func() ([]adfast.Option, error)
				label string
			}{
				{tt.opts.encodeOptions, "encode"},
				{tt.opts.renderOptions, "render"},
			} {
				got, err := dir.fn()
				switch {
				case tt.wantErr == "" && err != nil:
					t.Errorf("%s: unexpected error: %v", dir.label, err)
				case tt.wantErr != "" && err == nil:
					t.Errorf("%s: want an error containing %q, got %d options", dir.label, tt.wantErr, len(got))
				case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
					t.Errorf("%s: error = %q, want it to contain %q", dir.label, err, tt.wantErr)
				case tt.wantErr == "" && tt.opts.Product == "" && got != nil:
					t.Errorf("%s: an absent product must map to no options, got %d", dir.label, len(got))
				case tt.wantErr == "" && tt.opts.Product != "" && len(got) == 0:
					t.Errorf("%s: product %q mapped to no options", dir.label, tt.opts.Product)
				}
			}
		})
	}
}

func TestToADF(t *testing.T) {
	md := ":::info\nFeed :status[Done]{color=\"green\"} on :date[2026-04-12].\n:::\n"
	got, err := ToADF(md, Options{})
	if err != nil {
		t.Fatalf("ToADF: %v", err)
	}
	for _, want := range []string{`"type":"panel"`, `"panelType":"info"`, `"type":"status"`, `"type":"date"`} {
		if !strings.Contains(got, want) {
			t.Errorf("ToADF output is missing %s:\n%s", want, got)
		}
	}
}

// TestToADF_ProductBundleApplies proves the opts object actually reaches
// the product bundle rather than being decoration: only the Jira bundle
// turns a bare issue key into an inlineCard.
func TestToADF_ProductBundleApplies(t *testing.T) {
	md := "See BEE-42 today.\n"
	neutral, err := ToADF(md, Options{})
	if err != nil {
		t.Fatalf("ToADF neutral: %v", err)
	}
	if strings.Contains(neutral, "inlineCard") {
		t.Errorf("the neutral bundle expanded a bare issue key:\n%s", neutral)
	}
	withJira, err := ToADF(md, Options{Product: ProductJira, BaseURL: "https://hive.example.org"})
	if err != nil {
		t.Fatalf("ToADF jira: %v", err)
	}
	if !strings.Contains(withJira, "inlineCard") {
		t.Errorf("the jira bundle did not expand a bare issue key:\n%s", withJira)
	}
}

func TestToADF_RejectsUnknownProduct(t *testing.T) {
	if _, err := ToADF("x\n", Options{Product: "notion"}); err == nil {
		t.Fatal("want an error for an unknown product")
	}
}

func TestToMarkdown(t *testing.T) {
	adfJSON, err := ToADF("# Rooftop apiary\n\nFeed :status[Done]{color=\"green\"}.\n", Options{})
	if err != nil {
		t.Fatalf("ToADF: %v", err)
	}
	got, err := ToMarkdown(adfJSON, Options{})
	if err != nil {
		t.Fatalf("ToMarkdown: %v", err)
	}
	want := "# Rooftop apiary\n\nFeed :status[Done]{color=\"green\"}.\n"
	if got != want {
		t.Errorf("ToMarkdown = %q, want %q", got, want)
	}
}

func TestToMarkdown_Errors(t *testing.T) {
	tests := []struct {
		name, in, wantErr string
	}{
		{"not json", "{nope", "parsing ADF JSON"},
		{"a bare array", `[1,2,3]`, "not an ADF document"},
		{"a bare string", `"hello"`, "not an ADF document"},
		{"null", `null`, "not an ADF document"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ToMarkdown(tt.in, Options{})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestDiagnostics(t *testing.T) {
	// An unsupported code language is a product-aware notice, so it only
	// arises once the opts object selects a product.
	md := "```klingon\nqapla'\n```\n"
	neutral, err := Diagnostics(md, Options{})
	if err != nil {
		t.Fatalf("Diagnostics neutral: %v", err)
	}
	if len(neutral) != 0 {
		t.Errorf("neutral diagnostics = %+v, want none", neutral)
	}
	got, err := Diagnostics(md, Options{Product: ProductConfluence})
	if err != nil {
		t.Fatalf("Diagnostics confluence: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("want at least one diagnostic for an unsupported code language")
	}
	if got[0].Code == "" || got[0].Message == "" {
		t.Errorf("diagnostic %+v is missing its code or message", got[0])
	}
	if got[0].Start != nil || got[0].End != nil {
		t.Errorf("diagnostic %+v invented offsets; adfast diagnostics carry none yet", got[0])
	}
}

// TestDiagnostics_EmptyIsAnArray keeps the JS side from having to handle
// `null` where it asked for a list.
func TestDiagnostics_EmptyIsAnArray(t *testing.T) {
	got, err := bridgeDiagnostics("clean document\n", "")
	if err != nil {
		t.Fatalf("bridgeDiagnostics: %v", err)
	}
	if got != "[]" {
		t.Errorf("bridgeDiagnostics = %q, want %q", got, "[]")
	}
}

func TestBridgeOptions(t *testing.T) {
	tests := []struct {
		name, in string
		want     Options
		wantErr  bool
	}{
		{name: "empty", in: "", want: Options{}},
		{name: "null", in: "null", want: Options{}},
		{name: "undefined", in: "undefined", want: Options{}},
		{name: "empty object", in: "{}", want: Options{}},
		{
			name: "full", in: `{"product":"jira","baseUrl":"https://h.example.org","expandMode":"all"}`,
			want: Options{Product: "jira", BaseURL: "https://h.example.org", ExpandMode: "all"},
		},
		{name: "malformed", in: "{", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := bridgeOptions(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want an error for %q", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("bridgeOptions(%q): %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("bridgeOptions(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestBridgeResult(t *testing.T) {
	ok := bridgeResult("value", nil)
	if ok["ok"] != true || ok["value"] != "value" {
		t.Errorf("success result = %v", ok)
	}
	if _, present := ok["error"]; present {
		t.Errorf("success result carries an error key: %v", ok)
	}
	bad := bridgeResult("", ErrNotADocument)
	if bad["ok"] != false || bad["error"] != ErrNotADocument.Error() {
		t.Errorf("error result = %v", bad)
	}
}

// TestBridgeGuard_RecoversPanics is the whole reason bridgeGuard exists: a
// panic reaching a js.FuncOf boundary tears down the WASM instance, and
// the page can only recover by reloading.
func TestBridgeGuard_RecoversPanics(t *testing.T) {
	got := bridgeGuard(func() (string, error) { panic("boom") })
	if got["ok"] != false {
		t.Fatalf("guard result = %v, want ok:false", got)
	}
	msg, ok := got["error"].(string)
	if !ok || !strings.Contains(msg, "boom") {
		t.Errorf("guard error = %q, want it to mention the panic", msg)
	}
}

func TestBridgeExports(t *testing.T) {
	spans, err := bridgeScanSpans("::media[a.png]\n")
	if err != nil {
		t.Fatalf("bridgeScanSpans: %v", err)
	}
	if !strings.Contains(spans, `"name":"media"`) {
		t.Errorf("bridgeScanSpans = %s", spans)
	}

	doc, err := bridgeToADF("hello\n", `{"product":"confluence"}`)
	if err != nil {
		t.Fatalf("bridgeToADF: %v", err)
	}
	back, err := bridgeToMarkdown(doc, `{"product":"confluence"}`)
	if err != nil {
		t.Fatalf("bridgeToMarkdown: %v", err)
	}
	if back != "hello\n" {
		t.Errorf("round trip through the bridge = %q, want %q", back, "hello\n")
	}

	if _, err := bridgeToADF("x\n", "{"); err == nil {
		t.Error("want an error for malformed opts JSON")
	}
	if _, err := bridgeDiagnostics("x\n", `{"product":"gitlab"}`); err == nil {
		t.Error("want an error for an unknown product")
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	s, err := marshalJSON(v)
	if err != nil {
		t.Fatalf("marshalJSON: %v", err)
	}
	return s
}
