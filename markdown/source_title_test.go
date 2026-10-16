package markdown_test

import (
	"testing"

	"github.com/pmarschik/adfast/markdown"
)

// titleCase is one source plus what every Title in it slices to, in document
// order. An absent title is the sentinel noTitle (source_definitions_test.go
// defines it), which is not a string a title can hold, so it cannot be
// confused with a title written empty.
type titleCase struct {
	name string
	src  string
	want []string
}

var linkTitleCases = []titleCase{{
	name: "no title written",
	src:  "[a](x.md)\n",
	want: []string{noTitle},
}, {
	name: "a double-quoted title",
	src:  "[a](x.md \"t\")\n",
	want: []string{"t"},
}, {
	name: "a single-quoted title",
	src:  "[a](x.md 't')\n",
	want: []string{"t"},
}, {
	name: "a parenthesized title",
	src:  "[a](x.md (t))\n",
	want: []string{"t"},
}, {
	name: "a title written empty is distinct from no title",
	src:  "[a](x.md \"\")\n",
	want: []string{""},
}, {
	name: "an escaped delimiter inside the title",
	src:  "[a](x.md \"a \\\" b\")\n",
	want: []string{"a \\\" b"},
}, {
	name: "a full reference link has no title at the link",
	src:  "[a][id]\n\n[id]: x.md \"t\"\n",
	want: []string{noTitle},
}, {
	name: "a collapsed reference link has no title at the link",
	src:  "[id][]\n\n[id]: x.md \"t\"\n",
	want: []string{noTitle},
}, {
	name: "a shortcut reference link has no title at the link",
	src:  "[id]\n\n[id]: x.md \"t\"\n",
	want: []string{noTitle},
}, {
	name: "a title on the next line of a list item",
	src:  "- [a](x.md\n  \"t\")\n",
	want: []string{"t"},
}, {
	// The blockquote prefix between the two halves of the tail is not part of
	// the title's content.
	name: "a title on the next line of a blockquote",
	src:  "> [a](x.md\n> \"t\")\n",
	want: []string{"t"},
}}

// TestLinks_Title drives linkTitleCases. See Definition.Title and Link.Title
// for the contract this pins: the zero Span for no title and for a reference
// form, and a non-zero empty Span for a title written empty.
func TestLinks_Title(t *testing.T) {
	t.Parallel()
	for _, c := range linkTitleCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := markdown.Links([]byte(c.src))
			if len(got) != len(c.want) {
				t.Fatalf("Links(%q) = %d links, want %d", c.src, len(got), len(c.want))
			}
			for i, w := range c.want {
				if s := partText(c.src, got[i].Title, true); s != w {
					t.Errorf("link %d title %v sliced to %q, want %q", i, got[i].Title, s, w)
				}
			}
		})
	}
}

var imageTitleCases = []titleCase{{
	name: "no title written",
	src:  "![a](x.png)\n",
	want: []string{noTitle},
}, {
	name: "a double-quoted title",
	src:  "![a](x.png \"t\")\n",
	want: []string{"t"},
}, {
	name: "a single-quoted title",
	src:  "![a](x.png 't')\n",
	want: []string{"t"},
}, {
	name: "a parenthesized title",
	src:  "![a](x.png (t))\n",
	want: []string{"t"},
}, {
	name: "a title written empty is distinct from no title",
	src:  "![a](x.png \"\")\n",
	want: []string{""},
}, {
	name: "an escaped delimiter inside the title",
	src:  "![a](x.png \"a \\\" b\")\n",
	want: []string{"a \\\" b"},
}, {
	name: "a full reference image has no title at the image",
	src:  "![a][id]\n\n[id]: x.png \"t\"\n",
	want: []string{noTitle},
}, {
	name: "a collapsed reference image has no title at the image",
	src:  "![id][]\n\n[id]: x.png \"t\"\n",
	want: []string{noTitle},
}, {
	name: "a shortcut reference image has no title at the image",
	src:  "![id]\n\n[id]: x.png \"t\"\n",
	want: []string{noTitle},
}}

// TestImages_Title drives imageTitleCases. See Image.Title for the contract.
func TestImages_Title(t *testing.T) {
	t.Parallel()
	for _, c := range imageTitleCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := markdown.Images([]byte(c.src))
			if len(got) != len(c.want) {
				t.Fatalf("Images(%q) = %d images, want %d", c.src, len(got), len(c.want))
			}
			for i, w := range c.want {
				if s := partText(c.src, got[i].Title, true); s != w {
					t.Errorf("image %d title %v sliced to %q, want %q", i, got[i].Title, s, w)
				}
			}
		})
	}
}
