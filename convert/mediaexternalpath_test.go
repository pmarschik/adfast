package convert

// One document, five media rows, and the whole rendered output pinned:
// the format leg may not delete an address the author spelled, and it
// may not buy that by refusing to shorten the pictures that lose
// nothing.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/markdown"
)

// externalPathDoc is the document every case below shares. The rows are,
// in order:
//
//   - the two FIX rows: external media that spells a url AND a
//     markdown-relative path, as a leaf and as a caption container. An
//     ast.Image has one destination, so degrading these to ![alt](url)
//     deleted the path outright.
//   - the two rows that ALREADY kept their path, and that a fix must not
//     be paid for with: external media with no url (nothing to degrade
//     to) and a path beside an author-pinned id.
//   - the row that must STILL degrade: external media with a url and no
//     path, where the image form loses nothing. It sits in this document
//     on purpose — a "fix" that saves the path by blocking the
//     projection for all external media fails here, in this test,
//     instead of downstream in someone's diff.
//
// parityGoodImage and parityGoodLink close it out, as in every media
// document in this package.
const externalPathDoc = "::media[alt]{path=assets/shot.png type=external url=https://e.com/x.png}\n" +
	"\n" +
	":::media[cap]{path=assets/cap.png type=external url=https://e.com/c.png}\nA caption\n:::\n" +
	"\n" +
	"::media[nourl]{path=assets/nourl.png type=external}\n" +
	"\n" +
	"::media[pinned]{#0a1b2c3d-0e1f-2a3b-4c5d-6e7f8a9b0c1d path=assets/pinned.png}\n" +
	"\n" +
	"::media[plain]{type=external url=https://e.com/plain.png}\n" +
	"\n" + parityGoodImage + "\n\n" + parityGoodLink + "\n"

// TestFormatLegKeepsAnExternalMediaPath pins the format leg's exact bytes
// for externalPathDoc.
//
// This is a DEFECT PROOF. Before the fix, external media carrying a url
// degraded to an *ast.Image whatever else it spelled, so the first two
// rows came back as `![alt](https://e.com/x.png)` and
// `![cap](https://e.com/c.png "A caption")` — the caption survived as
// the image title and the path was gone, with no diagnostic. A consumer
// runs this leg to normalize a pulled document and to normalize both
// sides of a diff, so the loss landed in a working copy without a word.
//
// The last three rows are the preserved behavior the fix had to keep:
// two directives that already held their path (they have no image form
// to degrade to), and one that still becomes a picture (it spells no
// path, so the image's single destination holds everything the directive
// said).
func TestFormatLegKeepsAnExternalMediaPath(t *testing.T) {
	t.Parallel()
	// The three external rows spell layout="center": it is the required
	// mediaSingle attribute, which a non-file media directive completes on
	// both legs now (dialect's mediaSingleFromAttrs carries the reference
	// rows). The file-media "pinned" row below stays terse, because
	// align-start is elided for that type. What this test pins either way
	// is the PATH, and every path still survives.
	const want = "::media[alt]{layout=\"center\" path=\"assets/shot.png\" type=\"external\" " +
		"url=\"https://e.com/x.png\"}\n" +
		"\n" +
		":::media[cap]{layout=\"center\" path=\"assets/cap.png\" type=\"external\" " +
		"url=\"https://e.com/c.png\"}\n" +
		"A caption\n" +
		":::\n" +
		"\n" +
		"::media[nourl]{layout=\"center\" path=\"assets/nourl.png\" type=\"external\"}\n" +
		"\n" +
		"::media[pinned]{#0a1b2c3d-0e1f-2a3b-4c5d-6e7f8a9b0c1d path=\"assets/pinned.png\"}\n" +
		"\n" +
		"![plain](https://e.com/plain.png)\n" +
		"\n" + parityGoodImage + "\n\n" + parityGoodLink + "\n"
	got := parityFormatLeg(externalPathDoc)
	if got != want {
		t.Errorf("the format leg's external-media output changed\n  in:   %q\n  got:  %q\n  want: %q",
			externalPathDoc, got, want)
	}
	for _, path := range []string{"assets/shot.png", "assets/cap.png", "assets/nourl.png", "assets/pinned.png"} {
		if !strings.Contains(got, path) {
			t.Errorf("the format leg deleted an author's path %q from\n  %q", path, got)
		}
	}
	parityCheckGoodRows(t, "format", got)
}

// TestFormatLegExternalMediaPathIsAFixpoint runs the formatter on its own
// output. A projection that keeps a path only until the next pass, or one
// that writes a directive which reads back as an image, is not a format.
//
// This is a PRESERVED-BEHAVIOR PIN in the sense that the pre-fix
// implementation was a fixpoint too — it deleted the path on the first
// pass and had nothing left to delete on the second. It is here because
// the fix adds a blocking rule to the image projection, and every earlier
// blocking rule in this projection had to be checked against exactly this
// property (see TestNoOpResizeDoesNotBlockTheImageForm, where a directive
// that omitted the width it was blocked by re-read as an image and the
// formatter changed its own output).
func TestFormatLegExternalMediaPathIsAFixpoint(t *testing.T) {
	t.Parallel()
	once := parityFormatLeg(externalPathDoc)
	twice := parityFormatLeg(once)
	if once != twice {
		t.Errorf("the formatter is not a fixpoint on its own external-media output"+
			"\n  once:  %q\n  twice: %q", once, twice)
	}
}

// TestExternalMediaPathSurvivesTheEncodeLeg pins the same rule on
// Normalize, the encode-side entry point, because the guard lives in the
// projection both entry points share and only one of them has the
// totality contract.
//
// The encode leg is ALLOWED to drop what ADF has no node for, so the
// question here is not whether it may — it is whether it gains anything.
// It does not: ToADF projects the kept directive to the same media node
// it would have projected the image to, so keeping it costs the encode
// nothing and buys back the documented invariance of ToADF under the
// pass (see Normalize's package comment). Measured before the fix, the
// image form pinned `layout: "center"` onto the mediaSingle wrapper that
// the directive form leaves out, so ToADF(Normalize(n)) and ToADF(n)
// disagreed for this shape; now they agree.
//
// For THIS shape, and now for every shape: the wrapper attribute used to
// split the two encodes wherever an external media directive omitted a
// layout and reached the image form anyway. The directive form completes
// it for every media type now, so that wider gap is closed too — see
// mediawrapperlayout_test.go for the reference evidence that settled
// which of the two forms was wrong.
//
// Closing it cost this test its degrade DETECTOR, which is worth saying
// plainly rather than quietly re-pinning. The check used to be "the
// first block carries no layout attribute", which worked only while a
// kept external directive encoded without one. Both forms now encode
// mediaSingle{layout: "center"} with the same media leaf, so for this
// shape the two ADF payloads are byte-identical whether the pass kept
// the directive or degraded it, and no assertion on the ADF can tell
// them apart. The survival is still observable one step earlier, on the
// AST the pass returns: a kept directive renders with its path=, a
// degraded one renders as ![alt](url) and the path is gone. That is the
// property this test is named for, so the check moved there.
//
// This is a DEFECT PROOF: on the implementation before the path fix the
// normalized tree rendered as the plain image and the path was deleted.
func TestExternalMediaPathSurvivesTheEncodeLeg(t *testing.T) {
	t.Parallel()
	const row = "::media[alt]{path=assets/shot.png type=external url=https://e.com/x.png}"
	md := parityDoc(row)
	// Normalize rewrites in place, so each leg gets its own parse.
	rawDoc := ToADF(markdown.Parse([]byte(md)))
	normDoc := ToADF(Normalize(markdown.Parse([]byte(md))))
	raw, normalized := adfJSON(t, rawDoc), adfJSON(t, normDoc)
	if raw != normalized {
		t.Errorf("ToADF is not invariant under Normalize for external media with a path"+
			"\n  in:         %q\n  ToADF:      %s\n  normalized: %s", md, raw, normalized)
	}
	// The path lives on the AST, not in ADF, so the survival is asserted
	// on what the pass returns rather than on what it encodes to.
	kept := markdown.Render(Normalize(markdown.Parse([]byte(md))), markdown.WithPrettierText())
	if !strings.Contains(kept, `path="assets/shot.png"`) {
		t.Errorf("the pass degraded the directive to an image and deleted the author's path"+
			"\n  in:  %q\n  got: %q", md, kept)
	}
}

// adfJSON renders an ADF document to JSON so two encodes can be compared
// as one value, and a mismatch prints as the payload a caller would send.
func adfJSON(t *testing.T, doc adf.Doc) string {
	t.Helper()
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal the ADF document: %v", err)
	}
	return string(b)
}

// adfNodeJSON is adfJSON for a single node, so an assertion can name the
// block it means instead of the whole document.
func adfNodeJSON(t *testing.T, n adf.Node) string {
	t.Helper()
	b, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("marshal the ADF node: %v", err)
	}
	return string(b)
}
