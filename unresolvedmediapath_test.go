package adfast

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/convert"
)

// The bug this file exists for: a ::media directive addressed by path —
// the form a pulled document uses for a downloaded attachment — went
// through the ADF encode in SILENCE when nothing could resolve the path.
//
// `path` is not an attribute ADF can hold: ADF names an attachment by
// media id, so the path is a lookup key spent on encode. With no id
// behind it the payload keeps a media node with an EMPTY id — an
// attachment nothing on the page can find — and the path that said where
// the file is has nowhere to go. The markdown-image spelling of the same
// fact had reported unresolved-asset since it was written; the directive
// spelling reported nothing.
//
// Both fates ride in one document in every probe below, so a fix that
// silenced the resolvable half would fail here too.

// storeWithOnePath is the whole asset store these probes run under: one
// path resolves, everything else does not.
func storeWithOnePath() []Option {
	return []Option{
		WithAssetIDResolver(func(ref string) (string, bool) {
			return "abc-123", ref == "assets/shot.png"
		}),
		WithImageDimsResolver(func(ref string) (int, int, bool) {
			return 10, 20, ref == "assets/shot.png"
		}),
	}
}

// Two mutation proofs. With the UnresolvedMediaPath block removed from
// VisitExtension, the encode is byte-identical and only the report goes
// missing — every probe in this file fails, each on its own count:
//
//	--- FAIL: TestUnresolvedMediaPathIsReported
//	    want one unresolved-asset diagnostic — for the unresolvable
//	    directive only — got []
//	--- FAIL: TestResolvableMediaStaysQuiet/a_resolvable_path
//	    want the one diagnostic for assets/gone.png only, got []
//
// And with the predicate's id guard dropped — reporting a path the store
// cannot place even when an explicit id is spelled beside it — the
// false alarm is named:
//
//	--- FAIL: TestResolvableMediaStaysQuiet/an_id_beside_an_unresolvable_path
//	    want the one diagnostic for assets/gone.png only, got [media
//	    directive path assets/gone-too.png has no media id …, media
//	    directive path assets/gone.png has no media id …]
func TestUnresolvedMediaPathIsReported(t *testing.T) {
	const md = "::media[shot]{path=assets/shot.png}\n\n::media[gone]{path=assets/gone.png}\n"
	var messages []string
	opts := append(storeWithOnePath(), WithDiagnostics(func(d convert.Diagnostic) {
		if d.Code == convert.CodeUnresolvedAsset {
			messages = append(messages, d.Message)
		}
	}))

	got := adfJSON(t, mdToADF(md, opts...))

	if len(messages) != 1 {
		t.Fatalf("want one %s diagnostic — for the unresolvable directive only — got %v",
			convert.CodeUnresolvedAsset, messages)
	}
	// The sentence a consumer shows the author names the path it could not
	// place and what the payload will be missing.
	for _, part := range []string{"assets/gone.png", "has no media id", "cannot be addressed"} {
		if !strings.Contains(messages[0], part) {
			t.Errorf("the diagnostic must say %q, got %q", part, messages[0])
		}
	}
	// The good case, in the same payload: the store knows this one, so it
	// is addressable media carrying the id and the measured size.
	wantGood := `{"type":"media","attrs":{"alt":"shot","collection":"","height":20,` +
		`"id":"abc-123","type":"file","width":10}}`
	if !strings.Contains(got, wantGood) {
		t.Errorf("the resolvable directive must still resolve\n  want %s\n  got  %s", wantGood, got)
	}
	// And the reported half is a PIN, not a fix: the node still ships,
	// because the directive is an explicit instruction from the author and
	// dropping it would lose the alt text and the caption with it. What
	// changed is that the encode now says so.
	wantReported := `{"type":"media","attrs":{"alt":"gone","collection":"","type":"file"}}`
	if !strings.Contains(got, wantReported) {
		t.Errorf("the unresolvable directive's node changed\n  want %s\n  got  %s", wantReported, got)
	}
}

// The caption carrier is the same directive family and takes the same
// loss, so it must report too — it was the site that had never resolved
// a path at all.
func TestUnresolvedMediaPathIsReportedForTheCaptionForm(t *testing.T) {
	const md = ":::media[shot]{path=assets/shot.png}\nA **bold** caption\n:::\n\n" +
		":::media[gone]{path=assets/gone.png}\nA **bold** caption\n:::\n"
	var messages []string
	opts := append(storeWithOnePath(), WithDiagnostics(func(d convert.Diagnostic) {
		if d.Code == convert.CodeUnresolvedAsset {
			messages = append(messages, d.Message)
		}
	}))

	got := adfJSON(t, mdToADF(md, opts...))

	if len(messages) != 1 {
		t.Fatalf("want one %s diagnostic, got %v", convert.CodeUnresolvedAsset, messages)
	}
	if !strings.Contains(messages[0], "assets/gone.png") {
		t.Errorf("the diagnostic must name the unresolvable path, got %q", messages[0])
	}
	if !strings.Contains(got, `"id":"abc-123"`) {
		t.Errorf("the resolvable caption form must still resolve its path:\n%s", got)
	}
}

// The shapes that must stay QUIET, each in a document with a reported one
// so the probe cannot pass by reporting nothing at all: an explicit id (an
// id the store does not know still addresses the attachment), external
// media (addressed by url, no attachment), and a path the store places.
//
// A directive with no source attribute at all used to be on this list, on
// the ground that nothing was named and so nothing was lost. It is off it
// now and reported instead: nothing being lost is not the same as the
// payload being fine, and that shape ships a media node no product can
// resolve. See sourcelessmedia_test.go.
func TestResolvableMediaStaysQuiet(t *testing.T) {
	cases := []struct {
		name string
		row  string
	}{
		{"an id the store does not know", "::media[x]{id=not-in-the-store}"},
		// The shape the id guard is actually for: both spelled, and the
		// path is the one the store cannot place. Nothing is lost — the id
		// travels and addresses the attachment — so reporting here would
		// be a false alarm about a picture that is on the page.
		{"an id beside an unresolvable path", "::media[x]{id=abc-123 path=assets/gone-too.png}"},
		{"external media", "::media[x]{type=external url=https://x/a.png}"},
		{"a resolvable path", "::media[x]{path=assets/shot.png}"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var messages []string
			opts := append(storeWithOnePath(), WithDiagnostics(func(d convert.Diagnostic) {
				if d.Code == convert.CodeUnresolvedAsset {
					messages = append(messages, d.Message)
				}
			}))
			// The reported row rides along, so a probe that reports nothing
			// because the report broke fails instead of passing.
			md := c.row + "\n\n::media[gone]{path=assets/gone.png}\n"
			mdToADF(md, opts...)
			if len(messages) != 1 {
				t.Fatalf("want the one diagnostic for assets/gone.png only, got %v", messages)
			}
			if !strings.Contains(messages[0], "assets/gone.png") {
				t.Errorf("%q was reported, and it lost nothing: %q", c.row, messages[0])
			}
		})
	}
}

// With no asset store wired up at all, a path-addressed directive is
// just as unaddressable as one the store came back empty on, so it
// reports the same way. A consumer that formats markdown without any
// store never reaches this: the md→md leg keeps the directive and its
// path exactly as written, and only the ADF encode spends the path.
func TestUnresolvedMediaPathIsReportedWithNoStore(t *testing.T) {
	var messages []string
	mdToADF("::media[gone]{path=assets/gone.png}\n",
		WithDiagnostics(func(d convert.Diagnostic) {
			if d.Code == convert.CodeUnresolvedAsset {
				messages = append(messages, d.Message)
			}
		}))
	if len(messages) != 1 {
		t.Fatalf("want one %s diagnostic, got %v", convert.CodeUnresolvedAsset, messages)
	}
}
