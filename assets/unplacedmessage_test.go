package assets

import (
	"path/filepath"
	"strings"
	"testing"

	adfast "github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/convert"
)

// TestUnplacedImageMessageDoesNotClaimTheStoreLacksTheFile: the sentence
// an encode emits for a picture it cannot place used to read "has no
// media id (not in the asset store)". The parenthetical is a fact the
// encode does not have. The branch is reached on the asset-id lookup
// returning not-ok, and that answers one question only — whether an id
// is known — so a file the author has already dropped into assets/ and
// not yet uploaded reaches it too, and the sentence then sent that
// author to the folder to look for a file that was already there.
//
// The comparison path is where the two come apart, which is why the test
// drives ReportMarkdownOptions rather than the push pipeline: a push
// offers an unuploaded file to the uploader and it comes back with an
// id, so the message never fires. A report resolves through the blobs
// and uploads nothing, so a present-but-unrecorded file misses the
// lookup with the file sitting right there.
//
// Both rows below are the SAME encode and the SAME sentence. The
// difference is only whether the file exists, and that is the point: the
// encode cannot tell them apart, so it must not claim to.
func TestUnplacedImageMessageDoesNotClaimTheStoreLacksTheFile(t *testing.T) {
	for name, present := range map[string]bool{
		// The case the old wording was wrong about.
		"the file is there and simply unuploaded": true,
		// The case it was right about — same sentence, and it stays true.
		"the store has nothing for it": false,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			docDir := mustMkdir(t, filepath.Join(root, "docs", "a"))
			store := mustSplitStore(t, root, docDir)
			if present {
				writeAsset(t, docDir, "shot.png", tinyPNG(t, 3, 4))
			}

			var diags []string
			opts := append(ReportMarkdownOptions(ForScope(store, "PAGE-1")),
				adfast.WithDiagnostics(func(d convert.Diagnostic) {
					diags = append(diags, d.Message)
				}))
			adfast.ToADF(adfast.FromMarkdown("![one]("+cleanedRef+")\n", opts...), opts...)

			if len(diags) != 1 {
				t.Fatalf("want the one unplaceable-image diagnostic, got %v", diags)
			}
			if strings.Contains(diags[0], "not in the asset store") {
				t.Errorf("the message claims the store lacks the file, which the encode "+
					"cannot know (file present: %t): %q", present, diags[0])
			}
			// It must still name the gap and the action, or it has been
			// made vague rather than accurate.
			for _, part := range []string{cleanedRef, "has no media id", "uploaded"} {
				if !strings.Contains(diags[0], part) {
					t.Errorf("the diagnostic must say %q, got %q", part, diags[0])
				}
			}
			if present {
				// A report reads without repairing, so the file the author
				// put there is still theirs and untouched.
				wantExists(t, filepath.Join(docDir, "assets", "shot.png"))
			}
		})
	}
}
