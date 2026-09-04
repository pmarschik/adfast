package markdown_test

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/markdown"
)

// imageCase is one source plus the exact text every image must select.
// "whole|alt|dest" per image keeps a failure readable — the three extents
// can go wrong independently, and the destination is the one a caller
// rewrites.
type imageCase struct {
	name string
	src  string
	want []string
}

func imageTexts(src string, imgs []markdown.Image) []string {
	out := make([]string, 0, len(imgs))
	for _, im := range imgs {
		dest := "-"
		if im.Dest != (markdown.Span{}) {
			dest = src[im.Dest.Start:im.Dest.Stop]
		}
		out = append(out, strings.Join([]string{
			src[im.Span.Start:im.Span.Stop],
			src[im.Alt.Start:im.Alt.Stop],
			dest,
		}, "|"))
	}
	return out
}

var imageCases = []imageCase{{
	name: "a plain image",
	src:  "![alt](x.png)\n",
	want: []string{"![alt](x.png)|alt|x.png"},
}, {
	name: "an image with no alt text",
	src:  "![](x.png)\n",
	want: []string{"![](x.png)||x.png"},
}, {
	// The angle brackets stay outside Dest so a rewrite lands between them.
	name: "an angle bracketed destination excludes its brackets",
	src:  "![a](<b c.png>)\n",
	want: []string{"![a](<b c.png>)|a|b c.png"},
}, {
	name: "a title is outside the destination",
	src:  "![a](x.png \"t\")\n",
	want: []string{"![a](x.png \"t\")|a|x.png"},
}, {
	name: "a parenthesized title",
	src:  "![a](x.png (t))\n",
	want: []string{"![a](x.png (t))|a|x.png"},
}, {
	name: "a single quoted title",
	src:  "![a](x.png 't')\n",
	want: []string{"![a](x.png 't')|a|x.png"},
}, {
	// A title may hold the very syntax the scanner looks for.
	name: "a title holding a closing paren",
	src:  "![a](x.png \"a ) b\")\n",
	want: []string{"![a](x.png \"a ) b\")|a|x.png"},
}, {
	name: "balanced parens inside a destination",
	src:  "![a](x(1).png)\n",
	want: []string{"![a](x(1).png)|a|x(1).png"},
}, {
	name: "an empty destination is an insertion point",
	src:  "![a]()\n",
	want: []string{"![a]()|a|"},
}, {
	name: "an empty angle bracketed destination",
	src:  "![a](<>)\n",
	want: []string{"![a](<>)|a|"},
}, {
	// The label's closing bracket is not the first one in the source.
	name: "a nested bracket pair in the alt text",
	src:  "![a[b]c](x.png)\n",
	want: []string{"![a[b]c](x.png)|a[b]c|x.png"},
}, {
	name: "a closing bracket inside inline code in the alt text",
	src:  "![a`]`b](x.png)\n",
	want: []string{"![a`]`b](x.png)|a`]`b|x.png"},
}, {
	name: "an escaped closing bracket in the alt text",
	src:  "![a\\]b](x.png)\n",
	want: []string{"![a\\]b](x.png)|a\\]b|x.png"},
}, {
	// The whole tail lives inside inline code in the label, so only the
	// destination check can tell the two candidate closers apart.
	name: "a whole tail inside inline code in the alt text",
	src:  "![`](x.png)`](y.png)\n",
	want: []string{"![`](x.png)`](y.png)|`](x.png)`|y.png"},
}, {
	name: "raw html in the alt text",
	src:  "![a<b>c](x.png)\n",
	want: []string{"![a<b>c](x.png)|a<b>c|x.png"},
}, {
	name: "an autolink in the alt text",
	src:  "![<http://x/a>](y.png)\n",
	want: []string{"![<http://x/a>](y.png)|<http://x/a>|y.png"},
}, {
	// Alt is source, not text: the emphasis markers stay in it.
	name: "inline markup in the alt text stays verbatim",
	src:  "![*em* x](y.png)\n",
	want: []string{"![*em* x](y.png)|*em* x|y.png"},
}, {
	name: "a multi byte alt text",
	src:  "![мульти](x.png) tail\n",
	want: []string{"![мульти](x.png)|мульти|x.png"},
}, {
	name: "a full reference image has no destination at the image",
	src:  "![ref][id]\n\n[id]: x.png\n",
	want: []string{"![ref][id]|ref|-"},
}, {
	name: "a collapsed reference image",
	src:  "![id][]\n\n[id]: x.png\n",
	want: []string{"![id][]|id|-"},
}, {
	name: "a shortcut reference image",
	src:  "![id]\n\n[id]: x.png\n",
	want: []string{"![id]|id|-"},
}, {
	// No definition, so goldmark makes no image and neither does the view.
	name: "a reference image with no definition is not an image",
	src:  "![missing][id]\n",
	want: nil,
}, {
	name: "an image inside inline code is not an image",
	src:  "`![no](x.png)`\n",
	want: nil,
}, {
	name: "an image inside a fenced block is not an image",
	src:  "```\n![no](x.png)\n```\n",
	want: nil,
}, {
	name: "an image inside an html comment is not an image",
	src:  "<!-- ![no](x.png) -->\n",
	want: nil,
}, {
	name: "an image in a blockquote",
	src:  "> ![q](x.png)\n",
	want: []string{"![q](x.png)|q|x.png"},
}, {
	name: "an image in a list item",
	src:  "- ![l](x.png)\n",
	want: []string{"![l](x.png)|l|x.png"},
}, {
	name: "an image in a heading",
	src:  "# ![h](x.png)\n",
	want: []string{"![h](x.png)|h|x.png"},
}, {
	name: "an image in a table cell",
	src:  "| a |\n| - |\n| ![t](x.png) |\n",
	want: []string{"![t](x.png)|t|x.png"},
}, {
	name: "an image in a directive label",
	src:  "::note[![lab](x.png)]{c=red}\n",
	want: []string{"![lab](x.png)|lab|x.png"},
}, {
	name: "an image inside a container directive",
	src:  ":::note\n![in](x.png)\n:::\n",
	want: []string{"![in](x.png)|in|x.png"},
}, {
	name: "adjacent images",
	src:  "![a](x.png)![b](y.png)\n",
	want: []string{"![a](x.png)|a|x.png", "![b](y.png)|b|y.png"},
}, {
	name: "an image between prose",
	src:  "para ![a](x.png) more\n",
	want: []string{"![a](x.png)|a|x.png"},
}, {
	// Both are reported; the inner span lies inside the outer alt.
	name: "an image nested in another image's alt text",
	src:  "![![in](a.png)](b.png)\n",
	want: []string{"![![in](a.png)](b.png)|![in](a.png)|b.png", "![in](a.png)|in|a.png"},
}, {
	// The nested form writes the SAME destination as the outer one, so the
	// destination check accepts either candidate closer, and a floor above
	// the whole nested image is what picks the outer image's own.
	name: "a nested image sharing the outer image's destination",
	src:  "![![](a.png)](a.png)\n",
	want: []string{"![![](a.png)](a.png)|![](a.png)|a.png", "![](a.png)||a.png"},
}, {
	name: "an opening bracket inside inline code in the alt text",
	src:  "![a`[`b](x.png)\n",
	want: []string{"![a`[`b](x.png)|a`[`b|x.png"},
}, {
	name: "an escaped opening bracket in the alt text",
	src:  "![a\\[b](x.png)\n",
	want: []string{"![a\\[b](x.png)|a\\[b|x.png"},
}, {
	name: "an opening bracket inside raw html in the alt text",
	src:  "![a<i data-x=\"[\">b</i>](x.png)\n",
	want: []string{"![a<i data-x=\"[\">b</i>](x.png)|a<i data-x=\"[\">b</i>|x.png"},
}, {
	name: "a destination on the line after its paren",
	src:  "![a](\nx.png)\n",
	want: []string{"![a](\nx.png)|a|x.png"},
}, {
	// A list's continuation prefix is whitespace, which the tail skips
	// anyway, so a split tail resolves normally here.
	name: "a title on the next line of a list item",
	src:  "- ![a](x.png\n  \"t\")\n",
	want: []string{"![a](x.png\n  \"t\")|a|x.png"},
}, {
	// A blockquote's '>' prefix sits between the two halves of the tail.
	// The span is the written form, prefix and all, exactly as the list
	// item above keeps its continuation indent.
	name: "a title on the next line of a blockquote",
	src:  "> ![a](x.png\n> \"t\")\n",
	want: []string{"![a](x.png\n> \"t\")|a|x.png"},
}, {
	name: "a destination on the next line of a blockquote",
	src:  "> ![a](\n> x.png)\n",
	want: []string{"![a](\n> x.png)|a|x.png"},
}, {
	name: "a destination on the next line of a nested blockquote",
	src:  ">> ![a](\n>> x.png)\n",
	want: []string{"![a](\n>> x.png)|a|x.png"},
}, {
	// The prefix is skipped, not the padding a wrapped destination needs:
	// the angle brackets still bound Dest.
	name: "an angle bracketed destination on the next line of a blockquote",
	src:  "> ![a](\n> <b c.png> \"t\")\n",
	want: []string{"![a](\n> <b c.png> \"t\")|a|b c.png"},
}, {
	// A title that itself spans a quoted line: the closer search crosses
	// the prefix and the paren after it still closes the tail.
	name: "a title spanning two blockquote lines",
	src:  "> ![a](x.png \"t\n> u\")\n",
	want: []string{"![a](x.png \"t\n> u\")|a|x.png"},
}, {
	// The image ends before the prose that follows it on the quoted line.
	name: "a split tail in a blockquote with trailing prose",
	src:  "> ![a](x.png\n> \"t\") tail\n",
	want: []string{"![a](x.png\n> \"t\")|a|x.png"},
}, {
	// A '>' on a continuation line is a prefix only where the parser made
	// one. Here it opens a blockquote instead, so the paragraph ends and
	// there is no image to report.
	name: "a greater than sign on the line after an unclosed tail",
	src:  "![a](\n>x.png)\n",
	want: nil,
}, {
	// A reference label may cross a line, because CommonMark folds
	// whitespace runs when it matches a label, and the span keeps the
	// prefix that sits in the fold — the rule a split tail already follows.
	name: "a reference label crossing a line in a blockquote",
	src:  "> ![a][i\n> d]\n\n[i d]: x.png\n",
	want: []string{"![a][i\n> d]|a|-"},
}, {
	name: "a reference label crossing a line in a list item too",
	src:  "- ![a][i\n  d]\n\n[i d]: x.png\n",
	want: []string{"![a][i\n  d]|a|-"},
}, {
	name: "an image with no trailing newline",
	src:  "![a](x.png)",
	want: []string{"![a](x.png)|a|x.png"},
}, {
	name: "a document with no images",
	src:  "text and a [link](x)\n",
	want: nil,
}, {
	name: "an empty source",
	src:  "",
	want: nil,
}}

func TestImages_Coverage(t *testing.T) {
	t.Parallel()
	for _, c := range imageCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := markdown.Images([]byte(c.src))
			if !eq(imageTexts(c.src, got), c.want) {
				t.Errorf("Images(%q)\n got %q\nwant %q", c.src, imageTexts(c.src, got), c.want)
			}
		})
	}
}

// TestImages_ContractHolds pins document order, the containment of Alt and
// Dest in Span, and the tightness rule itself: an image's span starts at a
// '!' and ends at a ')' or a ']', never at a line boundary it did not
// happen to land on.
func TestImages_ContractHolds(t *testing.T) {
	t.Parallel()
	src := "![a](x.png) and ![b][id] and ![c](<d e.png> \"t\")\n\n> ![q](f.png)\n\n[id]: y.png\n"
	imgs := markdown.Images([]byte(src))
	if len(imgs) != 4 {
		t.Fatalf("got %d images, want 4: %q", len(imgs), imageTexts(src, imgs))
	}
	prev := 0
	for i, im := range imgs {
		if im.Span.Start < prev {
			t.Errorf("image %d = %v is out of document order", i, im.Span)
		}
		prev = im.Span.Start
		if im.Span.Start < 0 || im.Span.Stop > len(src) || im.Span.Len() < 5 {
			t.Errorf("image %d span %v is not a range of a %d byte source", i, im.Span, len(src))
		}
		if src[im.Span.Start] != '!' {
			t.Errorf("image %d starts at %q, want '!'", i, src[im.Span.Start])
		}
		if last := src[im.Span.Stop-1]; last != ')' && last != ']' {
			t.Errorf("image %d ends at %q, want ')' or ']'", i, last)
		}
		if im.Alt.Start < im.Span.Start || im.Alt.Stop > im.Span.Stop {
			t.Errorf("image %d alt %v is not inside its span %v", i, im.Alt, im.Span)
		}
		if im.Dest != (markdown.Span{}) &&
			(im.Dest.Start < im.Span.Start || im.Dest.Stop > im.Span.Stop) {
			t.Errorf("image %d dest %v is not inside its span %v", i, im.Dest, im.Span)
		}
	}
}

// TestImages_TightSpansStayOffTheProse is the reason the span is tight
// rather than widened to whole lines the way the block views are: replacing
// an image must leave the sentence around it alone.
func TestImages_TightSpansStayOffTheProse(t *testing.T) {
	t.Parallel()
	const src = "Before ![a](x.png) after.\n"
	s := markdown.NewSource([]byte(src))
	imgs := s.Images()
	if len(imgs) != 1 {
		t.Fatalf("got %d images, want 1", len(imgs))
	}
	got, err := s.Apply(markdown.Edit{Span: imgs[0].Span, Text: "GONE"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if want := "Before GONE after.\n"; string(got) != want {
		t.Errorf("Apply = %q, want %q", got, want)
	}
}

// TestImages_DoNotOverlapCodeSpans is the composition guarantee that
// matters: the two views come from ONE tree, so an image written inside a
// code block is not an image at all. Two independent scanners would
// disagree here, which is what this surface exists to prevent.
func TestImages_DoNotOverlapCodeSpans(t *testing.T) {
	t.Parallel()
	src := []byte("![real](a.png)\n\n```\n![fake](b.png)\n```\n\n    ![also fake](c.png)\n\n![real too](d.png)\n")
	s := markdown.NewSource(src)
	code := s.CodeSpans()
	imgs := s.Images()
	if len(imgs) != 2 {
		t.Fatalf("got %d images, want 2: %q", len(imgs), imageTexts(string(src), imgs))
	}
	for _, im := range imgs {
		if code.Overlaps(im.Span) {
			t.Errorf("image %q overlaps a code span", src[im.Span.Start:im.Span.Stop])
		}
	}
}

// TestImages_Memoize pins that a second call is the same slice, i.e. that
// the view is computed once per Source rather than per call.
func TestImages_Memoize(t *testing.T) {
	t.Parallel()
	s := markdown.NewSource([]byte("![a](x.png) ![b](y.png)\n"))
	first, second := s.Images(), s.Images()
	if len(first) != 2 || len(second) != len(first) {
		t.Fatalf("Images = %v then %v", first, second)
	}
	if &first[0] != &second[0] {
		t.Error("Images recomputed on the second call")
	}
}

// TestImages_RewriteDestinationsInOneParse is the shape the callers take:
// read the destination verbatim, decide policy over it, hand Edits back.
// Nothing here indexes a byte or re-derives an offset, and the image inside
// the fence is not touched because the parser never called it one.
func TestImages_RewriteDestinationsInOneParse(t *testing.T) {
	t.Parallel()
	const src = "![a](old/one.png)\n\n```\n![x](old/skip.png)\n```\n\n![b](<old/two a.png>) ![c][id]\n\n[id]: old/three.png\n"
	s := markdown.NewSource([]byte(src))

	var edits []markdown.Edit
	for _, im := range s.Images() {
		if im.Dest == (markdown.Span{}) {
			continue // a reference image is rewritten at its definition
		}
		dest := string(s.Text(im.Dest))
		if !strings.HasPrefix(dest, "old/") {
			t.Fatalf("destination = %q, want the raw written path", dest)
		}
		edits = append(edits, markdown.Edit{Span: im.Dest, Text: "new/" + strings.TrimPrefix(dest, "old/")})
	}
	if len(edits) != 2 {
		t.Fatalf("found %d rewritable images, want 2", len(edits))
	}
	got, err := s.Apply(edits...)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	const want = "![a](new/one.png)\n\n```\n![x](old/skip.png)\n```\n\n![b](<new/two a.png>) ![c][id]\n\n[id]: old/three.png\n"
	if string(got) != want {
		t.Errorf("Apply =\n%q\nwant\n%q", got, want)
	}
}

// TestImages_ComposeWithHeadingsInOneParse pins the point of the surface:
// three views and the splice all come off ONE Source, and the edits they
// produce merge without a second parse and without re-derived offsets.
func TestImages_ComposeWithHeadingsInOneParse(t *testing.T) {
	t.Parallel()
	const src = "# Old ![icon](old.png)\n\nbody\n\n```\n# ![no](no.png)\n```\n"
	s := markdown.NewSource([]byte(src))

	var edits []markdown.Edit
	for _, h := range s.Headings() {
		edits = append(edits, markdown.Edit{Span: markdown.Span{
			Start: h.Text.Start, Stop: h.Text.Start,
		}, Text: "New: "})
	}
	for _, im := range s.Images() {
		edits = append(edits, markdown.Edit{Span: im.Dest, Text: "new.png"})
	}
	if len(edits) != 2 {
		t.Fatalf("built %d edits, want 2", len(edits))
	}
	got, err := s.Apply(edits...)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	const want = "# New: Old ![icon](new.png)\n\nbody\n\n```\n# ![no](no.png)\n```\n"
	if string(got) != want {
		t.Errorf("Apply =\n%q\nwant\n%q", got, want)
	}
}

// imageLabelCases cover a DIRECTIVE'S LABEL, the subtree this view used to
// skip. Source.Links and linkLabelCases state the reason and the mechanism
// in full — the text form's label is parsed against a detached copy, so its
// offsets have to be recovered and shifted, while the leaf and container
// forms are parsed in place and have always reported. Both views had the one
// root cause and share the one descent, so both tables carry the forms that
// already worked beside the form that did not.
var imageLabelCases = []imageCase{{
	name: "all three directive forms in one document",
	src:  "Text :sup[![t](t.png)] end\n\n::leaf[![l](l.png)]\n\n:::box[![c](c.png)]\nbody\n:::\n",
	want: []string{
		"![t](t.png)|t|t.png",
		"![l](l.png)|l|l.png",
		"![c](c.png)|c|c.png",
	},
}, {
	name: "a label inside a label composes the shifts",
	src:  "Text :sup[a :sub[![t](t.png)] b] end\n",
	want: []string{"![t](t.png)|t|t.png"},
}, {
	name: "a label image and a prose image in document order",
	src:  "Text :sup[![a](a.png)] and ![b](b.png)\n",
	want: []string{"![a](a.png)|a|a.png", "![b](b.png)|b|b.png"},
}, {
	name: "a label image inside a blockquote",
	src:  "> q :sup[![t](t.png)]\n",
	want: []string{"![t](t.png)|t|t.png"},
}, {
	name: "a label image on a tab-prefixed continuation line",
	src:  "> q\n>\t:sup[![t](t.png)]\n",
	want: []string{"![t](t.png)|t|t.png"},
}, {
	name: "a thumbnail link in a label reports in both views",
	src:  "Text :sup[[![a](i.png)](https://ex.com/t)] end\n",
	want: []string{"![a](i.png)|a|i.png"},
}, {
	name: "an image in inline code inside a label is not an image",
	src:  "Text :sup[`![t](t.png)`] end\n",
	want: nil,
}, {
	// A PIN, not a fix: this passes on both versions of the descent, and it
	// records the one form a label cannot hold. The detached copy is parsed
	// with no reference map of its own, so `![t][id]` in a label is never an
	// image node at all — goldmark builds literal text, and there is nothing
	// for this view to under-report. It is the shape shiftSpan's zero-Dest
	// guard is written against, and it is why that guard is unreachable
	// today rather than merely untested.
	name: "a reference image in a label is not an image at all",
	src:  "Text :sup[![t][id]] end\n\n[id]: t.png\n",
	want: nil,
}, {
	name: "an empty label reports nothing and does not hide prose",
	src:  "Text :sup[] and ![b](b.png)\n",
	want: []string{"![b](b.png)|b|b.png"},
}}

// TestImages_DirectiveLabels drives imageLabelCases.
//
// Measured with the label descent removed from collectImages, which is what
// this view did before:
//
//	--- FAIL: TestImages_DirectiveLabels (0.00s)
//	    --- FAIL: TestImages_DirectiveLabels/a_label_image_on_a_tab-prefixed_continuation_line (0.00s)
//	        Images("> q\n>\t:sup[![t](t.png)]\n") =
//	             []
//	            want
//	             ["![t](t.png)|t|t.png"]
//	    --- FAIL: TestImages_DirectiveLabels/a_label_image_and_a_prose_image_in_document_order (0.00s)
//	        Images("Text :sup[![a](a.png)] and ![b](b.png)\n") =
//	             ["![b](b.png)|b|b.png"]
//	            want
//	             ["![a](a.png)|a|a.png" "![b](b.png)|b|b.png"]
//	    --- FAIL: TestImages_DirectiveLabels/a_thumbnail_link_in_a_label_reports_in_both_views (0.00s)
//	        Images("Text :sup[[![a](i.png)](https://ex.com/t)] end\n") =
//	             []
//	            want
//	             ["![a](i.png)|a|i.png"]
//	    --- FAIL: TestImages_DirectiveLabels/a_label_image_inside_a_blockquote (0.00s)
//	        Images("> q :sup[![t](t.png)]\n") =
//	             []
//	            want
//	             ["![t](t.png)|t|t.png"]
//	    --- FAIL: TestImages_DirectiveLabels/all_three_directive_forms_in_one_document (0.00s)
//	        Images("Text :sup[![t](t.png)] end\n\n::leaf[![l](l.png)]\n\n:::box[![c](c.png)]\nbody\n:::\n") =
//	             ["![l](l.png)|l|l.png" "![c](c.png)|c|c.png"]
//	            want
//	             ["![t](t.png)|t|t.png" "![l](l.png)|l|l.png" "![c](c.png)|c|c.png"]
//	    --- FAIL: TestImages_DirectiveLabels/a_label_inside_a_label_composes_the_shifts (0.00s)
//	        Images("Text :sup[a :sub[![t](t.png)] b] end\n") =
//	             []
//	            want
//	             ["![t](t.png)|t|t.png"]
//
// The rows that do NOT appear there passed on both versions and are the good
// cases: the inline-code carve-out, the empty label, the reference-image PIN
// the table labels as one, and the leaf and container images of the
// three-form document.
func TestImages_DirectiveLabels(t *testing.T) {
	t.Parallel()
	for _, c := range imageLabelCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := markdown.NewSource([]byte(c.src))
			if got := imageTexts(c.src, s.Images()); !eq(got, c.want) {
				t.Errorf("Images(%q) =\n %q\nwant\n %q", c.src, got, c.want)
			}
			if u := s.UnlocatedImages(); u != 0 {
				t.Errorf("UnlocatedImages() = %d, want 0", u)
			}
		})
	}
}

// TestImages_RewriteALabelDestinationInOneParse is the user-visible half:
// the span a caller gets for an image inside a text directive's label has to
// be the span it can SPLICE, not just one that reads back. An off-by-one
// shift passes a text comparison against the label's bytes and destroys the
// document here.
//
// Measured with the label descent removed:
//
//	--- FAIL: TestImages_RewriteALabelDestinationInOneParse (0.00s)
//	    built 1 edits, want 2
//
// The prose image beside the label one is the good case: it keeps the count
// honest, so a descent that reported the label image TWICE fails here too.
func TestImages_RewriteALabelDestinationInOneParse(t *testing.T) {
	t.Parallel()
	const src = "See :sup[![a](old.png)] and ![b](old2.png)\n"
	s := markdown.NewSource([]byte(src))

	var edits []markdown.Edit
	for _, im := range s.Images() {
		edits = append(edits, markdown.Edit{
			Span: im.Dest, Text: "new" + strings.TrimPrefix(string(s.Text(im.Dest)), "old"),
		})
	}
	if len(edits) != 2 {
		t.Fatalf("built %d edits, want 2", len(edits))
	}
	got, err := s.Apply(edits...)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	const want = "See :sup[![a](new.png)] and ![b](new2.png)\n"
	if string(got) != want {
		t.Errorf("Apply =\n%q\nwant\n%q", got, want)
	}
}
