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

// Mutation proof for the format half. With the path carry removed from
// normalizeMediaInline the author's attribute is gone from the output:
//
//	--- FAIL: TestFormatKeepsAnInlineMediaPath/an_inline_chip (0.00s)
//	    inlinemediapath_test.go:172: a format must not rewrite the author's
//	        source away
//	          in   "a :media[logo]{path=assets/logo.png} b\n" (39 bytes)
//	          want "a :media[logo]{path=\"assets/logo.png\"} b\n" (41 bytes)
//	          got  "a :media[logo] b\n" (17 bytes)
//
// The block case is a preserved-behavior PIN — it passes with and without
// the fix — and is here because the two spellings' formats must not drift
// apart again.
func TestFormatKeepsAnInlineMediaPath(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{
			name: "an inline chip",
			in:   "a :media[logo]{path=assets/logo.png} b\n",
			want: "a :media[logo]{path=\"assets/logo.png\"} b\n",
		},
		{
			// An explicit id and a path both survive, unlike the block
			// form's mediaSourceAttrs, which writes the path instead of
			// the id. Writing only the path here would re-point a pinned
			// id at whatever the store now says the file is; the encode
			// prefers the id and never spends the path beside it, so
			// keeping the pair costs nothing.
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
			name: "the block form (a pin)",
			in:   "::media[logo]{path=assets/logo.png}\n",
			want: "::media[logo]{path=\"assets/logo.png\"}\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := fmtMD(tc.in, oneAssetStore()...)
			if got != tc.want {
				t.Errorf("a format must not rewrite the author's source away\n"+
					"  in   %q (%d bytes)\n  want %q (%d bytes)\n  got  %q (%d bytes)",
					tc.in, len(tc.in), tc.want, len(tc.want), got, len(got))
			}
			// A format is a fixpoint: the second pass must not move it.
			if again := fmtMD(got, oneAssetStore()...); again != got {
				t.Errorf("the format is not a fixpoint\n  once %q\n  twice %q", got, again)
			}
		})
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
