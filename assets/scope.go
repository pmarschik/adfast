package assets

import "github.com/pmarschik/adfast/convert"

// ForScope binds a store to one visibility scope — the product
// container media ids are valid in. Jira binds attachments to one
// issue and Confluence to one page: a media id minted for another
// container renders broken there, so cross-document deduplication must
// stop at the container boundary. Through a scoped view, Lookup only
// returns ids of this scope, Pending re-lists content that is only
// attached elsewhere, and new associations record the scope:
//
//	view := assets.ForScope(store, "PROJ-123")
//	docs, err := assets.PushPipeline(ctx, view, up).MarkdownToADFAll(mds)
//
// The view OVERRIDES the scope argument of every scope-taking method
// with the bound scope — it speaks for exactly one container, so a
// caller-supplied scope (including "") is ignored. This keeps generic
// plumbing like Sync and IDResolver, which pass scope "", correctly
// scoped when handed a view.
//
// A content-addressed store like FSStore still deduplicates storage
// globally (one blob, one friendly file); only the media ids are per
// scope. Encode every document with the view of ITS container.
func ForScope(store Store, scope string) Store {
	view := &scopedStore{inner: store, scope: scope}
	if meta, ok := MetaOf(store); ok {
		return &scopedMeta{scopedStore: view, Metadata: meta}
	}
	return view
}

type scopedStore struct {
	inner Store
	scope string
}

// Resolve implements Store (ids are globally unique — no scoping).
func (s *scopedStore) Resolve(mediaID string) (convert.MediaAsset, bool) {
	return s.inner.Resolve(mediaID)
}

// Lookup implements Store; the bound scope overrides the argument.
func (s *scopedStore) Lookup(_, path string) (string, bool) {
	return s.inner.Lookup(s.scope, path)
}

// Add implements Store, recording the id under the bound scope (the
// scope argument is overridden).
func (s *scopedStore) Add(_, mediaID, suggestedName string, content []byte) (convert.MediaAsset, error) {
	return s.inner.Add(s.scope, mediaID, suggestedName, content)
}

// Assets implements Store.
func (s *scopedStore) Assets() map[string]convert.MediaAsset {
	return s.inner.Assets()
}

// Pending implements Store, relative to the bound scope (the scope
// argument is overridden).
func (s *scopedStore) Pending(_ string) ([]string, error) {
	return s.inner.Pending(s.scope)
}

// Associate implements Store, recording the id under the bound scope
// (the scope argument is overridden).
func (s *scopedStore) Associate(_, mediaID, path string) (convert.MediaAsset, error) {
	return s.inner.Associate(s.scope, mediaID, path)
}

// Load implements Store.
func (s *scopedStore) Load(path string) ([]byte, error) {
	return s.inner.Load(path)
}

// Dims implements Store.
func (s *scopedStore) Dims(path string) (width, height int, ok bool) {
	return s.inner.Dims(path)
}

// scopedMeta is the view over a store that has metadata, so that MetaOf
// answers for a view exactly when it answers for what it wraps. Records
// are addressed by content, which no scope narrows, so every method is a
// pass-through: Put records no media id, and it is the id — not the
// content and not what the embedder knows about it — that belongs to one
// container.
type scopedMeta struct {
	*scopedStore
	Metadata
}
