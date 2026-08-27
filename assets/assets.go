// Package assets is a pluggable, content-addressed store for media
// attachments referenced by markdown documents, kept in an assets/ folder
// next to the files.
//
// The user-facing layout is free-form: markdown references plain friendly
// names (assets/shot.png) that users may also create themselves. Downloaded
// attachments are content-addressed under a hidden assets/.store/ directory
// (<sha256[:16]>.<ext>) with the friendly name symlinked to the store file
// (plain copy where symlinks are unavailable), deduplicating identical
// content. assets/.store/index.json holds one record per distinct piece of
// content, keyed by that content's hash and carrying the friendly name, the
// media ids pointing at it, and any metadata the embedder keeps beside it —
// the markdown itself carries no ids. See Metadata for the second half,
// and Catalog for reading the records back as an inventory.
//
// Reference paths are hardened at the store boundary: they must stay
// inside the assets folder, symlinked entries must resolve into the
// hidden store (planted symlinks pointing elsewhere are rejected for
// both reads and writes), index records are vetted on load, and reads
// are capped at MaxAssetSize.
//
// FSStore implements the Store interface; IDResolver and DimsResolver
// adapt a Store to the convert package's resolver hooks.
package assets

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"  // dimension probing
	_ "image/jpeg" // dimension probing
	_ "image/png"  // dimension probing
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	_ "golang.org/x/image/bmp"  // dimension probing
	_ "golang.org/x/image/tiff" // dimension probing
	_ "golang.org/x/image/webp" // dimension probing

	"github.com/pmarschik/adfast/convert"
)

// Dir is the assets folder name next to issue markdown files.
const Dir = "assets"

// storeDir is the hidden content-addressed store inside Dir.
const storeDir = ".store"

// indexFile holds the content-hash-keyed asset records — see index.
const indexFile = "index.json"

// MaxAssetSize is the largest asset file the store reads or hands to an
// uploader (100 MiB). Load refuses larger files with an error; Pending
// skips them.
const MaxAssetSize = 100 << 20

// ErrPathEscapes reports a reference path or asset name that would land
// outside the assets folder (path traversal or a symlink escape).
var ErrPathEscapes = errors.New("asset path escapes the assets dir")

// ErrAbsolutePath reports an absolute reference path — references must
// stay relative to the markdown directory.
var ErrAbsolutePath = errors.New("asset path is absolute")

// Store manages the assets folder for one markdown directory.
// Implementations are pluggable; NewFSStore is the filesystem one.
//
// Methods taking a scope parameter restrict themselves to one visibility
// scope — the product container a media id is valid in (a Jira issue
// key, a Confluence page id). Scope "" is the unscoped view: it matches
// records of every scope and records new ids without one. Use ForScope
// to bind a store to a single container.
type Store interface {
	// Resolve returns the render asset (markdown-relative path + image
	// dimensions) for a media id known to the store. Media ids are
	// globally unique, so resolution takes no scope.
	Resolve(mediaID string) (convert.MediaAsset, bool)
	// Lookup maps a referenced markdown-relative path back to its media
	// id; how the mapping is maintained is the implementation's
	// concern. Within a non-empty scope, an exactly-scoped id wins over
	// an unscoped (legacy) one; ids of foreign scopes never match.
	// Scope "" matches any record.
	Lookup(scope, path string) (string, bool)
	// Add stores attachment content under a friendly name derived from
	// suggestedName and records the media id under scope ("" for
	// unscoped). It returns the render asset.
	Add(scope, mediaID, suggestedName string, content []byte) (convert.MediaAsset, error)
	// Assets returns the full id → asset map for rendering.
	Assets() map[string]convert.MediaAsset
	// Pending returns the markdown-referenced assets that have no media
	// id yet — the upload worklist for assets added to markdown first.
	// With a non-empty scope, only ids recorded for that scope (or
	// unscoped legacy ids) satisfy an asset: one attached in another
	// product container still needs an upload here, because products
	// like Jira and Confluence bind attachments to one container.
	// (What "referenced" means physically, and any size or safety
	// limits, are the implementation's concern — see FSStore.)
	Pending(scope string) ([]string, error)
	// Associate binds an existing assets file (markdown-relative path)
	// to the media id an upload assigned, recorded under scope ("" for
	// unscoped). The friendly file stays in place; the store records it
	// like Add.
	Associate(scope, mediaID, path string) (convert.MediaAsset, error)
	// Load returns the content bytes for a reference path — what an
	// Uploader sends. Path validation and any size limits are the
	// implementation's concern (see FSStore).
	Load(path string) ([]byte, error)
	// Dims probes the intrinsic pixel dimensions of a referenced image
	// path; ok is false for unreadable files and non-image formats. How
	// reference paths map to physical files is the store's concern.
	Dims(path string) (width, height int, ok bool)
}

// FSStore is the filesystem Store for one assets folder. All markdown
// documents next to that folder share it — paths are folder-relative —
// and multiple FSStore instances over the same folder cooperate: every
// mutation reloads the on-disk index, merges, and writes it back
// atomically, so an association recorded through one instance is not
// lost by another. Method access is safe for concurrent use within a
// process — ALL FSStore instances over the same index share one
// process-level lock, so concurrent instances cannot interleave their
// reload-merge/save cycles and drop each other's entries. Cross-process
// writers are serialized only by the atomic index replacement (last
// write wins per full index write).
type FSStore struct {
	// records is the index keyed by content hash — see assetRecord.
	records      map[string]assetRecord
	mu           *sync.Mutex
	docDir       string
	assetsParent string
	blobParent   string
	refPrefix    string
	// storeSubdir is the content-addressed blob directory, relative to
	// blobParent. Defaults to "assets/.store"; override with WithStoreDir.
	storeSubdir string
}

// Option configures an FSStore at construction.
type Option func(*FSStore)

// WithStoreDir overrides the content-addressed blob directory (and its
// index.json), relative to the store's blob parent. The default is the hidden
// "assets/.store" nested in the friendly folder; pass e.g. ".asset-store" to
// keep the blobs in a dedicated top-level directory instead. The friendly
// assets/ folder (and therefore markdown reference paths) is unaffected.
func WithStoreDir(subdir string) Option {
	return func(s *FSStore) {
		if subdir != "" {
			s.storeSubdir = filepath.Clean(subdir)
		}
	}
}

// indexLocks maps a canonical index path to its process-wide lock.
var indexLocks sync.Map

// indexLock returns the shared per-index-path mutex.
func indexLock(indexPath string) *sync.Mutex {
	key := indexPath
	if abs, err := filepath.Abs(indexPath); err == nil {
		key = abs
	}
	mu, _ := indexLocks.LoadOrStore(key, &sync.Mutex{})
	lock, ok := mu.(*sync.Mutex)
	if !ok { // unreachable: the map only ever holds *sync.Mutex
		return &sync.Mutex{}
	}
	return lock
}

// NewFSStore opens (or initializes) the asset store for the assets/ folder
// next to the markdown files in mdDir.
func NewFSStore(mdDir string, opts ...Option) (*FSStore, error) {
	return NewFSStoreAt(mdDir, mdDir, opts...)
}

// NewFSStoreAt opens the asset store whose assets/ folder lives under
// assetsParent, for documents in docDir — reference paths in markdown
// are computed relative to docDir ("../../assets/shot.png" for a
// project-root store two levels up). Use it for shared placements: a
// repository-root assets folder (pair with DiscoverRoot), an XDG data
// directory, or any other location. Documents at different depths each
// construct their own (cheap) instance over the same folder; instances
// cooperate through the shared index.
func NewFSStoreAt(assetsParent, docDir string, opts ...Option) (*FSStore, error) {
	return openFSStore(assetsParent, assetsParent, docDir, opts...)
}

// NewFSStoreSplit separates the TRUE store from the nice one: the
// content-addressed blobs and the index live under blobParent (shared,
// deduplicated across every document and view), while the friendly
// files — and therefore the markdown reference paths — stay in the
// assets/ folder next to the documents in docDir. Resolve materializes
// a missing friendly file from the shared blobs, so a view that never
// downloaded an asset still renders it locally.
func NewFSStoreSplit(blobParent, docDir string, opts ...Option) (*FSStore, error) {
	return openFSStore(blobParent, docDir, docDir, opts...)
}

func openFSStore(blobParent, assetsParent, docDir string, opts ...Option) (*FSStore, error) {
	s := &FSStore{blobParent: blobParent, assetsParent: assetsParent, docDir: docDir, records: map[string]assetRecord{}, storeSubdir: filepath.Join(Dir, storeDir)}
	for _, opt := range opts {
		opt(s)
	}
	s.mu = indexLock(s.indexPath())
	rel, err := filepath.Rel(docDir, s.assetsDir())
	if err != nil {
		return nil, fmt.Errorf("assets folder %q is not reachable from %q: %w", s.assetsDir(), docDir, err)
	}
	s.refPrefix = filepath.ToSlash(rel)
	raw, err := os.ReadFile(s.indexPath())
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read asset index: %w", err)
	}
	var idx index
	if err := json.Unmarshal(raw, &idx); err != nil {
		return nil, fmt.Errorf("parse asset index: %w", err)
	}
	for hash, record := range idx.Assets {
		if validRecord(hash, record) {
			sortIDs(record.IDs)
			s.records[hash] = record
		}
	}
	return s, nil
}

func (s *FSStore) assetsDir() string { return filepath.Join(s.assetsParent, Dir) }

// blobDir is the content-addressed store directory (the truth).
func (s *FSStore) blobDir() string { return filepath.Join(s.blobParent, s.storeSubdir) }

// linkTarget is the symlink destination for a friendly file pointing at
// a store blob — relative to the friendly folder, so trees stay
// relocatable ("." + "/.store/<hash>.png" in the fused layout).
func (s *FSStore) linkTarget(storeName string) string {
	rel, err := filepath.Rel(s.assetsDir(), filepath.Join(s.blobDir(), storeName))
	if err != nil {
		return filepath.Join(s.blobDir(), storeName)
	}
	return rel
}

// refPath is the markdown reference path for a friendly asset name.
func (s *FSStore) refPath(name string) string { return s.refPrefix + "/" + name }

// refToFull resolves a markdown reference path against docDir and
// guarantees the result stays inside the assets folder — the boundary
// for every reference path arriving from documents. Legitimate ../
// segments (project-root stores) pass; escapes do not.
func (s *FSStore) refToFull(path string) (string, error) {
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("asset path %q: %w", path, ErrAbsolutePath)
	}
	full := filepath.Clean(filepath.Join(s.docDir, filepath.FromSlash(path)))
	if !strings.HasPrefix(full, filepath.Clean(s.assetsDir())+string(os.PathSeparator)) {
		return "", fmt.Errorf("asset path %q: %w", path, ErrPathEscapes)
	}
	return full, nil
}

// resolveReadable admits the physical file behind a path-validated
// reference: a regular file, or a symlink that resolves INSIDE the
// store's blob directory — the friendly links the store itself creates,
// including split layouts where the blob dir lives outside the assets
// folder. Anything else (notably a planted symlink pointing at a file
// elsewhere) is rejected, so reference paths can never exfiltrate
// content from outside the store. It returns the physical path to read
// plus its FileInfo (for size checks before reading).
func (s *FSStore) resolveReadable(full string) (string, fs.FileInfo, error) {
	fi, err := os.Lstat(full)
	if err != nil {
		return "", nil, err
	}
	if fi.Mode().IsRegular() {
		return full, fi, nil
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return "", nil, fmt.Errorf("asset %q is not a regular file", full)
	}
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", nil, fmt.Errorf("asset %q: %w", full, err)
	}
	blob, err := filepath.EvalSymlinks(s.blobDir())
	if err != nil {
		// No blob store — nothing a symlink could legitimately target.
		return "", nil, fmt.Errorf("asset %q: symlink without a store: %w", full, ErrPathEscapes)
	}
	if !strings.HasPrefix(resolved, blob+string(os.PathSeparator)) {
		return "", nil, fmt.Errorf("asset %q: symlink resolves outside the store: %w", full, ErrPathEscapes)
	}
	target, err := os.Stat(resolved)
	if err != nil {
		return "", nil, err
	}
	if !target.Mode().IsRegular() {
		return "", nil, fmt.Errorf("asset %q is not a regular file", full)
	}
	return resolved, target, nil
}

// readRef loads the bytes behind a reference path with full validation:
// containment (refToFull), symlink vetting (resolveReadable), and the
// MaxAssetSize cap.
func (s *FSStore) readRef(path string) ([]byte, error) {
	full, err := s.refToFull(path)
	if err != nil {
		return nil, err
	}
	resolved, fi, err := s.resolveReadable(full)
	if err != nil {
		return nil, err
	}
	if fi.Size() > MaxAssetSize {
		return nil, fmt.Errorf("asset %q is %d bytes, limit is %d", path, fi.Size(), int64(MaxAssetSize))
	}
	return os.ReadFile(resolved)
}

// DiscoverRoot walks upward from dir and returns the first directory
// containing any of the anchor entries (".git", "go.work", a marker
// file, ...) — pair with NewFSStoreAt for a project-root assets folder
// shared by documents in nested directories:
//
//	root, ok := assets.DiscoverRoot(docDir, ".git")
//	store, err := assets.NewFSStoreAt(root, docDir)
func DiscoverRoot(dir string, anchors ...string) (string, bool) {
	dir = filepath.Clean(dir)
	for {
		for _, a := range anchors {
			if _, err := os.Lstat(filepath.Join(dir, a)); err == nil {
				return dir, true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// securePath joins name onto the assets dir (or the blob store) and
// guarantees the result stays inside it — the single boundary where
// externally influenced names touch the filesystem.
func (s *FSStore) securePath(inStore bool, name string) (string, error) {
	base := s.assetsDir()
	if inStore {
		base = s.blobDir()
	}
	p := filepath.Join(base, filepath.Base(name))
	if !strings.HasPrefix(p, filepath.Clean(base)+string(os.PathSeparator)) {
		return "", fmt.Errorf("asset name %q: %w", name, ErrPathEscapes)
	}
	return p, nil
}

func (s *FSStore) indexPath() string {
	return filepath.Join(s.blobDir(), indexFile)
}

// Resolve implements Store.
func (s *FSStore) Resolve(mediaID string) (convert.MediaAsset, bool) {
	s.mu.Lock()
	s.reloadIndex()
	hash, record, ok := s.recordFor(mediaID)
	s.mu.Unlock()
	if !ok {
		return convert.MediaAsset{}, false
	}
	return s.assetOf(hash, record.Name)
}

// recordFor finds the record a media id points at. Caller holds the lock.
func (s *FSStore) recordFor(mediaID string) (string, assetRecord, bool) {
	key := idKey(mediaID)
	for hash, record := range s.records {
		for _, ref := range record.IDs {
			if idKey(ref.ID) == key {
				return hash, record, true
			}
		}
	}
	return "", assetRecord{}, false
}

// assetOf is the render asset for a recorded blob, materializing the
// friendly file from the blob store when this folder has none yet.
func (s *FSStore) assetOf(hash, name string) (convert.MediaAsset, bool) {
	full, err := s.securePath(false, name)
	if err != nil {
		return convert.MediaAsset{}, false
	}
	if _, err := os.Stat(full); err != nil {
		if !s.materialize(hash, name) {
			return convert.MediaAsset{}, false
		}
	}
	asset := convert.MediaAsset{Path: s.refPath(name)}
	if w, h, ok := s.dimsAt(full); ok {
		asset.Width, asset.Height = w, h
	}
	return asset, true
}

// Lookup implements Store. FSStore maps the path by content: the
// referenced file's bytes identify the record, so renames of the
// friendly file do not break the mapping. Within a scope, an
// exactly-scoped id wins over an unscoped (legacy) one; ids of foreign
// scopes never match.
func (s *FSStore) Lookup(scope, path string) (string, bool) {
	content, err := s.readRef(path)
	if err != nil {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIndex()
	record, ok := s.records[hashContent(content)]
	if !ok {
		return "", false
	}
	return record.scopeFor(scope)
}

// reloadIndex merges the on-disk index into the in-memory map so
// mutations through other instances of the same folder are not lost.
// Records are vetted like at open time — a crafted index never
// contributes a path-steering entry. See assetRecord.mergeFrom for which
// half of a record each side wins.
func (s *FSStore) reloadIndex() {
	raw, err := os.ReadFile(s.indexPath())
	if err != nil {
		return
	}
	var idx index
	if json.Unmarshal(raw, &idx) != nil {
		return
	}
	for hash, disk := range idx.Assets {
		if !validRecord(hash, disk) {
			continue
		}
		ours, mine := s.records[hash]
		if !mine {
			sortIDs(disk.IDs)
			s.records[hash] = disk
			continue
		}
		s.records[hash] = ours.mergeFrom(disk)
	}
}

// Add implements Store. FSStore content-addresses the blob store, so
// identical content deduplicates: it lands in one blob and reuses an
// existing friendly file, no matter how many media ids reference it.
func (s *FSStore) Add(scope, mediaID, suggestedName string, content []byte) (convert.MediaAsset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIndex()
	hash, name, err := s.store(suggestedName, content)
	if err != nil {
		return convert.MediaAsset{}, err
	}
	s.records[hash] = s.records[hash].withID(mediaID, scope)
	if err := s.saveIndex(); err != nil {
		return convert.MediaAsset{}, err
	}
	return s.mustAsset(name), nil
}

// store writes content to its blob, makes sure a friendly name reaches
// it, and records the pairing — everything an asset needs except a media
// id. It returns the content hash the record is keyed by and the friendly
// name the markdown references. The caller holds the lock and saves.
func (s *FSStore) store(suggestedName string, content []byte) (hash, name string, err error) {
	if mkErr := os.MkdirAll(s.blobDir(), 0o750); mkErr != nil {
		return "", "", fmt.Errorf("create asset store: %w", mkErr)
	}
	if mkErr := os.MkdirAll(s.assetsDir(), 0o750); mkErr != nil {
		return "", "", fmt.Errorf("create assets dir: %w", mkErr)
	}
	hash = hashContent(content)
	storeName := hash + filepath.Ext(sanitizeName(suggestedName))
	storePath, err := s.securePath(true, storeName)
	if err != nil {
		return "", "", err
	}
	if _, statErr := os.Stat(storePath); errors.Is(statErr, fs.ErrNotExist) {
		if writeErr := os.WriteFile(storePath, content, 0o600); writeErr != nil {
			return "", "", fmt.Errorf("write asset content: %w", writeErr)
		}
	}

	// Reuse the friendly name that already references this content.
	record, known := s.records[hash]
	if known && s.friendlyMatches(record.Name, storeName) {
		return hash, record.Name, nil
	}
	name, err = s.createFriendly(sanitizeName(suggestedName), storeName, content)
	if err != nil {
		return "", "", err
	}
	record.Name = name
	s.records[hash] = record
	return hash, name, nil
}

// materialize creates the friendly file for an index record from the
// blob store — the self-healing path of a split store, where another
// view recorded the asset and this document's folder has no friendly
// file yet. It only ever CREATES: when anything already occupies the
// friendly path (a regular file, a dangling or planted symlink), it
// refuses — writing there would follow the symlink to an arbitrary
// destination.
func (s *FSStore) materialize(hash, name string) bool {
	storeName := hash + filepath.Ext(name)
	blob, err := s.securePath(true, storeName)
	if err != nil {
		return false
	}
	if _, statErr := os.Stat(blob); statErr != nil {
		return false
	}
	if mkErr := os.MkdirAll(s.assetsDir(), 0o750); mkErr != nil {
		return false
	}
	full, err := s.securePath(false, name)
	if err != nil {
		return false
	}
	if _, lstatErr := os.Lstat(full); lstatErr == nil {
		return false // occupied — never write over or through it
	}
	if linkErr := os.Symlink(s.linkTarget(storeName), full); linkErr != nil {
		content, readErr := os.ReadFile(blob)
		if readErr != nil {
			return false
		}
		if writeExclusive(full, content) != nil {
			return false
		}
	}
	return true
}

// writeExclusive creates a brand-new file: O_EXCL guarantees the write
// cannot follow a symlink planted at the path between check and write.
func writeExclusive(path string, content []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, werr := f.Write(content)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	return werr
}

// friendlyMatches reports whether assets/<name> currently resolves to the
// given store file.
func (s *FSStore) friendlyMatches(name, storeName string) bool {
	full, err := s.securePath(false, name)
	if err != nil {
		return false
	}
	target, err := os.Readlink(full)
	return err == nil && target == s.linkTarget(storeName)
}

// createFriendly links a fresh friendly name (suffixing -2, -3, … on
// conflicts with different content) to the store file.
func (s *FSStore) createFriendly(base, storeName string, content []byte) (string, error) {
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if stem == "" {
		stem = "attachment"
	}
	target := s.linkTarget(storeName)
	for i := 1; ; i++ {
		name := base
		if i > 1 {
			name = fmt.Sprintf("%s-%d%s", stem, i, ext)
		}
		full, pathErr := s.securePath(false, name)
		if pathErr != nil {
			return "", pathErr
		}
		if existing, err := os.Readlink(full); err == nil && existing == target {
			return name, nil
		}
		if fi, err := os.Lstat(full); err == nil {
			// A regular file with identical content is adopted as the
			// friendly name — the markdown-first flow: the user drops a
			// file into assets/ and references it before any upload; Add
			// runs after the upload with the assigned media id and must
			// not duplicate the file under a suffixed name. The user's
			// plain file stays in place (Resolve/Lookup work on plain
			// files too).
			if fi.Mode().IsRegular() {
				if existing, readErr := os.ReadFile(full); readErr == nil && hashContent(existing) == hashContent(content) {
					return name, nil
				}
			}
			continue // occupied by different content — try the next suffix
		}
		if err := os.Symlink(target, full); err != nil {
			// Fall back to a plain copy (e.g. no symlink support) —
			// exclusive creation, so nothing planted meanwhile is
			// followed or overwritten.
			if err := writeExclusive(full, content); err != nil {
				return "", fmt.Errorf("write asset reference: %w", err)
			}
		}
		return name, nil
	}
}

func (s *FSStore) mustAsset(name string) convert.MediaAsset {
	asset := convert.MediaAsset{Path: s.refPath(name)}
	if full, err := s.securePath(false, name); err == nil {
		if w, h, ok := s.dimsAt(full); ok {
			asset.Width, asset.Height = w, h
		}
	}
	return asset
}

// Pending implements Store: it walks the assets folder (excluding the
// hidden store) and returns, in sorted order, the markdown-relative
// paths of files whose content is not associated with any media id.
// With a non-empty scope, only ids recorded for that scope (or unscoped
// legacy ids) satisfy a file — content attached in another product
// container still needs an upload here. Planted symlinks and files over
// MaxAssetSize are skipped.
func (s *FSStore) Pending(scope string) ([]string, error) {
	s.mu.Lock()
	s.reloadIndex()
	known := make(map[string]bool, len(s.records))
	for hash, record := range s.records {
		if _, ok := record.scopeFor(scope); ok {
			known[hash] = true
		}
	}
	s.mu.Unlock()
	entries, err := os.ReadDir(s.assetsDir())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read assets dir: %w", err)
	}
	var pending []string
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		resolved, fi, resErr := s.resolveReadable(filepath.Join(s.assetsDir(), e.Name()))
		if resErr != nil || fi.Size() > MaxAssetSize {
			continue // planted symlink, unreadable, or oversized — not uploadable
		}
		content, readErr := os.ReadFile(resolved)
		if readErr != nil {
			continue
		}
		if !known[hashContent(content)] {
			pending = append(pending, s.refPath(e.Name()))
		}
	}
	slices.Sort(pending)
	return pending, nil
}

// Associate implements Store: the ergonomic counterpart of Add for the
// markdown-first flow — the file already exists under assets/, the
// upload just assigned its media id.
//
//	for _, p := range store.Pending("") { store.Associate("", upload(p), p) }
func (s *FSStore) Associate(scope, mediaID, path string) (convert.MediaAsset, error) {
	content, err := s.readRef(path)
	if err != nil {
		return convert.MediaAsset{}, fmt.Errorf("associate: %w", err)
	}
	return s.Add(scope, mediaID, filepath.Base(filepath.FromSlash(path)), content)
}

// Assets implements Store.
func (s *FSStore) Assets() map[string]convert.MediaAsset {
	s.mu.Lock()
	s.reloadIndex()
	type named struct{ hash, name string }
	held := make(map[string]named, len(s.records))
	for hash, record := range s.records {
		for _, ref := range record.IDs {
			held[idKey(ref.ID)] = named{hash: hash, name: record.Name}
		}
	}
	s.mu.Unlock()
	out := map[string]convert.MediaAsset{}
	for id, n := range held {
		if asset, ok := s.assetOf(n.hash, n.name); ok {
			out[id] = asset
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// saveIndex writes index.json deterministically (sorted keys, two-space
// indent, trailing newline) so independent writers produce identical
// bytes. NOTE: the deterministic byte output relies on encoding/json
// (v1) sorting map keys — revisit if this ever migrates to
// encoding/json/v2, which preserves insertion order instead.
func (s *FSStore) saveIndex() error {
	raw, err := json.MarshalIndent(index{Assets: s.records}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode asset index: %w", err)
	}
	// Atomic replacement through an unpredictably named temp file in
	// the same directory: concurrent instances never observe a torn
	// index, the last full write wins, and nobody can pre-plant the
	// temp path.
	f, err := os.CreateTemp(filepath.Dir(s.indexPath()), "index-*.json.tmp")
	if err != nil {
		return fmt.Errorf("write asset index: %w", err)
	}
	tmp := f.Name()
	_, werr := f.Write(append(raw, '\n'))
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write asset index: %w", werr)
	}
	if err := os.Rename(tmp, s.indexPath()); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace asset index: %w", err)
	}
	return nil
}

// sanitizeName strips path separators and control characters from an
// attachment filename supplied by the host product.
func sanitizeName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return '_'
		}
		return r
	}, name)
	if name == "" || name == "." || name == "/" {
		return "attachment"
	}
	return name
}

// hashContent returns the 16-hex-digit sha256 prefix used for store
// filenames. ContentHash is its exported face.
func hashContent(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])[:16]
}

// ImageDims probes the intrinsic pixel dimensions of an image file;
// ok is false for unreadable files and non-image formats.
func ImageDims(path string) (width, height int, ok bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, false
	}
	defer func() { _ = f.Close() }()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}

// dimsAt probes image dimensions behind an assets-dir path with the
// same symlink vetting as content reads.
func (s *FSStore) dimsAt(full string) (width, height int, ok bool) {
	resolved, fi, err := s.resolveReadable(full)
	if err != nil || fi.Size() > MaxAssetSize {
		return 0, 0, false
	}
	return ImageDims(resolved)
}

// Dims implements Store: it probes the dimensions of a referenced image
// path (validated like every reference path).
func (s *FSStore) Dims(path string) (width, height int, ok bool) {
	full, err := s.refToFull(path)
	if err != nil {
		return 0, 0, false
	}
	return s.dimsAt(full)
}

// IDResolver returns an convert.AssetIDResolver backed by the store.
// The unscoped lookup is correct for scoped views too: ForScope
// overrides the scope argument with its bound scope.
func IDResolver(s Store) convert.AssetIDResolver {
	return func(path string) (string, bool) {
		return s.Lookup("", path)
	}
}
