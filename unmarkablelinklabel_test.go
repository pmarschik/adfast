package adfast

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/convert"
)

// The bug this file exists for: wrapping an inline leaf in a link dropped the
// destination out of the payload and said NOTHING. A `:mention`, `:status`,
// `:emoji`, `:date`, `:placeholder`, `:extension` or `:media` inside a link
// encodes to a bare ADF leaf, and every one of those encoders leaves the mark
// slot empty — deliberately, because no Atlassian payload has been observed to
// mark one of these kinds. The node is real, so the label stays on the page and
// the author sees nothing wrong; the URL is simply gone. The encode is the last
// place that still knows the href existed, so this is where it must be said.
//
// Attaching the mark instead is the fix these tests deliberately do NOT pin: it
// would emit a shape the product has never been seen to produce, and would move
// the loss to a Jira write that adfast cannot observe.

// droppedLinks collects the destinations reported as dropped for one document,
// so a test can pin the count and the text.
func droppedLinks(t *testing.T, md string) (dropped []string, encoded string) {
	t.Helper()
	doc := mdToADF(md, WithDiagnostics(func(d convert.Diagnostic) {
		if d.Code == convert.CodeLinkDestinationDropped {
			dropped = append(dropped, d.Message)
		}
	}))
	return dropped, adfJSON(t, doc)
}

func TestLinkedInlineLeafReportsTheDroppedDestination(t *testing.T) {
	cases := []struct {
		name  string
		label string
		kind  string
	}{
		{"mention", ":mention[Jane Doe]{#712020:aa11}", "mention"},
		{"status", ":status[Done]{color=green}", "status"},
		{"emoji", `:emoji{#abc shortName=":team_logo:"}`, "emoji"},
		{"date", ":date[2024-01-02]{timestamp=1704153600000}", "date"},
		{"placeholder", ":placeholder[type here]", "placeholder"},
		{"inlineExtension", ":extension{type=com.example key=widget}", "inlineExtension"},
		{"mediaInline", ":media[the file]{id=abc-123}", "mediaInline"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The GOOD case shares the document: a plain label carries its
			// destination as a mark and must stay silent, so a diagnostic that
			// fires for every link at all fails here too.
			const good = "[a plain label](https://example.test/control)\n"
			dropped, got := droppedLinks(t,
				"["+tc.label+"](https://example.test/lost)\n\n"+good)

			if len(dropped) != 1 {
				t.Fatalf("want exactly one %s diagnostic for the linked %s (and none for the plain control), got %v",
					convert.CodeLinkDestinationDropped, tc.name, dropped)
			}
			for _, want := range []string{"https://example.test/lost", tc.kind} {
				if !strings.Contains(dropped[0], want) {
					t.Errorf("the message must name %q so the author can find the line\n  got %s", want, dropped[0])
				}
			}
			if strings.Contains(dropped[0], "https://example.test/control") {
				t.Errorf("the plain control must not be reported:\n  got %s", dropped[0])
			}
			// The href really is gone — that is what makes the diagnostic the
			// only trace, and pinning it here keeps the test honest if a later
			// change decides to attach the mark after all.
			if strings.Contains(got, "https://example.test/lost") {
				t.Errorf("the destination is expected to be absent from the payload:\n%s", got)
			}
			wantGood := `{"type":"text","marks":[{"attrs":{"href":"https://example.test/control"},"type":"link"}],` +
				`"text":"a plain label"}`
			if !strings.Contains(got, wantGood) {
				t.Errorf("the plain control must keep its link mark\n  want %s\n  got  %s", wantGood, got)
			}
		})
	}
}

// The href leaves the document only when NOTHING in the label carries it. A
// leaf with linked text beside it keeps the destination clickable, so there is
// no loss to report — the guard asks about the whole label, not about each
// node in it.
func TestLinkedInlineLeafBesideTextStaysQuiet(t *testing.T) {
	dropped, got := droppedLinks(t,
		"[see :mention[Jane]{#712020:aa11} today](https://example.test/kept)\n")

	if len(dropped) != 0 {
		t.Errorf("the destination is still on the surrounding text, want no %s diagnostic, got %v",
			convert.CodeLinkDestinationDropped, dropped)
	}
	if !strings.Contains(got, `"href":"https://example.test/kept"`) {
		t.Errorf("the destination must survive on the text nodes:\n%s", got)
	}
}

// A style wrapper is not a leaf: it flattens into text that DOES carry the
// mark. Pinning it keeps the widened guard from being read as "any directive
// in a link is a loss".
func TestLinkedStyleDirectiveStaysQuiet(t *testing.T) {
	for _, md := range []string{
		"[:u[underlined]](https://example.test/u)\n",
		"[:color[red]{color=red}](https://example.test/c)\n",
		"[:annotation[noted]{#a1}](https://example.test/a)\n",
		"[**bold**](https://example.test/b)\n",
	} {
		dropped, got := droppedLinks(t, md)
		if len(dropped) != 0 {
			t.Errorf("%s: want no %s diagnostic, got %v", md, convert.CodeLinkDestinationDropped, dropped)
		}
		if !strings.Contains(got, `"type":"link"`) {
			t.Errorf("%s: the destination must survive as a mark:\n%s", md, got)
		}
	}
}

// PRESERVED BEHAVIOR (a pin, not a proof): the empty-label half of the code —
// "[![]()](href)", an image with neither alt text nor a destination to name it
// after — reported before the guard was widened and must keep reporting, with
// its own wording. Widening the question from "produced no nodes" to "produced
// no node that can carry the href" must not lose the narrower case it grew out
// of.
func TestEmptiedLinkLabelStillReportsWithItsOwnWording(t *testing.T) {
	dropped, _ := droppedLinks(t, "[![]()](https://example.test/empty)\n")

	if len(dropped) != 1 {
		t.Fatalf("want exactly one %s diagnostic, got %v", convert.CodeLinkDestinationDropped, dropped)
	}
	if !strings.Contains(dropped[0], "its whole label converted to nothing") {
		t.Errorf("the empty-label case keeps its own wording, so a consumer can tell the two apart\n  got %s",
			dropped[0])
	}
}

// PRESERVED BEHAVIOR (a pin, not a proof): a link with no destination to lose
// stays quiet, and a linked image the store CAN place keeps its destination on
// the media node rather than losing it.
//
// The media half does NOT exercise carriesLink's subtree walk, despite the
// mediaSingle wrapper it produces: a linked image alone in a paragraph reaches
// ADF through the block path and never enters flattenLink at all. Nothing in
// the suite descends that walk today — see the note on carriesLink — and this
// test is the near miss, kept so the boundary is written down instead of
// rediscovered.
func TestLinkWithNothingToLoseStaysQuiet(t *testing.T) {
	if dropped, _ := droppedLinks(t, "[x]()\n"); len(dropped) != 0 {
		t.Errorf("an empty destination is not a loss, got %v", dropped)
	}

	var mediaDropped []string
	mediaDoc := mdToADF("[![alt](assets/shot.png)](https://example.test/media)\n",
		WithAssetIDResolver(func(string) (string, bool) { return "abc-123", true }),
		WithDiagnostics(func(d convert.Diagnostic) {
			if d.Code == convert.CodeLinkDestinationDropped {
				mediaDropped = append(mediaDropped, d.Message)
			}
		}))
	got := adfJSON(t, mediaDoc)
	if len(mediaDropped) != 0 {
		t.Errorf("the mark rides on the nested media node, want no %s diagnostic, got %v",
			convert.CodeLinkDestinationDropped, mediaDropped)
	}
	if !strings.Contains(got, `"marks":[{"attrs":{"href":"https://example.test/media"},"type":"link"}]`) {
		t.Errorf("the linked media must keep its mark:\n%s", got)
	}
}
