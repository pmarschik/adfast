// Package mediasrc holds the asset-store resolution every leg of the
// media projection shares: recovering the media id and the intrinsic
// dimensions that a PATH-ADDRESSED media directive leaves out.
//
// A downloaded attachment renders as `::media[alt]{path=assets/a.png}`,
// with no id and no width/height, because the asset store can recover
// all three from the path and repeating them in the document would only
// let them go stale. Recovering them is therefore not an option a leg
// may hold a private opinion about — it is how a path-addressed
// directive is read at all.
//
// It used to be spelled out at ONE call site: dialect's
// Media.EncodeADF, through the AssetID/AssetDims entry points. The
// md→md formatter's own copy of the projection looked the asset up by
// id and gave up when the directive carried only a path, so one and the
// same directive was a picture on one leg and an opaque directive on the
// other, with nothing in the API telling the caller which leg had run.
// The caption carrier (:::media, MediaCaption.EncodeADF) built its leaf
// through the same helper but without the recovery, so even on the ADF
// leg a captioned path-addressed picture encoded to a media node with an
// empty id — an attachment no Atlassian product can address.
//
// So the resolution lives here instead, in two functions both legs
// call. It is the sibling of mediaurl, which owns the other decision the
// legs share (whether an external URL may take the plain image form).
package mediasrc

// ID answers the media id a media directive names: the explicit id when
// it carries one, otherwise the id the asset store recovers from the
// path. Absent both, "" — nothing can address the attachment, and the
// caller reports the loss.
//
// resolve is the store lookup (nil when the caller configured none).
func ID(id, path string, resolve func(ref string) (mediaID string, ok bool)) string {
	if id != "" || path == "" || resolve == nil {
		return id
	}
	if resolved, ok := resolve(path); ok {
		return resolved
	}
	return id
}

// Dims answers the intrinsic pixel dimensions of the picture: the ones
// the directive spells, or the local file's own when it spells neither.
// A directive that carries one of the pair keeps what it has — half a
// size is the author's, not the store's, and re-deriving the other half
// would silently combine two sources.
//
// resolve measures the local file (nil when the caller configured none).
func Dims(width, height *float64, path string, resolve func(ref string) (w, h int, ok bool)) (outW, outH *float64) {
	if width != nil || height != nil || path == "" || resolve == nil {
		return width, height
	}
	w, h, ok := resolve(path)
	if !ok {
		return width, height
	}
	wf, hf := float64(w), float64(h)
	return &wf, &hf
}
