package adfast

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/convert"
)

// The bug this file exists for: the INLINE media chip, `:media[alt]{path=…}`,
// ignored its `path` outright. The block spellings (::media, :::media) and the
// markdown image form all resolve a path through the asset store into the
// media id ADF addresses an attachment by; the chip did not, so it encoded to
// a mediaInline with an EMPTY id even when the store held the file — and said
// nothing about it. Three ways of naming one picture, and the inline one was
// the only one that could not name it.
//
// The silence was the second half. The block forms report unresolved-asset
// when a path resolves to no id; the chip reported nothing whether the path
// was good or bad, because it never looked.
//
// Every probe below carries the RESOLVABLE case in the same document as the
// one under test, so an implementation that satisfies the assertion by
// resolving nothing (or by reporting everything) fails here too.

// oneAssetStore is the whole asset store these probes run under: one path is
// in it, everything else is not.
func oneAssetStore() []Option {
	return []Option{
		WithAssetIDResolver(func(ref string) (string, bool) {
			return "aaaa-id", ref == "assets/logo.png"
		}),
	}
}

// collectUnresolved wires a sink that keeps only the unresolved-asset
// messages, so an unrelated diagnostic cannot pad the counts below.
func collectUnresolved(messages *[]string) Option {
	return WithDiagnostics(func(d convert.Diagnostic) {
		if d.Code == convert.CodeUnresolvedAsset {
			*messages = append(*messages, d.Message)
		}
	})
}

// Mutation proof for the encode half. With the resolution in
// MediaInline.EncodeADF reduced to a call-then-discard (the package still
// compiles, the chip just keeps whatever id it was written with):
//
//	mi.ID = mediasrc.ID(mi.ID, n.Attrs["path"], ctx.AssetID)
//	->
//	_ = mediasrc.ID(mi.ID, n.Attrs["path"], ctx.AssetID)
//
//	--- FAIL: TestInlineMediaResolvesItsPath (0.00s)
//	    inlinemediapath_test.go:76: the chip must carry the id the store
//	        answers with
//	          want {"type":"mediaInline","attrs":{"alt":"logo","id":"aaaa-id","type":"file"}}
//	          got  {"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"a "},{"type":"mediaInline","attrs":{"alt":"logo","type":"file"}},{"type":"text","text":" b "},{"type":"mediaInline","attrs":{"alt":"pinned","id":"pinned-id","type":"file"}},{"type":"text","text":" c"}]}],"version":1}
//
// And forced always-on — resolving the path even when an explicit id is
// spelled beside it, `mi.ID = mediasrc.ID("", n.Attrs["path"], ctx.AssetID)`
// — the good case in the same document names the overreach:
//
//	--- FAIL: TestInlineMediaResolvesItsPath (0.00s)
//	    inlinemediapath_test.go:84: an explicit id must win over the path
//	        beside it, not be re-derived from it
//	          want {"type":"mediaInline","attrs":{"alt":"pinned","id":"pinned-id","type":"file"}}
//	          got  {"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"a "},{"type":"mediaInline","attrs":{"alt":"logo","id":"aaaa-id","type":"file"}},{"type":"text","text":" b "},{"type":"mediaInline","attrs":{"alt":"pinned","id":"aaaa-id","type":"file"}},{"type":"text","text":" c"}]}],"version":1}
func TestInlineMediaResolvesItsPath(t *testing.T) {
	const md = "a :media[logo]{path=assets/logo.png} b " +
		":media[pinned]{id=pinned-id path=assets/logo.png} c\n"
	got := adfJSON(t, mdToADF(md, oneAssetStore()...))

	// The path is a lookup key: the store's answer becomes the id.
	wantResolved := `{"type":"mediaInline","attrs":{"alt":"logo","id":"aaaa-id","type":"file"}}`
	if !strings.Contains(got, wantResolved) {
		t.Errorf("the chip must carry the id the store answers with\n  want %s\n  got  %s",
			wantResolved, got)
	}
	// And an explicit id is the author's, not the store's — mediasrc.ID
	// returns a given id untouched and only looks a path up when there is
	// none, so the pin survives the file sitting beside it.
	wantPinned := `{"type":"mediaInline","attrs":{"alt":"pinned","id":"pinned-id","type":"file"}}`
	if !strings.Contains(got, wantPinned) {
		t.Errorf("an explicit id must win over the path beside it, not be re-derived from it\n  want %s\n  got  %s",
			wantPinned, got)
	}
}

// Mutation proof for the reporting half. With the reportMediaSource call
// removed from inlineFlattener.VisitExtension the encode is unchanged and
// only the sentence goes missing:
//
//	--- FAIL: TestInlineMediaWithAnUnresolvablePathIsReported (0.00s)
//	    inlinemediapath_test.go:126: want one unresolved-asset diagnostic —
//	        for the unresolvable chip only — got []
//
// And with the id/external guard dropped from UnresolvedMediaPath (reporting
// any path, resolvable or not), the resolvable chip in the same paragraph
// names the false alarm:
//
//	--- FAIL: TestInlineMediaWithAnUnresolvablePathIsReported (0.00s)
//	    inlinemediapath_test.go:126: want one unresolved-asset diagnostic —
//	        for the unresolvable chip only — got [media directive path
//	        assets/logo.png has no media id (not in the asset store); the
//	        media node is written without one, so the attachment cannot be
//	        addressed, and the path does not travel in ADF media directive
//	        path assets/gone.png has no media id (not in the asset store);
//	        the media node is written without one, so the attachment cannot
//	        be addressed, and the path does not travel in ADF]
func TestInlineMediaWithAnUnresolvablePathIsReported(t *testing.T) {
	const md = "a :media[logo]{path=assets/logo.png} b :media[gone]{path=assets/gone.png} c\n"
	var messages []string
	got := adfJSON(t, mdToADF(md, append(oneAssetStore(), collectUnresolved(&messages))...))

	if len(messages) != 1 {
		t.Fatalf("want one %s diagnostic — for the unresolvable chip only — got %v",
			convert.CodeUnresolvedAsset, messages)
	}
	// The same code and the same sentence the block forms emit: the loss is
	// the same loss, so an inline chip is not a second thing for a consumer
	// to special-case.
	for _, part := range []string{"assets/gone.png", "has no media id", "cannot be addressed"} {
		if !strings.Contains(messages[0], part) {
			t.Errorf("the diagnostic must say %q, got %q", part, messages[0])
		}
	}
	// The good case rides in the same paragraph and still resolves.
	if !strings.Contains(got, `"id":"aaaa-id"`) {
		t.Errorf("the resolvable chip must still resolve:\n%s", got)
	}
	// A PIN, not a fix: the reported chip still SHIPS, without an id. The
	// author wrote a chip and the alt text is theirs; what changed is that
	// the encode says the attachment cannot be found.
	wantReported := `{"type":"mediaInline","attrs":{"alt":"gone","type":"file"}}`
	if !strings.Contains(got, wantReported) {
		t.Errorf("the unresolvable chip's node changed\n  want %s\n  got  %s", wantReported, got)
	}
}

// Every spelling's format in ONE table, because the bug this half of the
// file now covers is precisely the two of them disagreeing: the block
// form's mediaSourceAttrs wrote the path INSTEAD of an explicit id, so a
// mere format discarded a pin the inline chip's leg kept — and the next
// encode re-derived the id from the file, addressing whatever attachment
// the store answers with today.
//
// The two halves live in two functions — normalizeMediaInline for the
// chip, mediaSourceAttrs for the block and caption forms — and each was
// mutated ALONE. The failing sets are disjoint, so neither half is
// carrying the other.
//
// Inline half, the path carry reduced to a call-then-discard so the
// package still compiles (`_ = v.Attrs["path"]`) — the two inline rows
// fail and no block row does:
//
//	--- FAIL: TestFormatKeepsAMediaSource/an_inline_chip (0.00s)
//	    inlinemediapath_test.go:326: a format must not rewrite the author's source away
//	          in   "a :media[logo]{path=assets/logo.png} b\n" (39 bytes)
//	          want "a :media[logo]{path=\"assets/logo.png\"} b\n" (41 bytes)
//	          got  "a :media[logo] b\n" (17 bytes)
//	--- FAIL: TestFormatKeepsAMediaSource/a_pinned_id_beside_a_path (0.00s)
//	    inlinemediapath_test.go:326: a format must not rewrite the author's source away
//	          in   "a :media[pinned]{id=pinned-id path=assets/logo.png} b\n" (54 bytes)
//	          want "a :media[pinned]{#pinned-id path=\"assets/logo.png\"} b\n" (54 bytes)
//	          got  "a :media[pinned]{#pinned-id} b\n" (31 bytes)
//
// Block half, the pin carry in convert's mediaSourceAttrs likewise
// discarded rather than deleted:
//
//	if m.pinnedID != "" && m.path != "" {
//		attrs["id"] = m.pinnedID
//	}
//	->
//	_ = m.pinnedID
//
// the three pinned block rows fail and no inline row does:
//
//	--- FAIL: TestFormatKeepsAMediaSource/a_pinned_id_beside_a_path_on_the_block_form (0.00s)
//	    inlinemediapath_test.go:326: a format must not rewrite the author's source away
//	          in   "::media[logo]{#pinned-id path=assets/logo.png}\n" (47 bytes)
//	          want "::media[logo]{#pinned-id path=\"assets/logo.png\"}\n" (49 bytes)
//	          got  "::media[logo]{path=\"assets/logo.png\"}\n" (38 bytes)
//	--- FAIL: TestFormatKeepsAMediaSource/a_pinned_id_beside_a_path_on_the_caption_form (0.00s)
//	    inlinemediapath_test.go:326: a format must not rewrite the author's source away
//	          in   ":::media[logo]{#pinned-id path=assets/logo.png}\nA caption.\n:::\n" (63 bytes)
//	          want ":::media[logo]{#pinned-id path=\"assets/logo.png\"}\nA caption.\n:::\n" (65 bytes)
//	          got  ":::media[logo]{path=\"assets/logo.png\"}\nA caption.\n:::\n" (54 bytes)
//	--- FAIL: TestFormatKeepsAMediaSource/a_pinned_id_beside_an_unresolvable_path (0.00s)
//	    inlinemediapath_test.go:326: a format must not rewrite the author's source away
//	          in   "::media[logo]{#pinned-id path=assets/gone.png}\n" (47 bytes)
//	          want "::media[logo]{#pinned-id path=\"assets/gone.png\"}\n" (49 bytes)
//	          got  "::media[logo]{path=\"assets/gone.png\"}\n" (38 bytes)
//
// The block carry's condition has two halves of its own, and each has its
// own good row. Degenerating it three ways:
//
// Writing the RESOLVED id, `attrs["id"] = m.id`, which for a path-only
// directive is the store's answer — pinning a derived fact into the
// document is exactly what leaving the id out avoids:
//
//	--- FAIL: TestFormatKeepsAMediaSource/the_block_form_carries_only_the_path_it_was_given (0.00s)
//	    inlinemediapath_test.go:326: a format must not rewrite the author's source away
//	          in   "::media[logo]{path=assets/logo.png}\n" (36 bytes)
//	          want "::media[logo]{path=\"assets/logo.png\"}\n" (38 bytes)
//	          got  "::media[logo]{#aaaa-id path=\"assets/logo.png\"}\n" (47 bytes)
//	--- FAIL: TestFormatKeepsAMediaSource/a_cataloged_id_becomes_the_catalog's_path (0.00s)
//	    inlinemediapath_test.go:326: a format must not rewrite the author's source away
//	          in   "::media[shot]{#cataloged-id}\n" (29 bytes)
//	          want "::media[shot]{path=\"assets/shot.png\"}\n" (38 bytes)
//	          got  "::media[shot]{#cataloged-id path=\"assets/shot.png\"}\n" (52 bytes)
//
// Dropping the whole guard, `attrs["id"] = m.pinnedID`, which writes a
// bare valueless `id` the re-parse reads as an attribute:
//
//	--- FAIL: TestFormatKeepsAMediaSource/the_block_form_carries_only_the_path_it_was_given (0.00s)
//	    inlinemediapath_test.go:326: a format must not rewrite the author's source away
//	          in   "::media[logo]{path=assets/logo.png}\n" (36 bytes)
//	          want "::media[logo]{path=\"assets/logo.png\"}\n" (38 bytes)
//	          got  "::media[logo]{id path=\"assets/logo.png\"}\n" (41 bytes)
//	--- FAIL: TestFormatKeepsAMediaSource/a_cataloged_id_becomes_the_catalog's_path (0.00s)
//	    inlinemediapath_test.go:326: a format must not rewrite the author's source away
//	          in   "::media[shot]{#cataloged-id}\n" (29 bytes)
//	          want "::media[shot]{path=\"assets/shot.png\"}\n" (38 bytes)
//	          got  "::media[shot]{#cataloged-id path=\"assets/shot.png\"}\n" (52 bytes)
//
// And dropping only the AUTHOR'S-PATH half — `_ = m.path`, keeping the
// pin whenever an id was spelled at all, even where the path came from
// the download catalog rather than the author. Only the catalog row
// names this one, which is why it is in the table:
//
//	--- FAIL: TestFormatKeepsAMediaSource/a_cataloged_id_becomes_the_catalog's_path (0.00s)
//	    inlinemediapath_test.go:326: a format must not rewrite the author's source away
//	          in   "::media[shot]{#cataloged-id}\n" (29 bytes)
//	          want "::media[shot]{path=\"assets/shot.png\"}\n" (38 bytes)
//	          got  "::media[shot]{#cataloged-id path=\"assets/shot.png\"}\n" (52 bytes)
func TestFormatKeepsAMediaSource(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
		// extra is the option a single row needs on top of the shared
		// store — only the download-catalog row does.
		extra []Option
	}{
		{
			name: "an inline chip",
			in:   "a :media[logo]{path=assets/logo.png} b\n",
			want: "a :media[logo]{path=\"assets/logo.png\"} b\n",
		},
		{
			// An explicit id and a path both survive. Writing only the
			// path would re-point a pinned id at whatever the store now
			// says the file is; the encode prefers the id and never spends
			// the path beside it, so keeping the pair costs nothing.
			name: "a pinned id beside a path",
			in:   "a :media[pinned]{id=pinned-id path=assets/logo.png} b\n",
			want: "a :media[pinned]{#pinned-id path=\"assets/logo.png\"} b\n",
		},
		{
			// The good case for the carry: no path spelled, no path
			// written. A carry that fired unconditionally would write a
			// bare `path` attribute here — an empty lookup key the
			// re-parse reads as a source.
			name: "no path at all",
			in:   "a :media[chip]{#pinned-id} b\n",
			want: "a :media[chip]{#pinned-id} b\n",
		},
		{
			// The same fact in the block spelling, and the reason both
			// spellings sit in one table: this row used to come back
			// path-only, so the pin an author wrote survived a format on
			// one leg and not on the other.
			name: "a pinned id beside a path on the block form",
			in:   "::media[logo]{#pinned-id path=assets/logo.png}\n",
			want: "::media[logo]{#pinned-id path=\"assets/logo.png\"}\n",
		},
		{
			// And in the caption spelling, which shares the same
			// projection (mediaSourceAttrs) and so shared the same loss.
			name: "a pinned id beside a path on the caption form",
			in:   ":::media[logo]{#pinned-id path=assets/logo.png}\nA caption.\n:::\n",
			want: ":::media[logo]{#pinned-id path=\"assets/logo.png\"}\nA caption.\n:::\n",
		},
		{
			// The good case for the block carry: a path-addressed
			// directive spells NO id, and none may appear. m.id here is
			// the store's answer to the path — writing it back would pin a
			// derived fact into the document and let it go stale.
			name: "the block form carries only the path it was given",
			in:   "::media[logo]{path=assets/logo.png}\n",
			want: "::media[logo]{path=\"assets/logo.png\"}\n",
		},
		{
			// A pin whose path resolves to NOTHING is still a pin: the
			// author's id is the only thing addressing the attachment, and
			// the format that dropped it left a directive that encodes to
			// a media node with an empty id.
			name: "a pinned id beside an unresolvable path",
			in:   "::media[logo]{#pinned-id path=assets/gone.png}\n",
			want: "::media[logo]{#pinned-id path=\"assets/gone.png\"}\n",
		},
		{
			// The OTHER good case, and the second half of the condition:
			// here the author spelled an id and NO path, and the path in
			// the output is the download catalog's answer for that id. The
			// store has just said the two name one attachment, so the id
			// is redundant with the path it produced and the canonical
			// form drops it — the same slimming the decode leg performs
			// (see TestFormatModeSlimsMediaViaResolver). A carry that
			// looked only at "did the author spell an id" would keep it
			// here and un-slim every pulled document.
			name: "a cataloged id becomes the catalog's path",
			in:   "::media[shot]{#cataloged-id}\n",
			want: "::media[shot]{path=\"assets/shot.png\"}\n",
			extra: []Option{WithMediaAssets(map[string]convert.MediaAsset{
				"cataloged-id": {Path: "assets/shot.png"},
			})},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append(oneAssetStore(), tc.extra...)
			got := fmtMD(tc.in, opts...)
			if got != tc.want {
				t.Errorf("a format must not rewrite the author's source away\n"+
					"  in   %q (%d bytes)\n  want %q (%d bytes)\n  got  %q (%d bytes)",
					tc.in, len(tc.in), tc.want, len(tc.want), got, len(got))
			}
			// A format is a fixpoint: the second pass must not move it.
			if again := fmtMD(got, opts...); again != got {
				t.Errorf("the format is not a fixpoint\n  once %q\n  twice %q", got, again)
			}
		})
	}
}

// reIDStore is the store that makes the loss visible: the file went up
// again and got a NEW media id, so the id it now answers with is not the
// one the author pinned. That is the whole hazard — a format that writes
// the path instead of the pin hands the next encode a lookup, and the
// lookup answers something else.
func reIDStore() []Option {
	return []Option{
		WithAssetIDResolver(func(ref string) (string, bool) {
			return "reuploaded-id", ref == "assets/logo.png"
		}),
	}
}

// The harm, measured on the encode rather than on the text: a format must
// not change the attachment the document addresses. The table above pins
// what the format WRITES; this pins what that costs.
//
// The pinned directive and a path-only one sit in the SAME document, so an
// implementation that keeps the pin by refusing to resolve paths at all
// fails on the second block.
//
// Mutation, the block half as a call-then-discard (`_ = m.pinnedID` in
// convert's mediaSourceAttrs):
//
//	--- FAIL: TestFormatKeepsABlockMediaPinAddressable (0.00s)
//	    inlinemediapath_test.go:391: a format must not change the attachment
//	        the document addresses
//	          before {"type":"doc","content":[{"type":"mediaSingle","attrs":{"layout":"align-start"},"content":[{"type":"media","attrs":{"alt":"logo","collection":"","id":"pinned-id","type":"file"}}]},{"type":"mediaSingle","attrs":{"layout":"align-start"},"content":[{"type":"media","attrs":{"alt":"other","collection":"","id":"reuploaded-id","type":"file"}}]}],"version":1}
//	          after  {"type":"doc","content":[{"type":"mediaSingle","attrs":{"layout":"align-start"},"content":[{"type":"media","attrs":{"alt":"logo","collection":"","id":"reuploaded-id","type":"file"}}]},{"type":"mediaSingle","attrs":{"layout":"align-start"},"content":[{"type":"media","attrs":{"alt":"other","collection":"","id":"reuploaded-id","type":"file"}}]}],"version":1}
//	          fmt    "::media[logo]{path=\"assets/logo.png\"}\n\n::media[other]{path=\"assets/logo.png\"}\n"
//
// This test does NOT catch the always-on mutation (writing the resolved
// m.id): a derived id written beside its own path still encodes to that
// same id, so the ADF is unchanged. The table above is what names that
// one, on the text.
func TestFormatKeepsABlockMediaPinAddressable(t *testing.T) {
	const md = "::media[logo]{#pinned-id path=assets/logo.png}\n\n" +
		"::media[other]{path=assets/logo.png}\n"

	before := adfJSON(t, mdToADF(md, reIDStore()...))
	// The pin addresses the pinned attachment, and the path-only directive
	// beside it addresses the one the store answers with — two different
	// attachments, from one file, in one document.
	if !strings.Contains(before, `"id":"pinned-id"`) {
		t.Fatalf("the pin must address the attachment it names:\n%s", before)
	}
	if !strings.Contains(before, `"id":"reuploaded-id"`) {
		t.Fatalf("the path-only directive must still resolve through the store:\n%s", before)
	}

	formatted := fmtMD(md, reIDStore()...)
	after := adfJSON(t, mdToADF(formatted, reIDStore()...))
	if after != before {
		t.Errorf("a format must not change the attachment the document addresses\n"+
			"  before %s\n  after  %s\n  fmt    %q", before, after, formatted)
	}
}

// The DECODE leg is unchanged by all of the above, and this pins that: a
// pull writes the media id it was handed, with the `#id` shorthand, and
// never a path — the ADF it decodes has no path to write. Passes with and
// without the fix, i.e. a preserved-behavior PIN.
func TestDecodedInlineMediaStillWritesItsID(t *testing.T) {
	const doc = `{"version":1,"type":"doc","content":[{"type":"paragraph","content":[
	  {"type":"text","text":"a "},
	  {"type":"mediaInline","attrs":{"id":"aaaa-id","type":"file","collection":"","alt":"logo"}},
	  {"type":"text","text":" b"}]}]}`
	var v any
	if err := json.Unmarshal([]byte(doc), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := adfToMD(v, oneAssetStore()...)
	const want = "a :media[logo]{#aaaa-id collection} b\n"
	if got != want {
		t.Errorf("the decode leg must be untouched\n  want %q\n  got  %q", want, got)
	}
}
