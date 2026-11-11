package frontmatter_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/pmarschik/adfast/frontmatter"
)

// FuzzParseSequenceSourceText checks the source-text parse against Parse
// on arbitrary input: it never panics, reports the same error, and returns
// the same keys and structure, differing only where a sequence item that
// Parse decoded as a number, boolean or timestamp became a string.
func FuzzParseSequenceSourceText(f *testing.F) {
	for _, seed := range []string{
		"",
		"---\nv: [1.0, 2, true, ~, 2024-01-02]\n---",
		"a: &a [1.0]\nb: *a\n",
		"base: &b\n  t: [1.0]\nm:\n  <<: [*b, {t: [2]}]\n",
		"x: [&a 1.0, *a]\n",
		"\ufeffv: [1]\r\n",
		"{0x10: [1.0], \"0x10\": [2.0]}",
		"a: &a\n  <<: *a\n",
		"b: &b\n  t: [1.0]\nm:\n  <<: *b\n  t: [!!int 1]\n",
		"m:\n  <<: [{t: [!!int 1]}, {t: [2.0]}]\n",
		"v: !!set {1.0, 2}\n",
		"v: [!!str 1.0, !!binary MQ==, !!float 1]\n",
		"x: &k foo\n*k : [1.0]\nk: [2.0]\n",
		"k: &a 1.0\nl: [*a]\nx: [&b 2.0]\ny: *b\n",
		"v: [2024-01-02, !!timestamp 2024-01-02, 99999999999999999999, 0o17]\n",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		want, wantErr := frontmatter.Parse(in)
		got, err := frontmatter.ParseSequenceSourceText(in)
		if (err == nil) != (wantErr == nil) || (err != nil && err.Error() != wantErr.Error()) {
			t.Fatalf("err = %v, Parse err = %v", err, wantErr)
		}
		if err != nil {
			return
		}
		if msg := sourceTextDiff(want, got, false); msg != "" {
			t.Fatalf("%s\nParse: %#v\nSourceText: %#v", msg, want, got)
		}
	})
}

// sourceTextDiff returns "" when got is want with, at most, sequence-item
// scalars replaced by strings.
func sourceTextDiff(want, got any, inSeq bool) string {
	switch w := want.(type) {
	case map[string]any:
		return mapDiff(w, got)
	case []any:
		return sliceDiff(w, got)
	}
	return scalarDiff(want, got, inSeq)
}

func mapDiff(want map[string]any, got any) string {
	g, ok := got.(map[string]any)
	if !ok || len(g) != len(want) {
		return fmt.Sprintf("map shape differs: %#v vs %#v", want, got)
	}
	for k, wv := range want {
		gv, ok := g[k]
		if !ok {
			return fmt.Sprintf("key %q missing", k)
		}
		if msg := sourceTextDiff(wv, gv, false); msg != "" {
			return msg
		}
	}
	return ""
}

func sliceDiff(want []any, got any) string {
	g, ok := got.([]any)
	if !ok || len(g) != len(want) {
		return fmt.Sprintf("slice shape differs: %#v vs %#v", want, got)
	}
	for i := range want {
		if msg := sourceTextDiff(want[i], g[i], true); msg != "" {
			return msg
		}
	}
	return ""
}

func scalarDiff(want, got any, inSeq bool) string {
	if reflect.DeepEqual(want, got) || fmt.Sprintf("%#v", want) == fmt.Sprintf("%#v", got) {
		return ""
	}
	if _, isStr := got.(string); !inSeq || !isStr {
		return fmt.Sprintf("scalar %#v became %#v outside a sequence item", want, got)
	}
	switch want.(type) {
	case int, int64, uint64, float64, bool, time.Time:
		return ""
	}
	return fmt.Sprintf("scalar %#v (%T) must not change", want, want)
}
