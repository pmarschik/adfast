package dialect

import (
	"maps"
	"testing"

	"github.com/pmarschik/adfast/ast"
)

// TestRewriteMediaPath: the write half of MediaLookupPath's read. The
// path lives in the raw attribute map, and the leaf form ALSO binds it to
// a typed field — so a write that moves only the map leaves the node
// contradicting itself, and a consumer reading Media.Path is told the
// file is somewhere it is not.
//
// Drop the typed-field arm (the `bound` write) and the leaf row goes red,
// the caption and chip rows staying green because they have no such field
// by design:
//
//	--- FAIL: TestRewriteMediaPath/the_block_form (0.00s)
//	    mediapath_test.go:61: Media.Path = "assets/old.png", want "assets/new.png" — the typed field drifted from the attrs map
//	    (once for each of the three attribute shapes)
//
// Give this function MediaLookupPath's inert-source guard instead, so a
// pinned id or external addressing holds the write off, and the id and
// external shapes of all three kinds go red:
//
//	--- FAIL: TestRewriteMediaPath/the_block_form (0.00s)
//	    mediapath_test.go:61: RewriteMediaPath(map[id:0a1b2c3d-1111-2222-3333-444455556666 path:assets/old.png]) declined the path
//	    (the same in .../the_caption_form and .../the_inline_chip)
func TestRewriteMediaPath(t *testing.T) {
	for _, tc := range []struct {
		node func(attrs map[string]string) ast.Node
		name string
	}{
		{
			name: "the block form",
			node: func(attrs map[string]string) ast.Node { return NewMedia(attrs, nil) },
		},
		{
			name: "the caption form",
			node: func(attrs map[string]string) ast.Node { return &MediaCaption{Attrs: attrs} },
		},
		{
			name: "the inline chip",
			node: func(attrs map[string]string) ast.Node { return &MediaInline{Attrs: attrs} },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// An explicit id and external addressing must NOT hold the
			// rewrite off: neither of them moves a file. This is where
			// RewriteMediaPath deliberately answers for more directives
			// than MediaLookupPath.
			for _, extra := range []map[string]string{
				{},
				{"id": "0a1b2c3d-1111-2222-3333-444455556666"},
				{"type": "external"},
			} {
				attrs := map[string]string{"path": "assets/old.png"}
				maps.Copy(attrs, extra)
				wantRewritten(t, tc.node(attrs), attrs)
			}
		})
	}
}

// wantRewritten drives one media node through RewriteMediaPath and checks
// that the mapping saw the old path and that every home of the new one
// agrees: the return value, the attribute map, and the typed field the
// leaf form binds it to.
func wantRewritten(t *testing.T, n ast.Node, attrs map[string]string) {
	t.Helper()
	var seen string
	got, ok := RewriteMediaPath(n, func(path string) string {
		seen = path
		return "assets/new.png"
	})
	if !ok {
		t.Fatalf("RewriteMediaPath(%v) declined the path", attrs)
	}
	if seen != "assets/old.png" {
		t.Errorf("rewrite saw %q, want %q", seen, "assets/old.png")
	}
	if got != "assets/new.png" {
		t.Errorf("returned %q, want %q", got, "assets/new.png")
	}
	if attrs["path"] != "assets/new.png" {
		t.Errorf("attrs[path] = %q, want %q", attrs["path"], "assets/new.png")
	}
	if media, isLeaf := n.(*Media); isLeaf && media.Path != "assets/new.png" {
		t.Errorf("Media.Path = %q, want %q — the typed field drifted from the attrs map",
			media.Path, "assets/new.png")
	}
}

// TestRewriteMediaPath_NothingToRewrite: a node with no path of its own,
// and a node that is not a media directive at all, must both decline —
// without calling the mapping, which would otherwise be handed "" and
// could write a store path onto a directive that named no file.
//
// Drop the empty-path guard and every media row goes red, the image and
// panel rows staying green because the kind switch already declined them
// — and the node built without an Attrs map at all does not merely fail,
// it takes the process down:
//
//	--- FAIL: TestRewriteMediaPath_NothingToRewrite/a_sourceless_block_form (0.00s)
//	    mediapath_test.go:133: RewriteMediaPath = ("assets/new.png", true), want ("", false)
//	    mediapath_test.go:136: the mapping ran for a node that names no path
//	    (the same two in .../a_block_form_addressed_by_id_only,
//	    .../a_caption_form_with_no_path and .../a_chip_with_no_path)
//	--- FAIL: TestRewriteMediaPath_NothingToRewrite/a_media_node_with_a_nil_attrs_map (0.00s)
//	panic: assignment to entry in nil map [recovered, repanicked]
func TestRewriteMediaPath_NothingToRewrite(t *testing.T) {
	for _, tc := range []struct {
		node ast.Node
		name string
	}{
		{NewMedia(map[string]string{}, nil), "a sourceless block form"},
		{NewMedia(map[string]string{"id": "abc"}, nil), "a block form addressed by id only"},
		{&MediaCaption{Attrs: map[string]string{}}, "a caption form with no path"},
		{&MediaInline{Attrs: map[string]string{}}, "a chip with no path"},
		{&MediaInline{}, "a media node with a nil attrs map"},
		{&ast.Image{URL: "assets/old.png"}, "an image is not a directive"},
		{&Panel{PanelType: "info", Attrs: map[string]string{"path": "assets/old.png"}}, "a panel is not media"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			got, ok := RewriteMediaPath(tc.node, func(string) string {
				called = true
				return "assets/new.png"
			})
			if ok || got != "" {
				t.Errorf("RewriteMediaPath = (%q, %v), want (\"\", false)", got, ok)
			}
			if called {
				t.Error("the mapping ran for a node that names no path")
			}
		})
	}
}
