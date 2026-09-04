package adfast

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/convert"
)

// The bug this file exists for: `::media[alt]{}` — a media directive
// carrying no media id, no url and no path — encoded to a media node with
// NO SOURCE and said nothing about it.
//
// ADF addresses a picture two ways and that node supplies neither, so it
// resolves to nothing: it will not render, and the payload holds no clue
// why. The author reading their own markdown sees a well-formed directive;
// the reader of the pushed page sees a gap. Silence is what makes it
// dangerous, and adfast's house rule is that a construct the payload
// cannot resolve either round-trips or reports.
//
// It was quiet by an argument that is true as far as it goes — nothing was
// LOST, because the author named nothing to lose — and that is why the
// silence survived. But "nothing was lost" is not "the payload is fine".
// The neighboring shape, a `path=` the store cannot place, has reported
// CodeUnresolvedAsset for the same reason: with no id, nothing on the page
// finds the file. So the report reuses that code rather than growing the
// enumerable inventory a consumer classifies against; what separates the
// two is the message, because THIS one is permanent — no upload resolves a
// directive that named nothing to upload — and the sentence says so.
//
// The node still ships, and that is a PIN, not part of the fix: the
// directive is the author's explicit instruction and carries the alt text
// and the caption, and an empty `::media[alt]{}` is a reasonable line to
// leave while the picture it will name is still being made. Refusing it at
// parse time was the alternative and would have turned that half-written
// line into literal prose on the md→md leg, which today keeps it
// untouched. Both legs are probed below.
//
// Every probe rides a RESOLVABLE directive in the same document, so a
// change that reported everything (or silenced everything) fails here.

// The four sourceless spellings. Each shares its document with a
// directive the store places, which must stay quiet.
func TestSourcelessMediaIsReported(t *testing.T) {
	cases := []struct {
		name  string
		row   string
		label string
	}{
		{"no attributes at all", "::media[the logo]{}", `label "the logo"`},
		// A directive can carry presentation attributes and still name no
		// picture to present.
		{"presentation attributes only", "::media[the logo]{width=10}", `label "the logo"`},
		// External media is addressed by url, so external WITHOUT one is
		// sourceless too — the type alone names nothing.
		{"external with no url", "::media[the logo]{type=external}", `label "the logo"`},
		// Nothing at all to name the line by; the message says that
		// instead of quoting an empty string.
		{"no label either", "::media{}", "carries no label either"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var messages []string
			opts := append(storeWithOnePath(), WithDiagnostics(func(d convert.Diagnostic) {
				if d.Code == convert.CodeUnresolvedAsset {
					messages = append(messages, d.Message)
				}
			}))
			md := c.row + "\n\n::media[shot]{path=assets/shot.png}\n"

			got := adfJSON(t, mdToADF(md, opts...))

			if len(messages) != 1 {
				t.Fatalf("want one %s diagnostic — for %q only — got %v",
					convert.CodeUnresolvedAsset, c.row, messages)
			}
			// The sentence has to say what is wrong, that the picture is
			// gone, that waiting for an upload will not help, and where to
			// look.
			for _, part := range []string{
				"names no source", "will not render", "no upload can resolve it", c.label,
			} {
				if !strings.Contains(messages[0], part) {
					t.Errorf("the diagnostic must say %q, got %q", part, messages[0])
				}
			}
			// The good case, in the same payload: the store places this one,
			// so it is addressable media carrying the id and the size.
			wantGood := `"id":"abc-123"`
			if !strings.Contains(got, wantGood) {
				t.Errorf("the resolvable directive must still resolve\n  want %s\n  got  %s", wantGood, got)
			}
		})
	}
}

// The caption carrier takes the same fate from the same cause, and its
// label lives one level deeper (a leading paragraph, not the children), so
// it is probed separately.
func TestSourcelessMediaIsReportedForTheCaptionForm(t *testing.T) {
	const md = ":::media[the logo]{}\nA **bold** caption\n:::\n\n" +
		":::media[shot]{path=assets/shot.png}\nA **bold** caption\n:::\n"
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
	if !strings.Contains(messages[0], `label "the logo"`) {
		t.Errorf("the caption form's label must locate the line, got %q", messages[0])
	}
	// The caption is why the node ships rather than dropping: it is
	// content the author wrote, and it has nowhere else to go.
	if !strings.Contains(got, `"type":"caption"`) {
		t.Errorf("the sourceless directive's caption must still ship:\n%s", got)
	}
	if !strings.Contains(got, `"id":"abc-123"`) {
		t.Errorf("the resolvable caption form must still resolve its path:\n%s", got)
	}
}

// One report per directive, never two. A sourceless directive and an
// unresolvable path are different shapes with different messages, and the
// two predicates are disjoint by construction (dialect.SourcelessMedia
// requires no path, dialect.UnresolvedMediaPath requires one) — so a
// document holding both gets exactly one sentence for each.
//
// This is the disjointness itself under test, not the else that expresses
// it: dropping that else is measurably a no-op, and it stands only to keep
// a later widening of either predicate from doubling up.
func TestSourcelessAndUnresolvableMediaAreReportedApart(t *testing.T) {
	const md = "::media[nothing]{}\n\n::media[gone]{path=assets/gone.png}\n\n" +
		"::media[shot]{path=assets/shot.png}\n"
	var messages []string
	mdToADF(md, append(storeWithOnePath(), WithDiagnostics(func(d convert.Diagnostic) {
		if d.Code == convert.CodeUnresolvedAsset {
			messages = append(messages, d.Message)
		}
	}))...)

	if len(messages) != 2 {
		t.Fatalf("want exactly two diagnostics — one per unaddressable directive — got %v", messages)
	}
	sourceless := strings.Contains(messages[0], "names no source")
	unresolved := strings.Contains(messages[1], "assets/gone.png")
	if !sourceless || !unresolved {
		t.Errorf("the two shapes must report their own sentence, got %v", messages)
	}
	if strings.Contains(messages[1], "names no source") {
		t.Errorf("an unresolvable path is not sourceless — it named a path: %q", messages[1])
	}
}

// The PIN half: the ADF encode keeps the node and the md→md leg keeps the
// directive. Only the report is new, so a fix that started dropping either
// one fails here.
func TestSourcelessMediaStillShipsAndStillRoundTrips(t *testing.T) {
	got := adfJSON(t, mdToADF("::media[the logo]{}\n", storeWithOnePath()...))
	const wantNode = `{"type":"media","attrs":{"alt":"the logo","collection":"","type":"file"}}`
	if !strings.Contains(got, wantNode) {
		t.Errorf("the reported node must still ship\n  want %s\n  got  %s", wantNode, got)
	}

	// The md→md leg is total and reports nothing: it keeps the directive
	// exactly as written (bar the empty attribute braces, which say
	// nothing), so a formatter run cannot lose a line the author is still
	// working on.
	var messages []string
	out := fmtMD("::media[the logo]{}\n", append(storeWithOnePath(),
		WithDiagnostics(func(d convert.Diagnostic) {
			if d.Code == convert.CodeUnresolvedAsset {
				messages = append(messages, d.Message)
			}
		}))...)
	if out != "::media[the logo]\n" {
		t.Errorf("the formatter must keep the directive, got %q", out)
	}
	if len(messages) != 0 {
		t.Errorf("the md→md leg loses nothing, so it reports nothing, got %v", messages)
	}
}
