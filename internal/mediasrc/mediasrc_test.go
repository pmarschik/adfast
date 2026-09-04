package mediasrc

import (
	"fmt"
	"testing"
)

// The store lookups are guarded, and each guard is a decision the legs
// used to make differently or not at all. These pin the guards; the
// projection behavior they produce is pinned in convert's media parity
// tests.

func TestID(t *testing.T) {
	t.Parallel()
	store := func(ref string) (string, bool) { return "AID", ref == "assets/a.png" }
	cases := []struct {
		name string
		id   string
		path string
		res  func(string) (string, bool)
		want string
	}{
		{"a path resolves", "", "assets/a.png", store, "AID"},
		// An explicit id is the author's and outranks the store, which may
		// hold a different id for the same file after a re-upload.
		{"an explicit id wins", "MINE", "assets/a.png", store, "MINE"},
		{"an unknown path keeps nothing", "", "assets/other.png", store, ""},
		{"no path, nothing to resolve", "", "", store, ""},
		{"no resolver configured", "", "assets/a.png", nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := ID(c.id, c.path, c.res); got != c.want {
				t.Errorf("ID(%q, %q) = %q, want %q", c.id, c.path, got, c.want)
			}
		})
	}
}

func TestDims(t *testing.T) {
	t.Parallel()
	store := func(ref string) (int, int, bool) { return 10, 20, ref == "assets/a.png" }
	f := func(v float64) *float64 { return &v }
	cases := []struct {
		w, h  *float64
		res   func(string) (int, int, bool)
		wantW *float64
		wantH *float64
		name  string
		path  string
	}{
		{
			name: "both absent, the file measures them",
			path: "assets/a.png", res: store,
			wantW: f(10), wantH: f(20),
		},
		// Half a size is the author's, not the store's: filling the other
		// half would silently combine two sources, and overwriting the
		// given half would discard an explicit value.
		{
			name: "a width alone is left alone",
			w:    f(7), path: "assets/a.png", res: store,
			wantW: f(7),
		},
		{
			name: "a height alone is left alone",
			h:    f(7), path: "assets/a.png", res: store,
			wantH: f(7),
		},
		{name: "an unknown path measures nothing", path: "assets/other.png", res: store},
		{name: "no path, nothing to measure", res: store},
		{name: "no resolver configured", path: "assets/a.png"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			gotW, gotH := Dims(c.w, c.h, c.path, c.res)
			if !eq(gotW, c.wantW) || !eq(gotH, c.wantH) {
				t.Errorf("Dims(%s, %s, %q) = (%s, %s), want (%s, %s)",
					show(c.w), show(c.h), c.path, show(gotW), show(gotH), show(c.wantW), show(c.wantH))
			}
		})
	}
}

func eq(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func show(v *float64) string {
	if v == nil {
		return "nil"
	}
	return fmt.Sprintf("%g", *v)
}
