package assets

import (
	"os"
	"path/filepath"
	"testing"

	adfast "github.com/pmarschik/adfast"
)

// A reference-style image keeps its destination on the definition, not on
// the image node, so both of this package's walks have to look there too.
// Neither did when the markdown leg started keeping references: the walks
// matched *ast.Image only, so "![logo]" with "[logo]: assets/one.png"
// silently stopped uploading its file and stopped being re-pathed on a
// layout change — the picture was in the document and the bytes were
// never anywhere.

// TestSyncOnEncode_ReferenceStyleImageUploads: the file a reference-style
// image points at is referenced, so it goes up in the batch and the
// document encodes with its media node — exactly as the inline form does.
func TestSyncOnEncode_ReferenceStyleImageUploads(t *testing.T) {
	mdDir := t.TempDir()
	dir := mustMkdir(t, filepath.Join(mdDir, "assets"))
	for i, name := range []string{"one.png", "scratch.png"} {
		mustDo(t, os.WriteFile(filepath.Join(dir, name), tinyPNG(t, i+1, i+1), 0o600))
	}
	store := mustStore(t, mdDir)

	var calls int
	up := mappedUploader(t, &calls, map[string]string{"assets/one.png": uuidA})

	docs := mustPushAll(t, PushPipeline(t.Context(), store, up), []string{
		"![the logo][logo]\n\n[logo]: assets/one.png\n",
	})
	if calls != 1 {
		t.Errorf("uploader calls = %d, want 1 batch", calls)
	}
	wantMedia(t, docs[0], uuidA)
	// The unreferenced file must still be pending: collecting definitions
	// must not turn the whole worklist into "referenced".
	wantPending(t, store, "assets/scratch.png")
}

// TestCollectLocalImages_OnlyImageDefinitionsCount: a definition a LINK
// resolves to is a document the author points at, not an embed, so it is
// not an asset to upload. This is the line between the two — without it
// every linked PDF would be dragged into the batch.
func TestCollectLocalImages_OnlyImageDefinitionsCount(t *testing.T) {
	tests := []struct {
		name, md string
		want     []string
	}{
		{"inline image", "![logo](./logo.png)\n", []string{"logo.png"}},
		{"shortcut image reference", "![logo]\n\n[logo]: ./logo.png\n", []string{"logo.png"}},
		{"full image reference", "![the logo][logo]\n\n[logo]: ./logo.png\n", []string{"logo.png"}},
		{"link reference is not an embed", "[a link][doc]\n\n[doc]: ./doc.pdf\n", nil},
		{"unused definition is not referenced", "Body.\n\n[logo]: ./logo.png\n", nil},
		{"remote destination is not local", "![logo]\n\n[logo]: https://e.com/l.png\n", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := map[string]bool{}
			collectLocalImages(adfast.FromMarkdown(tc.md), got)
			if len(got) != len(tc.want) {
				t.Fatalf("collected %v, want %v", got, tc.want)
			}
			for _, ref := range tc.want {
				if !got[ref] {
					t.Errorf("collected %v, want it to hold %q", got, ref)
				}
			}
		})
	}
}

// TestRewriteReferences_ReferenceStyleImage: the layout change moves the
// assets folder, and the destination that has to follow it is the
// definition's. The reference itself must not change — it names a label,
// not a path.
func TestRewriteReferences_ReferenceStyleImage(t *testing.T) {
	root := t.TempDir()
	docDir := mustMkdir(t, filepath.Join(root, "docs"))
	local, err := NewFSStore(docDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, addErr := local.Add("", uuidA, "shot.png", tinyPNG(t, 3, 4)); addErr != nil {
		t.Fatal(addErr)
	}
	md := "# Title\n\n![screen][shot]\n\n[shot]: assets/shot.png\n"

	// Same layout: the rewrite is a no-op, definition included.
	same := adfast.ToMarkdown(
		adfast.FromMarkdown(md, adfast.WithPrettierFormat(), RewriteReferences(local, local)),
		adfast.WithPrettierFormat(), RewriteReferences(local, local),
	)
	if same != md {
		t.Errorf("same-layout format changed the document:\n%q", same)
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
	want := "# Title\n\n![screen][shot]\n\n[shot]: ../assets/shot.png\n"
	if rewritten != want {
		t.Errorf("rewritten:\n%q\nwant:\n%q", rewritten, want)
	}
}

// TestRewriteReferences_LeavesALinkDefinitionAlone: the same line drawn
// again on the rewrite side. A definition a link resolves to is not an
// asset path, so a basename match against the store must not capture it.
func TestRewriteReferences_LeavesALinkDefinitionAlone(t *testing.T) {
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
	md := "[a link][shot]\n\n[shot]: assets/shot.png\n"
	got := adfast.ToMarkdown(
		adfast.FromMarkdown(md, adfast.WithPrettierFormat(), RewriteReferences(nil, shared)),
		adfast.WithPrettierFormat(), RewriteReferences(nil, shared),
	)
	if got != md {
		t.Errorf("a link's definition was rewritten:\n%q", got)
	}
}
