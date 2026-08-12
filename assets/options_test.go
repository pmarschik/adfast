package assets

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	adfast "github.com/pmarschik/adfast"
)

// TestRenderOptions_MaterializesOnlyTheDocumentsOwnMedia: Resolve is not a
// passive read — it repairs the friendly file for the id it is asked about,
// next to the document being rendered. One index serves every document under
// the root, so rendering must ask about the media the document contains and no
// more; asking for the whole index left a copy of every asset in the
// repository beside whichever document was rendered.
func TestRenderOptions_MaterializesOnlyTheDocumentsOwnMedia(t *testing.T) {
	root := t.TempDir()
	docA := filepath.Join(root, "docs", "a")
	docB := filepath.Join(root, "docs", "b")
	for _, d := range []string{docA, docB} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	viewA, err := NewFSStoreSplit(root, docA)
	if err != nil {
		t.Fatal(err)
	}
	viewB, err := NewFSStoreSplit(root, docB)
	if err != nil {
		t.Fatal(err)
	}
	if _, addErr := viewA.Add("", uuidA, "one.png", tinyPNG(t, 3, 4)); addErr != nil {
		t.Fatal(addErr)
	}
	if _, addErr := viewB.Add("", uuidB, "two.png", tinyPNG(t, 5, 6)); addErr != nil {
		t.Fatal(addErr)
	}

	// A document naming only its own image, rendered back from ADF.
	mdOpts := MarkdownOptions(viewA)
	doc := adfast.ToADF(adfast.FromMarkdown("![one](assets/one.png)\n", mdOpts...), mdOpts...)
	if !hasMedia(doc.Content, uuidA) {
		t.Fatalf("test setup: the document must reference %s", uuidA)
	}
	renderOpts := RenderOptions(viewA)
	md := adfast.ToMarkdown(adfast.FromADF(doc, renderOpts...), renderOpts...)

	if want := "![one](assets/one.png)\n"; md != want {
		t.Errorf("rendered = %q, want %q", md, want)
	}
	entries, err := os.ReadDir(filepath.Join(docA, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "one.png" {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("document A's assets = %v, want only one.png", names)
	}
}

// TestEnsureUploaded: the push-side entry point syncs pending assets
// FIRST and returns markdown options that already resolve the
// just-uploaded ids — encoding never observes an uploadable asset.
func TestEnsureUploaded(t *testing.T) {
	mdDir := t.TempDir()
	writeAsset(t, mdDir, "shot.png", tinyPNG(t, 2, 2))
	s, err := NewFSStore(mdDir)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	up := UploaderFunc(func(_ context.Context, batch []PendingAsset) ([]UploadResult, error) {
		calls++
		results := make([]UploadResult, 0, len(batch))
		for _, p := range batch {
			results = append(results, UploadResult{Path: p.Path, MediaID: uuidA})
		}
		return results, nil
	})

	opts, err := EnsureUploaded(t.Context(), s, up)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("uploader calls = %d, want 1", calls)
	}
	doc := adfast.ToADF(adfast.FromMarkdown("![a](assets/shot.png)\n", opts...), opts...)
	if !hasMedia(doc.Content, uuidA) {
		t.Error("returned options must resolve the just-uploaded id")
	}

	// Nothing pending anymore: no further uploads on the next call.
	if _, err := EnsureUploaded(t.Context(), s, up); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("idle EnsureUploaded must not upload again, calls = %d", calls)
	}
}
