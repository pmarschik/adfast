package assets

import (
	"os"
	"path/filepath"
	"testing"

	adfast "github.com/pmarschik/adfast"
)

// A picture has FOUR spellings — ![alt](path), ::media, :::media and the
// inline :media chip — and the layout rewrite matched *ast.Image alone,
// the same blind spot the upload scan had. So a store layout change
// re-pathed the image form and left the three directive spellings naming
// the file's old home: the encode resolved that path to nothing and
// shipped a media node with an EMPTY id, and the author was told the path
// could not be resolved with no hint that a layout change is what broke
// it. Two spellings of one picture in one document diverged silently.

// TestRewriteReferences_MediaSpellings: the assets folder moves out of the
// document's directory, and every spelling of the picture has to follow
// it. The image form is the GOOD case, in the same document, so a fix that
// re-paths the directives by breaking the plain form fails here.
//
// The last two rows hold the boundaries. A path beside a PINNED id is
// re-pathed even though nothing looks it up (see
// dialect.RewriteMediaPath: a pin does not move a file, and the format
// keeps such a path on purpose, so leaving it would keep the author's
// bytes and rot their meaning) while the pin itself is untouched. A path
// the new store knows NOTHING about stays exactly as written, which is
// what separates a real store lookup from prepending "../".
//
// Restore the *ast.Image-only walk and the directives go red, the image
// form and the unknown path staying green:
//
//	--- FAIL: TestRewriteReferences_MediaSpellings (0.00s)
//	    rewrite_test.go:104: rewritten:
//	        "# Title\n\n![the plain form](../assets/shot.png)\n\n::media[the block form]{path=\"assets/shot.png\"}\n\n:::media[the caption form]{path=\"assets/shot.png\"}\nA caption.\n:::\n\nA chip :media[the chip]{path=\"assets/shot.png\"} in a line.\n\n::media[a pinned id]{#0a1b2c3d-1111-2222-3333-444455556666 path=\"assets/shot.png\"}\n\n::media[nothing knows this one]{path=\"assets/gone.png\"}\n"
//	        want:
//	        "# Title\n\n![the plain form](../assets/shot.png)\n\n::media[the block form]{path=\"../assets/shot.png\"}\n\n:::media[the caption form]{path=\"../assets/shot.png\"}\nA caption.\n:::\n\nA chip :media[the chip]{path=\"../assets/shot.png\"} in a line.\n\n::media[a pinned id]{#0a1b2c3d-1111-2222-3333-444455556666 path=\"../assets/shot.png\"}\n\n::media[nothing knows this one]{path=\"assets/gone.png\"}\n"
//
// Keep the widened walk but reuse the upload scan's
// dialect.MediaLookupPath for it, so a path beside a pin is skipped, and
// the pinned row alone goes red — everything else in the document,
// including the plain form and the unknown path, stays green:
//
//	--- FAIL: TestRewriteReferences_MediaSpellings (0.00s)
//	    rewrite_test.go:104: rewritten:
//	        "…::media[a pinned id]{#0a1b2c3d-1111-2222-3333-444455556666 path=\"assets/shot.png\"}…"
//	        want:
//	        "…::media[a pinned id]{#0a1b2c3d-1111-2222-3333-444455556666 path=\"../assets/shot.png\"}…"
func TestRewriteReferences_MediaSpellings(t *testing.T) {
	root := t.TempDir()
	docDir := mustMkdir(t, filepath.Join(root, "docs"))
	local, err := NewFSStore(docDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, addErr := local.Add("", uuidA, "shot.png", tinyPNG(t, 3, 4)); addErr != nil {
		t.Fatal(addErr)
	}
	md := "# Title\n\n" +
		"![the plain form](assets/shot.png)\n\n" +
		"::media[the block form]{path=assets/shot.png}\n\n" +
		":::media[the caption form]{path=assets/shot.png}\nA caption.\n:::\n\n" +
		"A chip :media[the chip]{path=assets/shot.png} in a line.\n\n" +
		"::media[a pinned id]{#" + uuidB + " path=assets/shot.png}\n\n" +
		"::media[nothing knows this one]{path=assets/gone.png}\n"

	// The same layout re-paths nothing: the rewrite may only differ from
	// the plain format in the paths it had a reason to move.
	canonical := adfast.ToMarkdown(
		adfast.FromMarkdown(md, adfast.WithPrettierFormat()),
		adfast.WithPrettierFormat(),
	)
	same := adfast.ToMarkdown(
		adfast.FromMarkdown(md, adfast.WithPrettierFormat(), RewriteReferences(local, local)),
		adfast.WithPrettierFormat(), RewriteReferences(local, local),
	)
	if same != canonical {
		t.Errorf("same-layout rewrite changed the document:\n%q\nwant:\n%q", same, canonical)
	}

	// The assets folder physically moves to the project root.
	if mvErr := os.Rename(filepath.Join(docDir, "assets"), filepath.Join(root, "assets")); mvErr != nil {
		t.Fatal(mvErr)
	}
	shared, err := NewFSStoreAt(root, docDir)
	if err != nil {
		t.Fatal(err)
	}
	rewritten := adfast.ToMarkdown(
		adfast.FromMarkdown(md, adfast.WithPrettierFormat(), RewriteReferences(nil, shared)),
		adfast.WithPrettierFormat(), RewriteReferences(nil, shared),
	)
	want := "# Title\n\n" +
		"![the plain form](../assets/shot.png)\n\n" +
		"::media[the block form]{path=\"../assets/shot.png\"}\n\n" +
		":::media[the caption form]{path=\"../assets/shot.png\"}\nA caption.\n:::\n\n" +
		"A chip :media[the chip]{path=\"../assets/shot.png\"} in a line.\n\n" +
		"::media[a pinned id]{#" + uuidB + " path=\"../assets/shot.png\"}\n\n" +
		"::media[nothing knows this one]{path=\"assets/gone.png\"}\n"
	if rewritten != want {
		t.Errorf("rewritten:\n%q\nwant:\n%q", rewritten, want)
	}
}

// TestRewriteReferences_ExternalMediaPath: `type=external` with no url of
// its own is the one media form that keeps a path the ADF encode never
// spends and that the formatter does not degrade to an image. It is
// re-pathed for the same reason the pinned id is — external addressing
// does not move a file, and the day the author drops the type the path is
// the only address left.
//
// The image form beside it is the GOOD case again. Either mutation of
// the walk — restoring the *ast.Image-only match, or reusing the upload
// scan's dialect.MediaLookupPath — leaves the image alone and turns the
// directive red:
//
//	--- FAIL: TestRewriteReferences_ExternalMediaPath (0.00s)
//	    rewrite_test.go:150: rewritten:
//	        "![local](../assets/shot.png)\n\n::media[external]{path=\"assets/shot.png\" type=\"external\"}\n"
//	        want:
//	        "![local](../assets/shot.png)\n\n::media[external]{path=\"../assets/shot.png\" type=\"external\"}\n"
func TestRewriteReferences_ExternalMediaPath(t *testing.T) {
	root := t.TempDir()
	docDir := mustMkdir(t, filepath.Join(root, "docs"))
	local, err := NewFSStore(docDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, addErr := local.Add("", uuidA, "shot.png", tinyPNG(t, 3, 4)); addErr != nil {
		t.Fatal(addErr)
	}
	if mvErr := os.Rename(filepath.Join(docDir, "assets"), filepath.Join(root, "assets")); mvErr != nil {
		t.Fatal(mvErr)
	}
	shared, err := NewFSStoreAt(root, docDir)
	if err != nil {
		t.Fatal(err)
	}
	md := "![local](assets/shot.png)\n\n::media[external]{path=assets/shot.png type=external}\n"
	got := adfast.ToMarkdown(
		adfast.FromMarkdown(md, adfast.WithPrettierFormat(), RewriteReferences(nil, shared)),
		adfast.WithPrettierFormat(), RewriteReferences(nil, shared),
	)
	want := "![local](../assets/shot.png)\n\n" +
		"::media[external]{path=\"../assets/shot.png\" type=\"external\"}\n"
	if got != want {
		t.Errorf("rewritten:\n%q\nwant:\n%q", got, want)
	}
}

// TestRewriteReferences_RemoteMediaURLIsNotAPath: external media that
// names its url is degraded to an image by the formatter before the
// rewrite transform ever sees it, and the rewrite must leave a remote
// destination alone. A PIN: the walk's widening must not start treating a
// url as a path.
func TestRewriteReferences_RemoteMediaURLIsNotAPath(t *testing.T) {
	root := t.TempDir()
	docDir := mustMkdir(t, filepath.Join(root, "docs"))
	local, err := NewFSStore(docDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, addErr := local.Add("", uuidA, "shot.png", tinyPNG(t, 3, 4)); addErr != nil {
		t.Fatal(addErr)
	}
	if mvErr := os.Rename(filepath.Join(docDir, "assets"), filepath.Join(root, "assets")); mvErr != nil {
		t.Fatal(mvErr)
	}
	shared, err := NewFSStoreAt(root, docDir)
	if err != nil {
		t.Fatal(err)
	}
	md := "::media[remote]{type=external url=https://example.com/shot.png}\n"
	got := adfast.ToMarkdown(
		adfast.FromMarkdown(md, adfast.WithPrettierFormat(), RewriteReferences(nil, shared)),
		adfast.WithPrettierFormat(), RewriteReferences(nil, shared),
	)
	if want := "![remote](https://example.com/shot.png)\n"; got != want {
		t.Errorf("rewritten:\n%q\nwant:\n%q", got, want)
	}
}
