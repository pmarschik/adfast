package adfast

import "testing"

// A typed dialect kind reads its payload from the places its ADF node
// has room for, and a directive that spells none of them re-derives to
// nothing. The format leg used to let that nothing through, so the
// directive vanished from the document: "see :status{text=Done
// color=green} in the log" formatted to "see  in the log". That is a
// formatter deleting an author's words, which is the one thing
// NormalizeFormat exists to forbid.
//
// The leg keeps the node the author wrote instead. Every dialect kind
// renders its directive form from its raw attribute map, so keeping it
// writes back what was parsed, down to the attribute the kind ignores —
// which is also what the faithful ast→md render produces and what
// prettier 3.8.1, having no directive grammar at all, leaves untouched.

func TestFormatKeepsADirectiveTheKindCannotRead(t *testing.T) {
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
		// The reported shape: ADF's status node has a text attribute,
		// so an author may well write one, but the directive grammar
		// spells the text as the LABEL and the kind reads it only from
		// there.
		{
			name: "status with a text attribute the kind does not read",
			src:  "see :status{text=Done color=green} in the log",
			want: "see :status{color=\"green\" text=\"Done\"} in the log\n",
		},
		{
			name: "status with no label at all",
			src:  "see :status{color=red} in the log",
			want: "see :status{color=\"red\"} in the log\n",
		},
		// A mark kind is the case no attribute reading could ever save:
		// ADF's underline is a MARK, and a mark with no text under it
		// has nowhere to put an author's payload.
		{name: "underline with nothing to mark", src: "see :u{a=b} in the log", want: "see :u{a=\"b\"} in the log\n"},
		{name: "text color with nothing to mark", src: "see :color{x=1} in the log", want: "see :color{x=\"1\"} in the log\n"},
		{name: "background with nothing to mark", src: "see :bg{x=1} in the log", want: "see :bg{x=\"1\"} in the log\n"},
		{name: "subscript with nothing to mark", src: "see :sub{x=1} in the log", want: "see :sub{x=\"1\"} in the log\n"},
		{name: "superscript with nothing to mark", src: "see :sup{x=1} in the log", want: "see :sup{x=\"1\"} in the log\n"},
		// The retired mark and the id-less annotation dissolve to their
		// own inline text, which is nothing when there is none.
		{name: "retired font size with nothing to mark", src: "see :fontSize{size=20} in the log", want: "see :fontSize{size=\"20\"} in the log\n"},
		{name: "annotation with nothing to anchor", src: "see :annotation{x=1} in the log", want: "see :annotation{x=\"1\"} in the log\n"},
		// The remaining atoms, one per missing payload.
		{name: "emoji with no shortName", src: "see :emoji{id=x} in the log", want: "see :emoji{#x} in the log\n"},
		{name: "mention with no name", src: "see :mention{x=1} in the log", want: "see :mention{x=\"1\"} in the log\n"},
		{name: "date with no timestamp and an unparseable label", src: "see :date[nonsense]{x=1} in the log", want: "see :date[nonsense]{x=\"1\"} in the log\n"},
		{name: "placeholder with a text attribute the kind does not read", src: "see :placeholder{text=Hi} in the log", want: "see :placeholder{text=\"Hi\"} in the log\n"},
		// The inline extension is spelled ":extension" — the one dialect
		// name that spans all three directive surfaces.
		{name: "inline extension with neither type nor key", src: "see :extension{x=1} in the log", want: "see :extension{x=\"1\"} in the log\n"},
		// The marks around the kept node are the author's own and ride
		// with it, like the marks around any other inline node the
		// formatter cannot re-derive.
		{name: "the enclosing emphasis rides along", src: "a *:status{color=red}* b", want: "a _:status{color=\"red\"}_ b\n"},
		{name: "the enclosing link rides along", src: "[:status{color=red}](https://e.com)x", want: "[:status{color=\"red\"}](https://e.com)x\n"},

		// GOOD ROWS — shapes that already worked and must not move.
		//
		// A payload the kind DOES read still re-derives to the canonical
		// form. These are the proof the keep did not swallow the
		// canonicalization it backs up.
		{name: "status whose label the kind reads", src: "see :status[Done]{color=green} in the log", want: "see :status[Done]{color=\"green\"} in the log\n"},
		{name: "status re-derives its default color", src: "see :status[Done] in the log", want: "see :status[Done]{color=\"neutral\"} in the log\n"},
		{name: "date re-derives the timestamp from its label", src: "see :date[2024-01-01] in the log", want: "see :date[2024-01-01]{timestamp=\"1704067200000\"} in the log\n"},
		{name: "date re-derives the label from its timestamp", src: "see :date{timestamp=1704067200000} in the log", want: "see :date[2024-01-01]{timestamp=\"1704067200000\"} in the log\n"},
		{
			// The one canonicalization that rewrites the ADF as well:
			// an emoji with a known shortname becomes its character on
			// both legs, so the encode invariant below is not this
			// row's to keep.
			name:       "emoji still projects to its character",
			src:        "see :emoji{shortName=:smile:} in the log",
			want:       "see 😄 in the log\n",
			adfDiffers: true,
		},
		{name: "mention keeps its account id", src: "see :mention[Jane]{#712020:aa} in the log", want: "see :mention[Jane]{#712020:aa} in the log\n"},
		{name: "underline still wraps the text it marks", src: "see :u[x] in the log", want: "see :u[x] in the log\n"},
		{name: "inline extension keeps the type and key it reads", src: "see :extension{type=com.x key=k} in the log", want: "see :extension{key=\"k\" type=\"com.x\"} in the log\n"},
		// A name the dialect does not know was never dropped on this
		// leg: it stays a generic text directive and comes back spelled
		// as written. The keep makes the known names behave the same
		// way, so this row is where that "same way" is written down.
		{name: "an unknown name was always kept", src: "see :foo{a=b} in the log", want: "see :foo{a=\"b\"} in the log\n"},
		// The bare-name rule's own shape: a KNOWN name with no payload
		// at all is prose, not a directive, and stays prose.
		{name: "a bare known name is still prose", src: "see :status in the log", want: "see :status in the log\n"},
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
			// The keep is invisible to the encode: ToADF drops the node
			// itself, which is what keeps ToADF∘NormalizeFormat == ToADF.
			if adfGot, adfWant := marshalADF(t, got), marshalADF(t, tc.src); !tc.adfDiffers && adfGot != adfWant {
				t.Errorf("format changed meaning:\n adf(fmt): %s\n adf(src): %s", adfGot, adfWant)
			}
		})
	}
}
