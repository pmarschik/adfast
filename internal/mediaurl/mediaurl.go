// Package mediaurl holds the one URL decision every leg of the media
// projection shares: whether an external media reference may take the
// plain ![alt](url) markdown image form.
//
// The projection is implemented once per leg — dialect's decode hooks
// (adf→md), convert's normalizer (md→md) and convert's encode
// (md→adf) — and each leg used to spell the decision out for itself
// with its own strings.HasPrefix pair. That doubling is how the two
// legs drifted apart: WithPreserveLocalImages was threaded into the
// decode copy and the format copy went on hardcoding an absolute-http
// test, so `::media[alt]{type=external url=img/a.png}` under the option
// stayed a directive on the format leg and became `![alt](img/a.png)`
// on the ADF leg, with nothing in the API telling the caller which leg
// had run.
//
// So the decision lives here instead, in one function all the legs
// call. A leg may still decline an image for its own reasons (a border,
// a display width, a layout the image cannot hold); what it may not do
// is hold a private opinion about the URL.
package mediaurl

import "strings"

// AbsoluteHTTP reports whether ref is an absolute http(s) URL: a
// picture any reader of the document can fetch, wherever the markdown
// file itself lives.
func AbsoluteHTTP(ref string) bool {
	return strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://")
}

// ProjectsToImage reports whether an external media URL may take the
// plain ![alt](url) markdown image form.
//
// An absolute http(s) URL always may — that is the ordinary embedded
// image, and ADF's external media holds exactly what ![alt](url) does.
//
// A document-relative path may only under WithPreserveLocalImages
// (preserveLocal), because resolving it needs the markdown file's own
// directory: it is a local image not yet uploaded, kept in image form
// so a round-trip and a later push upload can still see the path.
// Without the opt-in it stays a ::media directive, which is the
// default precisely so a relative external URL survives re-encode
// losslessly.
//
// An empty URL never does: there is no picture to point at, and
// external media with no url is not expressible as an image at all.
func ProjectsToImage(ref string, preserveLocal bool) bool {
	if ref == "" {
		return false
	}
	return AbsoluteHTTP(ref) || preserveLocal
}
