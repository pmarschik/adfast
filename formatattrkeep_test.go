package adfast

import "testing"

// A typed dialect kind re-derives its canonical payload from a per-kind
// allowlist mirroring that kind's EncodeADF, and the format leg used to
// delete every attribute outside that list. Measured before this rule:
//
//	":media{x=1}"                      -> ":media"
//	":media{0}"                        -> ":media"
//	":status[Done]{color=red foo=bar}" -> ":status[Done]{color=\"red\"}"
//	":u[x]{a=b}"                       -> ":u[x]"
//	":mention[Bob]{id=1 zz=2}"         -> ":mention[Bob]{#1}"
//
// The encode leg drops those attributes because ADF has nowhere to put
// them. The format leg writes MARKDOWN, where the author's own spelling
// is what it has to give back — an unknown directive name already
// survives spelled as written, and prettier 3.8.1, having no directive
// grammar at all, leaves every one of these untouched.
//
// For ":media" it was worse than deleted text. The bare-name rule reads
// a payload-less KNOWN name back as prose, so the chip that went in came
// out as the word ":media": the formatter changed the document's
// meaning, which is why every row below also asserts the ADF is equal
// on both sides. FuzzFormatSemanticsPreserved found ":media{0}" for
// that reason and its seed pins it.
//
// The same bare-name rule is what "media spelling only the default type"
// is about: the chip is the one inline kind whose entire canonical
// payload is optional, so ":media{type=file}" re-derived to a bare
// ":media" and became prose with no unread attribute involved at all.

func TestFormatKeepsADirectiveAttributeTheKindCannotRead(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		src  string
		want string
		// adfDiffers marks a row whose canonicalization is itself an
		// ADF-visible rewrite, so the encode invariant below does not
		// apply to it.
		adfDiffers bool
	}{
		// The reported shape, and the one that changed meaning: the
		// unread attribute was the chip's whole payload, so deleting it
		// left a bare name the parser reads as prose.
		{name: "media chip with an attribute the kind cannot read", src: ":media{x=1}", want: ":media{x=\"1\"}\n"},
		{name: "the fuzzer's crasher", src: ":media{0}", want: ":media{0}\n"},
		{name: "media chip mid-sentence", src: "see :media{x=1} in the log", want: "see :media{x=\"1\"} in the log\n"},
		// The atoms that kept their node and lost only the author's
		// text. Each re-derives a payload the kind DOES read, and the
		// leftovers now ride along beside it.
		{name: "status keeps an attribute beside its color", src: ":status[Done]{color=red foo=bar}", want: ":status[Done]{color=\"red\" foo=\"bar\"}\n"},
		{name: "mention keeps an attribute beside its id", src: ":mention[Bob]{id=1 zz=2}", want: ":mention[Bob]{#1 zz=\"2\"}\n"},
		{name: "date keeps an attribute beside its timestamp", src: ":date{timestamp=0 zz=1}", want: ":date[1970-01-01]{timestamp=\"0\" zz=\"1\"}\n"},
		{name: "placeholder keeps an attribute beside its text", src: ":placeholder[hi]{zz=1}", want: ":placeholder[hi]{zz=\"1\"}\n"},
		{name: "inline extension keeps an attribute beside its key", src: ":extension{type=com.x key=k zz=1}", want: ":extension{key=\"k\" type=\"com.x\" zz=\"1\"}\n"},
		// An emoji with a known shortname projects to its character,
		// and a character has no node an attribute could ride on — so
		// the projection waits when there is one to lose. The pure
		// projection is a GOOD row below.
		{name: "emoji with an unread attribute keeps its directive form", src: ":emoji{shortName=:smile: zz=1}", want: ":emoji{shortName=\":smile:\" zz=\"1\"}\n"},
		// The MARK kinds are the half no attribute-carrying could save:
		// ":u" is a mark pushed onto the context and dissolved, and ADF
		// gives underline no attribute to hold "a=b". The author's node
		// is kept whole instead, children and all.
		{name: "underline keeps an attribute the mark cannot hold", src: ":u[x]{a=b}", want: ":u[x]{a=\"b\"}\n"},
		{name: "subscript keeps an attribute the mark cannot hold", src: ":sub[x]{a=b}", want: ":sub[x]{a=\"b\"}\n"},
		{name: "superscript keeps an attribute the mark cannot hold", src: ":sup[x]{a=b}", want: ":sup[x]{a=\"b\"}\n"},
		{name: "text color keeps an attribute beside its color", src: ":color[x]{color=red zz=1}", want: ":color[x]{color=\"red\" zz=\"1\"}\n"},
		{name: "background keeps an attribute beside its color", src: ":bg[x]{color=red zz=1}", want: ":bg[x]{color=\"red\" zz=\"1\"}\n"},
		{name: "annotation keeps an attribute beside its id", src: ":annotation[x]{id=1 zz=2}", want: ":annotation[x]{#1 zz=\"2\"}\n"},
		// A kept mark still normalizes what it marks, in both nesting
		// directions, so the keep does not freeze a subtree.
		{name: "a kept mark normalizes the marks inside it", src: ":u[:color[x]{color=red}]{a=b}", want: ":u[:color[x]{color=\"red\"}]{a=\"b\"}\n"},
		{name: "a kept mark re-derives the payload inside it", src: ":u[:status[Done]]{a=b}", want: ":u[:status[Done]{color=\"neutral\"}]{a=\"b\"}\n"},
		{name: "a kept mark nests inside a re-derived one", src: ":color[:u[x]]{color=red zz=1}", want: ":color[:u[x]]{color=\"red\" zz=\"1\"}\n"},
		{name: "the enclosing emphasis rides along", src: "a *:u[x]{a=b}* b", want: "a _:u[x]{a=\"b\"}_ b\n"},

		// The media chip that spelled only its DEFAULT type: no unread
		// attribute, but the canonical form drops the default, and a
		// bare known name re-parses as prose.
		{name: "media chip spelling only the default type", src: ":media{type=file}", want: ":media{type=\"file\"}\n"},
		{name: "media chip spelling only the default type mid-sentence", src: "a :media{type=file} b", want: "a :media{type=\"file\"} b\n"},
		{name: "media chip whose only attribute is an empty id", src: ":media{id=}", want: ":media{type=\"file\"}\n"},

		// GOOD ROWS — shapes that already worked and must not move.
		//
		// The canonical payload still wins on every attribute the kind
		// DOES read: no default is re-spelled, no filler is added, and
		// nothing gains a second spelling.
		{name: "a chip with an id needs no filler type", src: ":media{id=abc}", want: ":media{#abc}\n"},
		{name: "a non-default type is written as spelled", src: ":media{type=external}", want: ":media{type=\"external\"}\n"},
		{name: "a chip with alt text needs no filler type", src: ":media[alt]", want: ":media[alt]\n"},
		{name: "an empty collection is a payload, not a default", src: ":media{collection=}", want: ":media{collection}\n"},
		{name: "status re-derives its canonical color", src: ":status[Done]{color=red}", want: ":status[Done]{color=\"red\"}\n"},
		{name: "mention re-derives its id spelling", src: ":mention[Bob]{id=1}", want: ":mention[Bob]{#1}\n"},
		{name: "annotation re-derives its default type", src: ":annotation[x]{id=1}", want: ":annotation[x]{#1 annotationType=\"inlineComment\"}\n"},
		{name: "underline still dissolves to a mark", src: ":u[x]", want: ":u[x]\n"},
		{name: "text color still dissolves to a mark", src: ":color[x]{color=red}", want: ":color[x]{color=\"red\"}\n"},
		{name: "date still re-derives its label", src: ":date[2024-01-01]", want: ":date[2024-01-01]{timestamp=\"1704067200000\"}\n"},
		{name: "inline extension re-derives what it reads", src: ":extension{type=com.x key=k}", want: ":extension{key=\"k\" type=\"com.x\"}\n"},
		{
			// The projection stays for the pure case: an emoji with a
			// known shortname becomes its character on both legs, and
			// the format contract skips the class (docHasEmoji) because
			// markdown persistence sheds the node either way.
			name:       "emoji with nothing to lose still projects to its character",
			src:        ":emoji{shortName=:smile:}",
			want:       "😄\n",
			adfDiffers: true,
		},
		// The bare-name rule's own shape: a KNOWN name with no payload
		// at all is prose on the way IN, and stays prose.
		{name: "a bare known name is still prose", src: ":media", want: ":media\n"},
		// A name the dialect does not know was never touched, and is
		// where "spelled as written" is written down.
		{name: "an unknown name was always kept", src: ":foo{a=b}", want: ":foo{a=\"b\"}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := fmtMD(tc.src)
			if got != tc.want {
				t.Errorf("format %q\n got  %q\n want %q", tc.src, got, tc.want)
			}
			// Totality is a fixpoint claim: a leg that deletes on the
			// second pass is still lossy.
			if again := fmtMD(got); again != got {
				t.Errorf("format is not a fixpoint for %q\n once  %q\n twice %q", tc.src, got, again)
			}
			// The contract FuzzFormatSemanticsPreserved enforces, and
			// the half of this defect that was a meaning change rather
			// than a text deletion.
			if adfGot, adfWant := marshalADF(t, got), marshalADF(t, tc.src); !tc.adfDiffers && adfGot != adfWant {
				t.Errorf("format changed meaning:\n adf(fmt): %s\n adf(src): %s", adfGot, adfWant)
			}
		})
	}
}
