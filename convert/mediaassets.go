package convert

// A media id becomes an image only when the caller has the downloaded file
// behind it, and there are two ways to say which files those are: hand over the
// whole collection up front (WithMediaAssets), or answer one id at a time
// (WithMediaAssetResolver). Both end up here, as one lookup the conversion asks
// and the rest of the package need not distinguish.

import "github.com/pmarschik/adfast/extension"

// mediaAssetMap indexes the configured downloaded attachments
// (extension.MediaAsset, aliased as MediaAsset) by media id.
type mediaAssetMap map[string]extension.MediaAsset

// MediaAssetResolver answers, for a single media id, the downloaded local file
// that stands for it — the lazy form of WithMediaAssets' map.
//
// It exists because handing over a whole collection is not always harmless: an
// asset store shared by many documents can hold every file the caller ever
// downloaded, and producing an entry may cost something (a stat, a fetch, a
// symlink written next to the document being rendered). A resolver is asked only
// about the media a conversion actually meets. Returning false is the same
// answer as an absent map entry: that media keeps its ::media directive.
type MediaAssetResolver func(mediaID string) (MediaAsset, bool)

// mediaAssets is one conversion's view of both option forms: the eagerly
// configured map first, the resolver for whatever it does not cover.
type mediaAssets struct {
	byID    mediaAssetMap
	resolve MediaAssetResolver
	// memo remembers resolver answers, misses included. A conversion asks about
	// the same id more than once (normalization and the ADF→AST walk each look
	// media up), and a resolver is allowed to be expensive. Options are applied
	// per conversion, so the cache never outlives the document it was filled
	// for.
	memo map[string]mediaAssetAnswer
}

// mediaAssetAnswer is a resolver reply, held so a miss is cached as firmly as
// a hit.
type mediaAssetAnswer struct {
	asset MediaAsset
	held  bool
}

// newMediaAssets builds the lookup for one conversion.
func newMediaAssets(cfg config) mediaAssets {
	m := mediaAssets{byID: cfg.mediaAssets, resolve: cfg.resolveMediaAsset}
	if m.resolve != nil {
		m.memo = make(map[string]mediaAssetAnswer)
	}
	return m
}

// lookup reports the downloaded file recorded for a media id, if any.
func (m mediaAssets) lookup(id string) (MediaAsset, bool) {
	if a, ok := m.byID[id]; ok {
		return a, true
	}
	if m.resolve == nil || id == "" {
		return MediaAsset{}, false
	}
	if answer, seen := m.memo[id]; seen {
		return answer.asset, answer.held
	}
	a, ok := m.resolve(id)
	if ok {
		a = withDerivedDims(a)
	}
	m.memo[id] = mediaAssetAnswer{asset: a, held: ok}
	return a, ok
}

// withDerivedDims fills in HasDim for callers that never set it (the historical
// struct had no such field): a nonzero Width or Height implies known
// dimensions, both zero means not a parseable image. An asset that already has
// HasDim set is taken at its word.
func withDerivedDims(a MediaAsset) MediaAsset {
	if !a.HasDim && (a.Width != 0 || a.Height != 0) {
		a.HasDim = true
	}
	return a
}
