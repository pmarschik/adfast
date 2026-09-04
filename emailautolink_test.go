package adfast

import (
	"strings"
	"testing"
)

// TestEmailAutolinkCarriesItsScheme pins the mailto: scheme onto an email
// autolink's href. goldmark leaves URL() schemeless for an email autolink
// because its own HTML renderer prepends the scheme at render time, so a
// consumer reading the tree used to get a bare address as a destination:
// a RELATIVE link once it reaches ADF, and a relative-path link once the
// document round trips — a silent change of meaning, not a formatting
// difference.
//
// The table pairs each autolink form with the neighboring forms that must
// NOT gain a scheme, so a fix that reaches too far fails here too: an
// explicit [label](addr) destination is whatever the author wrote, and a
// www literal already carries the http:// goldmark put there.
func TestEmailAutolinkCarriesItsScheme(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		md   string
		href string // the destination expected in the ADF link mark
		want string // the markdown the document renders back to
	}{
		{
			// The form a person actually writes in prose.
			name: "bare email literal",
			md:   "Write to support@ixopay.com about it.\n",
			href: "mailto:support@ixopay.com",
			want: "Write to support@ixopay.com about it.\n",
		},
		{
			// The angle form is a CommonMark email autolink, and it is not
			// subject to the GFM literal validity gate.
			name: "angle email autolink",
			md:   "Write to <support@ixopay.com> about it.\n",
			href: "mailto:support@ixopay.com",
			want: "Write to <support@ixopay.com> about it.\n",
		},
		{
			// No dot in the domain, so CommonMark still linkifies the angle
			// form even though the GFM literal would be rejected.
			name: "angle autolink with a dotless domain",
			md:   "<foo@bar1>\n",
			href: "mailto:foo@bar1",
			want: "<foo@bar1>\n",
		},
		{
			// The linkify pass takes an address whose domain label opens
			// on '_', but the angle parser stops short of it, so the
			// autolink form would not read back. The explicit form is what
			// keeps the render a fixpoint. (Found by the round-trip
			// fuzzer; remark stringifies this one as <addr> and then gains
			// a bracket pair on every further render.)
			name: "domain label opening on an underscore",
			md:   "00@0._AA\n",
			href: "mailto:00@0._AA",
			want: "[00@0.\\_AA](mailto:00@0._AA)\n",
		},
		{
			// GOOD case: an explicit destination is already a mailto: URL
			// and must not be prefixed twice.
			name: "explicit mailto link",
			md:   "[support](mailto:support@ixopay.com)\n",
			href: "mailto:support@ixopay.com",
			want: "[support](mailto:support@ixopay.com)\n",
		},
		{
			// GOOD case: an explicit destination is whatever the author
			// wrote. This one really is a relative path.
			name: "explicit link to a bare address",
			md:   "[x](support@ixopay.com)\n",
			href: "support@ixopay.com",
			want: "[x](support@ixopay.com)\n",
		},
		{
			// GOOD case: the www literal's scheme comes from goldmark and
			// is untouched, label and all.
			name: "www literal",
			md:   "Visit www.example.com now\n",
			href: "http://www.example.com",
			want: "Visit [www.example.com](http://www.example.com) now\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := marshalADF(t, tt.md); !strings.Contains(got, `"href":"`+tt.href+`"`) {
				t.Errorf("adf has no link mark with href %q:\n %s", tt.href, got)
			}
			got := ToMarkdown(FromMarkdown(tt.md))
			if got != tt.want {
				t.Errorf("round trip = %q, want %q", got, tt.want)
			}
			if twice := ToMarkdown(FromMarkdown(got)); twice != got {
				t.Errorf("not idempotent:\n once:  %q\n twice: %q", got, twice)
			}
		})
	}
}
