package mediaurl

import "testing"

// TestProjectsToImage pins the shared decision every leg of the media
// projection now asks. The preserveLocal=false column is the behavior
// each leg's own hardcoded prefix test used to produce, so it is a
// preserved-behavior PIN; the relative-path row under preserveLocal=true
// is the rule the format leg was missing.
func TestProjectsToImage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ref           string
		why           string
		preserveLocal bool
		want          bool
	}{
		{"https://x/a.png", "an absolute https url is the ordinary embedded image", false, true},
		{"http://x/a.png", "http counts too", false, true},
		{"https://x/a.png", "the opt-in does not disturb an absolute url", true, true},
		{"img/a.png", "a relative path stays ::media by default, so re-encode is lossless", false, false},
		{"img/a.png", "and reaches image form under WithPreserveLocalImages", true, true},
		{"/abs/a.png", "a root-relative path is local too", true, true},
		{"", "no url, no picture to point at", false, false},
		{"", "and the opt-in must not invent one", true, false},
	}
	for _, c := range cases {
		if got := ProjectsToImage(c.ref, c.preserveLocal); got != c.want {
			t.Errorf("ProjectsToImage(%q, %v) = %v, want %v: %s",
				c.ref, c.preserveLocal, got, c.want, c.why)
		}
	}
}

func TestAbsoluteHTTP(t *testing.T) {
	t.Parallel()
	for ref, want := range map[string]bool{
		"http://x":  true,
		"https://x": true,
		"HTTPS://x": false, // the ADF and markdown urls this sees are not case-folded
		"ftp://x":   false,
		"img/a.png": false,
		"":          false,
	} {
		if got := AbsoluteHTTP(ref); got != want {
			t.Errorf("AbsoluteHTTP(%q) = %v, want %v", ref, got, want)
		}
	}
}
