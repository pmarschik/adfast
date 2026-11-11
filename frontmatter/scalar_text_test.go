package frontmatter_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/pmarschik/adfast/frontmatter"
)

func TestParseSequenceSourceTextKeepsItemSpelling(t *testing.T) {
	got, err := frontmatter.ParseSequenceSourceText(
		"---\nv: [1.0, 2, \"3.0\", 1.50, 0x1F, 1_000, true, false, ~, null, 2024-01-02, x]\n---")
	if err != nil {
		t.Fatal(err)
	}
	want := []any{"1.0", "2", "3.0", "1.50", "0x1F", "1_000", "true", "false", nil, nil, "2024-01-02", "x"}
	if !reflect.DeepEqual(got["v"], want) {
		t.Errorf("v = %#v\nwant %#v", got["v"], want)
	}
}

func TestParseSequenceSourceTextEmptySlotStaysNil(t *testing.T) {
	got, err := frontmatter.ParseSequenceSourceText("v:\n  - 1.0\n  -\n  - 2\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := []any{"1.0", nil, "2"}; !reflect.DeepEqual(got["v"], want) {
		t.Errorf("v = %#v, want %#v", got["v"], want)
	}
}

func TestParseSequenceSourceTextKeyLevelScalarsStayTyped(t *testing.T) {
	got, err := frontmatter.ParseSequenceSourceText("points: 5.0\ncount: 3\nok: true\nnothing: ~\nname: \"1.0\"\n")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"points": 5.0, "count": 3, "ok": true, "nothing": nil, "name": "1.0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestParseSequenceSourceTextNesting(t *testing.T) {
	got, err := frontmatter.ParseSequenceSourceText(
		"a:\n  tags:\n    - 7\n    - [1.0, [2.50]]\n    - {n: 3, l: [4.0]}\n")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"a": map[string]any{"tags": []any{
		"7",
		[]any{"1.0", []any{"2.50"}},
		map[string]any{"n": 3, "l": []any{"4.0"}},
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

func TestParseSequenceSourceTextAliasesAndMerges(t *testing.T) {
	tests := []struct {
		want any
		name string
		fm   string
		path []string
	}{
		{[]any{"1.0", "2"}, "aliased list", "base: &b [1.0, 2]\nother: *b\n", []string{"other"}},
		{[]any{"1.0", "1.0"}, "aliased item", "x: [&a 1.0, *a]\n", []string{"x"}},
		{[]any{"1.0"}, "merged mapping", "base: &b\n  tags: [1.0]\nm:\n  <<: *b\n", []string{"m", "tags"}},
		{[]any{"1.0"}, "merged mapping list, earlier wins", "a: &a\n  t: [1.0]\nb: &b\n  t: [2.0]\nm:\n  <<: [*a, *b]\n", []string{"m", "t"}},
		{[]any{"x"}, "own key wins over merge", "base: &b\n  tags: [1.0]\nm:\n  <<: *b\n  tags: ['x']\n", []string{"m", "tags"}},
		{[]any{"7"}, "own numeric list wins over merged list", "base: &b\n  tags: [1.0]\nm:\n  <<: *b\n  tags: [7]\n", []string{"m", "tags"}},
		{[]any{nil}, "own shorter list vs longer merged list", "base: &b\n  tags: [1.0, 2.0]\nm:\n  <<: *b\n  tags: [~]\n", []string{"m", "tags"}},
		{[]any{1}, "shadowed merged list does not rewrite a tagged own item", "base: &b\n  tags: [1.0]\nm:\n  <<: *b\n  tags: [!!int 1]\n", []string{"m", "tags"}},
		{[]any{1}, "shadowed later merged list does not rewrite a tagged item", "m:\n  <<: [{t: [!!int 1]}, {t: [2.0]}]\n", []string{"m", "t"}},
		{[]any{"1.0"}, "own key list beside merge", "base: &b\n  other: 1\nm:\n  <<: *b\n  tags: [1.0]\n", []string{"m", "tags"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := frontmatter.ParseSequenceSourceText(tt.fm)
			if err != nil {
				t.Fatal(err)
			}
			var v any = got
			for _, key := range tt.path {
				m, ok := v.(map[string]any)
				if !ok {
					t.Fatalf("path %v: %#v is not a mapping", tt.path, v)
				}
				v = m[key]
			}
			if !reflect.DeepEqual(v, tt.want) {
				t.Errorf("got %#v, want %#v", v, tt.want)
			}
		})
	}
}

// An alias used as a mapping key names its anchor, not the key it decodes
// to. Matching decoded keys by the key node's text once paired the alias
// key's list with an unrelated key spelled like the anchor, and wrote one
// list's spelling into the other.
func TestParseSequenceSourceTextAliasKeyDoesNotCrossLists(t *testing.T) {
	got, err := frontmatter.ParseSequenceSourceText("x: &k foo\n*k : [1.0]\nk: [2.0]\n")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"x": "foo", "foo": []any{"1.0"}, "k": []any{"2.0"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

// Every non-string scalar kind the decoder resolves comes back as text
// when it is a sequence item; the same spellings directly under a key
// stay typed, and an explicit tag keeps its type.
func TestParseSequenceSourceTextScalarKinds(t *testing.T) {
	got, err := frontmatter.ParseSequenceSourceText(
		"v: [2024-01-02, 2001-12-14t21:59:43.10-05:00, 99999999999999999999, 1e3, .inf, 0o17, +5, -0, yes, !!timestamp 2024-01-02, !!float 1, !!str 1.0]\n" +
			"due: 2024-01-02\nbig: 99999999999999999999\n")
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2024, time.January, 2, 0, 0, 0, 0, time.UTC)
	want := []any{
		"2024-01-02", "2001-12-14t21:59:43.10-05:00", "99999999999999999999", "1e3", ".inf",
		"0o17", "+5", "-0", "yes", day, 1.0, "1.0",
	}
	if !reflect.DeepEqual(got["v"], want) {
		t.Errorf("v = %#v\nwant %#v", got["v"], want)
	}
	if got["due"] != day {
		t.Errorf("due = %#v, want the time.Time %v", got["due"], day)
	}
	if got["big"] != 1e20 {
		t.Errorf("big = %#v, want the float 1e20", got["big"])
	}
}

// An anchored sequence item aliased under a key: the key-level alias stays
// typed, the item and an alias item to a key-level anchor become text.
func TestParseSequenceSourceTextAnchoredScalarAcrossContexts(t *testing.T) {
	got, err := frontmatter.ParseSequenceSourceText("k: &a 1.0\nl: [*a]\nx: [&b 2.0]\ny: *b\n")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"k": 1.0, "l": []any{"1.0"}, "x": []any{"2.0"}, "y": 2.0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

// Block sequences, block scalars and sequences of mappings follow the same
// rules as flow sequences.
func TestParseSequenceSourceTextBlockForms(t *testing.T) {
	got, err := frontmatter.ParseSequenceSourceText(
		"v:\n  - 01\n  - |\n    1.0\n  - name: 2.0\n    tags:\n      - 3.0\n  - - 4.0\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []any{
		"01",
		"1.0\n",
		map[string]any{"name": 2.0, "tags": []any{"3.0"}},
		[]any{"4.0"},
	}
	if !reflect.DeepEqual(got["v"], want) {
		t.Errorf("v = %#v\nwant %#v", got["v"], want)
	}
}

// A merge key whose source is not a mapping makes the decoder fail; the
// error must surface exactly as Parse reports it.
func TestParseSequenceSourceTextErrorMatchesParse(t *testing.T) {
	for _, in := range []string{"a: [1.0\n", "m:\n  <<: 5\n"} {
		_, wantErr := frontmatter.Parse(in)
		_, err := frontmatter.ParseSequenceSourceText(in)
		if (err == nil) != (wantErr == nil) || (err != nil && err.Error() != wantErr.Error()) {
			t.Errorf("%q: err = %v, Parse err = %v", in, err, wantErr)
		}
	}
}

func TestParseSequenceSourceTextFences(t *testing.T) {
	want := map[string]any{"v": []any{"1.0"}}
	for name, in := range map[string]string{
		"fenced":       "---\nv: [1.0]\n---",
		"fenced crlf":  "---\r\nv: [1.0]\r\n---\r\n",
		"fenceless":    "v: [1.0]",
		"leading nl":   "\n---\nv: [1.0]\n---\n",
		"trailing doc": "---\nv: [1.0]\n---\nv: [2.0]\n",
	} {
		got, err := frontmatter.ParseSequenceSourceText(in)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %#v, want %#v", name, got, want)
		}
	}
	for _, in := range []string{"", "---\n---", "  \n", "# only a comment\n"} {
		got, err := frontmatter.ParseSequenceSourceText(in)
		if err != nil || got == nil || len(got) != 0 {
			t.Errorf("%q: got %#v, %v; want an empty map", in, got, err)
		}
	}
}

// Parse itself must keep decoding typed values: the new entry point is
// opt-in.
func TestParseStaysTyped(t *testing.T) {
	got, err := frontmatter.Parse("v: [1.0, 2, true]\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := []any{1.0, 2, true}; !reflect.DeepEqual(got["v"], want) {
		t.Errorf("Parse v = %#v, want %#v", got["v"], want)
	}
}

// PatchPreserving reads the block itself, never through the source-text
// parse, so a list's authored spelling survives a write.
func TestSourceTextDoesNotDisturbWrites(t *testing.T) {
	block := "---\nv: [1.0, 2, 0x1F]\nstatus: Open\n---"
	out, err := frontmatter.PatchPreserving(block, map[string]any{"status": "Done"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "---\nv: [1.0, 2, 0x1F]\nstatus: Done\n---"; out != want {
		t.Errorf("PatchPreserving = %q, want %q", out, want)
	}
}
