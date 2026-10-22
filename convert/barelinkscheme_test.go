package convert

import "testing"

// isBareAutolinkScheme is the whole of what keeps the ADF leg's bare-link
// decision from re-breaking the custom-scheme fuzz crash it was written to
// fix: a link mark whose text equals its href used
// to always render as Bare: true, and an arbitrary scheme like "Ad:" is
// angle-autolink valid but not GFM bare-literal valid — writing it bare
// loses the link outright on first render (round-trip corpus:
// "!<Ad:><Ad:>0", pinned in testdata/fuzz/FuzzRoundTripIdempotent).
//
// Every row here is load-bearing against one of the ways that restriction
// can silently widen back out: an overly permissive prefix check (http/
// https without the "://", or a scheme-position match instead of a
// line-anchored one), a case fold applied to mailto that
// convertGoldmarkAutoLink's own parse does not apply, or — the rows added
// when a literal, unbracketed "mailto:addr" turned out not to re-linkify at
// all under goldmark's extension/linkify.go (util.FindEmailIndex's
// local-part scan breaks at the ':') — treating the full "mailto:addr"
// string as bare-safe the same way http(s) treats text equal to href.
func TestIsBareAutolinkScheme(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		href string
		text string
		want bool
	}{
		{"http", "http://ex.com", "http://ex.com", true},
		{"https", "https://ex.com", "https://ex.com", true},
		{"http uppercase scheme", "HTTP://ex.com", "HTTP://ex.com", true},
		{"https mixed-case scheme", "HTTPs://ex.com", "HTTPs://ex.com", true},
		{"http dotless host", "http://0", "http://0", true},
		// http(s) is bare-safe regardless of text: autolinkText's own
		// text-equals-href gate is what actually decides whether the
		// bare spelling gets used, so a non-matching text here is inert.
		{"http with a differing text is still scheme-safe", "http://ex.com", "Example", true},

		{"mailto address matches the stripped address: bare is safe", "mailto:a@ex.com", "a@ex.com", true},
		{
			"mailto literal full URL as text: bare is NOT safe, a reparse " +
				"drops the link outright (FindEmailIndex breaks at the colon " +
				"in \"mailto:\" before it ever reaches '@')",
			"mailto:a@ex.com", "mailto:a@ex.com", false,
		},
		{"mailto uppercase is not recognized", "MAILTO:a@ex.com", "a@ex.com", false},
		{"mailto mixed case is not recognized", "Mailto:a@ex.com", "a@ex.com", false},
		{"a custom scheme angle-autolink accepts but bare-literal does not", "Ad:x", "Ad:x", false},
		{"tel is angle-autolink valid but not bare-literal valid", "tel:123", "tel:123", false},
		{"ftp is neither http(s) nor mailto", "ftp://ex.com", "ftp://ex.com", false},
		{"www literal has no scheme prefix at all", "www.ex.com", "www.ex.com", false},
		{"scheme substring not at the start", "xhttp://ex.com", "xhttp://ex.com", false},
		{"empty url", "", "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := isBareAutolinkScheme(c.href, c.text); got != c.want {
				t.Errorf("isBareAutolinkScheme(%q, %q) = %v, want %v", c.href, c.text, got, c.want)
			}
		})
	}
}
