package assets

import (
	"cmp"
	"encoding/json"
	"regexp"
	"slices"
	"strings"
)

// index is the on-disk shape of index.json: one record per distinct piece
// of content, keyed by the content hash that names its blob.
//
// The hash is the key rather than a field because it is the only thing
// about an asset that is true before anything else is. A media id is
// assigned by an upload, a friendly name is chosen when the file is first
// written, and both can be plural — one blob can carry an id per product
// container and be reached under a name per folder. What identifies the
// asset through all of that is its content, so that is what the record
// hangs on, and two documents that produce byte-identical content share
// one record without either of them arranging it.
type index struct {
	Assets map[string]assetRecord `json:"assets"`
}

// assetRecord is one content-addressed asset in index.json.
//
// Meta is the embedder's half of the record: a namespace per concern,
// each holding whatever JSON that concern wants to keep about the
// content. The store neither reads nor validates it — it is carried,
// merged and written back verbatim — so a host that generates its assets
// (a diagram renderer, a chart pipeline) can record what drew them
// beside the asset itself instead of in a manifest of its own.
type assetRecord struct {
	// Name is the friendly filename the assets folder reaches the blob
	// under, without any directory part.
	Name string `json:"name"`
	// Meta is embedder metadata, one entry per namespace.
	Meta map[string]json.RawMessage `json:"meta,omitempty"`
	// IDs are the media ids that point at this content, one per
	// visibility scope the content was uploaded into. Sorted, so the
	// file is byte-stable across writers.
	IDs []mediaRef `json:"ids,omitempty"`
}

// mediaRef is one media id pointing at a record's content. Scope is the
// visibility container the id is valid in (a Jira issue key, a
// Confluence page id); empty for an unscoped record.
type mediaRef struct {
	ID    string `json:"id"`
	Scope string `json:"scope,omitempty"`
}

// storeHashRe is the shape of a content hash in the index — the
// 16-hex-digit sha256 prefix contentHash produces.
var storeHashRe = regexp.MustCompile(`^[0-9a-f]{16}$`)

// validRecord vets a record loaded from disk: the key must have the
// store's hash shape and the friendly name must be a plain filename (no
// separators, no traversal segments, exactly what sanitizeName would
// produce). index.json is data, not trusted input — a crafted record
// must not steer any filesystem path. Meta is not vetted because it
// never reaches the filesystem; it is the embedder's to interpret.
func validRecord(hash string, r assetRecord) bool {
	if !storeHashRe.MatchString(hash) {
		return false
	}
	n := r.Name
	if n == "" || strings.ContainsAny(n, `/\`) || strings.Contains(n, "..") {
		return false
	}
	return n == sanitizeName(n)
}

// idKey is how a media id is matched: ids are case-insensitive, and the
// index is written in whatever case the product handed over.
func idKey(id string) string { return strings.ToLower(id) }

// scopeFor returns the id this record offers within scope, preferring an
// exactly-scoped one over an unscoped (legacy) record. Ids of foreign
// scopes never match; scope "" matches any id.
func (r assetRecord) scopeFor(scope string) (string, bool) {
	legacy, legacyOK := "", false
	for _, ref := range r.IDs {
		switch {
		case scope == "" || ref.Scope == scope:
			return ref.ID, true
		case ref.Scope == "":
			legacy, legacyOK = ref.ID, true
		}
	}
	return legacy, legacyOK
}

// withID returns the record with mediaID recorded under scope, replacing
// any previous entry for the same id. The result stays sorted.
func (r assetRecord) withID(mediaID, scope string) assetRecord {
	key := idKey(mediaID)
	r.IDs = slices.DeleteFunc(slices.Clone(r.IDs), func(ref mediaRef) bool { return idKey(ref.ID) == key })
	r.IDs = append(r.IDs, mediaRef{ID: mediaID, Scope: scope})
	sortIDs(r.IDs)
	return r
}

// sortIDs orders a record's ids so independent writers produce identical
// bytes.
func sortIDs(ids []mediaRef) {
	slices.SortFunc(ids, func(a, b mediaRef) int {
		return cmp.Or(strings.Compare(idKey(a.ID), idKey(b.ID)), strings.Compare(a.Scope, b.Scope))
	})
}

// mergeFrom folds a record read from disk into one already in memory.
// In-memory wins — it is either what this process just recorded or what
// it validated at open time — and the disk copy fills the gaps: ids
// another writer added, metadata namespaces this process does not use.
//
// Filling rather than replacing is what makes deletion work without
// tombstones: every mutation reloads, applies, and saves the whole map
// under the shared index lock, so a namespace this process removed is
// already gone from disk by the time anything reloads.
func (r assetRecord) mergeFrom(disk assetRecord) assetRecord {
	for _, ref := range disk.IDs {
		if !slices.ContainsFunc(r.IDs, func(ours mediaRef) bool { return idKey(ours.ID) == idKey(ref.ID) }) {
			r.IDs = append(r.IDs, ref)
		}
	}
	sortIDs(r.IDs)
	for ns, raw := range disk.Meta {
		if _, ours := r.Meta[ns]; ours {
			continue
		}
		if r.Meta == nil {
			r.Meta = map[string]json.RawMessage{}
		}
		r.Meta[ns] = raw
	}
	return r
}
