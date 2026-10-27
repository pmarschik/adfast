package adfast

import (
	"strings"
	"testing"
)

// TestPunycodeHostSurvivesTheADFLeg is the ADF-leg counterpart of
// markdown.TestPunycodeLiteralHrefKeepsTheLabelAndEncodesTheHost, which pins
// punycodeLiteralHref end to end through Parse alone. The function it covers
// runs at parse time — convertGoldmarkAutoLink and splitURLLiterals in
// markdown/goldmark_to_ast.go are its only two callers, and both build the
// *ast.Link that every later stage, ADF included, just carries forward — so
// nothing specific to the ADF leg needed to change for this to already work.
// This file is the regression guard that confirms that: the ADF link mark's
// href attribute holds the IDNA form, the Markdown round trip
// (md → adf → md) is a fixpoint, and a second pass changes nothing further.
//
// THE ROUND TRIP IS NOT A NO-OP BYTE-FOR-BYTE, which is the one surprise
// worth pinning, though it is not new here: a bare "www." literal's own
// source spelling survives a render only through bareWWWLiteral (prettier
// mode) or autolinkText (every mode), and both require the label to equal
// the URL — already false for an all-ASCII host, since goldmark's own
// "http://" scheme never appears in the label (see emailautolink_test.go's
// "www literal" row, bracketed the same way with no punycode involved at
// all). A non-ASCII host fails the same test for a second, independent
// reason once its href is punycoded, so the render falls through to the
// ordinary "[label](url)" form either way — which is exactly the shape the
// reference's OWN ADF-round-trip leg renders this address as, not to be
// confused with its plain-markdown leg, which never punycodes at all (see
// punycodeLiteralHref's own doc in markdown/urlliteral.go for both measured
// rows side by side; the ADF-leg one is "see www.exämple.com here" ->
// "see [www.exämple.com](http://www.xn--exmple-cua.com) here\n").
func TestPunycodeHostSurvivesTheADFLeg(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		md   string
		href string
		want string
	}{
		{
			name: "bare www literal with one non-ASCII byte in the host",
			md:   "see www.exämple.com here\n",
			href: "http://www.xn--exmple-cua.com",
			want: "see [www.exämple.com](http://www.xn--exmple-cua.com) here\n",
		},
		{
			name: "schemed https://www. form of the same host",
			md:   "see https://www.exämple.com here\n",
			href: "https://www.xn--exmple-cua.com",
			want: "see [https://www.exämple.com](https://www.xn--exmple-cua.com) here\n",
		},
		{
			name: "a host whose middle segment is entirely non-ASCII",
			md:   "see www.例子公司.com here\n",
			href: "http://www.xn--fsqr9b55c21l.com",
			want: "see [www.例子公司.com](http://www.xn--fsqr9b55c21l.com) here\n",
		},
		{
			// Regression guard on the guard: an all-ASCII host must not pick
			// up any punycode mangling. The bracketed round trip here is
			// pre-existing and orthogonal to this fix — see
			// emailautolink_test.go's "www literal" row — because a bare
			// "www." literal always gains goldmark's "http://" scheme, which
			// the label itself never carries, and that alone already fails
			// autolinkText's text-equals-URL test. The row exists so a
			// broken punycode guard would show up here as a THIRD distinct
			// href, not as a silent pass.
			name: "an all-ASCII host is untouched",
			md:   "see www.example.com here\n",
			href: "http://www.example.com",
			want: "see [www.example.com](http://www.example.com) here\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := adfJSON(t, mdToADF(c.md)); !strings.Contains(got, `"href":"`+c.href+`"`) {
				t.Errorf("adf has no link mark with href %q:\n %s", c.href, got)
			}
			got := roundTrip(t, c.md)
			if got != c.want {
				t.Errorf("round trip = %q, want %q", got, c.want)
			}
			if twice := roundTrip(t, got); twice != got {
				t.Errorf("not idempotent:\n once:  %q\n twice: %q", got, twice)
			}
		})
	}
}
