package adfast

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/convert"
)

// PRESERVED-BEHAVIOR PINS, not fix proofs. Nothing changed in the media
// projection here; what changed is what WithPreserveLocalImages' doc
// comment claims about it. The comment used to say the option keeps the
// reference "instead of dropping it" and that the final push encode
// wants "an unresolved image to drop", and it said the option is read
// by every leg of the media projection "and they agree" — three
// statements the code does not support:
//
//   - off, the picture drops but the LABEL survives, as a link to the
//     path, so nothing is deleted outright;
//   - the option only reaches an image on its own line, because that is
//     the only media form with an external variant;
//   - an inline image is therefore byte-identical either way.
//
// These tests hold the code to the corrected wording so the comment
// cannot drift back.

// pliADF encodes md and returns the ADF wire JSON plus the diagnostic
// codes, with the option on or off.
func pliADF(t *testing.T, md string, preserve bool) (wire string, codes []string) {
	t.Helper()
	opts := []Option{WithDiagnostics(func(d convert.Diagnostic) {
		codes = append(codes, d.Code)
	})}
	if preserve {
		opts = append(opts, WithPreserveLocalImages())
	}
	b, err := json.Marshal(mdToADF(md, opts...))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b), codes
}

func TestPreserveLocalImages_OffKeepsTheLabelAsALinkToThePath(t *testing.T) {
	const src = "![sketch](assets/x.png)\n"

	got, codes := pliADF(t, src, false)

	// The claim under test is that the label SURVIVES. Assert it is
	// there rather than only that the media node is absent, which any
	// empty document would satisfy.
	if !strings.Contains(got, `"text":"sketch"`) {
		t.Errorf("the alt text did not survive the degradation:\n%s", got)
	}
	if !strings.Contains(got, `"href":"assets/x.png"`) {
		t.Errorf("the path did not survive as a link destination:\n%s", got)
	}
	if strings.Contains(got, `"media"`) {
		t.Errorf("off, the local image must not reach a media node:\n%s", got)
	}
	if len(codes) != 1 || codes[0] != convert.CodeUnresolvedAsset {
		t.Errorf("diagnostics = %v, want exactly [%s]", codes, convert.CodeUnresolvedAsset)
	}
}

func TestPreserveLocalImages_OnPromotesAnOwnLineImageToExternalMedia(t *testing.T) {
	const src = "![sketch](assets/x.png)\n"

	got, codes := pliADF(t, src, true)

	if !strings.Contains(got, `"type":"external"`) || !strings.Contains(got, `"url":"assets/x.png"`) {
		t.Errorf("the option did not promote the image to external media:\n%s", got)
	}
	if len(codes) != 0 {
		t.Errorf("diagnostics = %v, want none: the path traveled, so nothing was lost", codes)
	}
}

func TestPreserveLocalImages_LeavesAnInlineImageUntouched(t *testing.T) {
	const src = "see ![sketch](assets/x.png) here\n"

	// Precondition: the option must actually be doing something in this
	// build, or "inline is unchanged" would be vacuously true.
	blockOff, _ := pliADF(t, "![sketch](assets/x.png)\n", false)
	blockOn, _ := pliADF(t, "![sketch](assets/x.png)\n", true)
	if blockOff == blockOn {
		t.Fatal("the option changed nothing even for an own-line image; " +
			"the inline comparison below would prove nothing")
	}

	off, offCodes := pliADF(t, src, false)
	on, onCodes := pliADF(t, src, true)

	if off != on {
		t.Errorf("the option must not reach an inline image, but the ADF differs\noff: %s\non:  %s", off, on)
	}
	// mediaInline has no external variant, so the path cannot travel and
	// the diagnostic fires in BOTH runs.
	for _, c := range [][]string{offCodes, onCodes} {
		if len(c) != 1 || c[0] != convert.CodeUnresolvedAsset {
			t.Errorf("diagnostics = %v, want exactly [%s] with and without the option",
				c, convert.CodeUnresolvedAsset)
		}
	}
	if strings.Contains(on, `"mediaInline"`) {
		t.Errorf("an unplaceable inline image must not become mediaInline:\n%s", on)
	}
}
