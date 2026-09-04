package assets

import "slices"

// PendingRefs is the upload worklist for ONE document: the references
// that document actually makes, narrowed to the ones the store can read
// and holds no media id for in scope.
//
// It answers the same question as Store.Pending from the other side.
// Pending starts at the assets folder and lists what is in it; PendingRefs
// starts at the document and asks the store about each path it names. The
// two agree wherever a reference points into a folder the store lists, and
// they part where a store can READ further than it LISTS. A composed store
// may deliberately keep a folder out of its worklist — so that a push never
// scans a project for files no document mentions — and still answer for a
// path a document spells out. Only the document's own references can widen
// that, which is why they are the input here.
//
// refs are markdown-relative destinations as the document spells them
// ("./assets/x.png"). They are reduced to one spelling per file,
// deduplicated, and returned sorted, so the batch an uploader receives does
// not depend on the order the document happened to name them in. Remote and
// empty destinations are ignored: there is no file to send.
//
// A reference the store cannot read is left out rather than reported. It is
// a picture with no file behind it — something for the caller to tell its
// author about, not something an upload can fix — and the same exclusion
// keeps an oversized file or a planted symlink out of a batch, because the
// store refuses to load those too.
//
// scope is the product container the media ids belong to; pass "" through a
// view from ForScope, which overrides it with the container it is bound to.
func PendingRefs(store Store, scope string, refs []string) []string {
	seen := make(map[string]bool, len(refs))
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref == "" || isRemoteURL(ref) {
			continue
		}
		path := normalizeRef(ref)
		if seen[path] {
			continue
		}
		seen[path] = true
		if _, err := store.Load(path); err != nil {
			continue
		}
		if _, held := store.Lookup(scope, path); held {
			continue
		}
		out = append(out, path)
	}
	slices.Sort(out)
	return out
}
