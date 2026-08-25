package confluence

import (
	"encoding/xml"
	"regexp"
	"strings"

	"github.com/pmarschik/adfast/adf"
)

// Read-back repair: Confluence stores a page in its own storage format,
// so reading the page back as ADF is a conversion, and the conversion
// loses information that the submitted document carried.
//
// Two losses were measured on a live page (read-only, 2026-08-21), by
// comparing ?body-format=atlas_doc_format against ?body-format=storage
// of the same version:
//
//   - The code mark on link text is dropped. Storage holds
//     <code><a href="X">errors.Is</a></code>; the ADF read gives a text
//     node with the link mark and no code mark.
//   - An internal page link loses its title slug. Storage holds
//     <ac:link><ri:page ri:content-title="BIN Lookup Knowledge Base"/>
//     <ac:link-body>…</ac:link-body></ac:link>; the ADF read gives
//     ".../wiki/spaces/PT/pages/443514894", without the "/BIN+Lookup+
//     Knowledge+Base" the submitted href had.
//
// Both losses make a comparison between the local document and the
// remote one report a difference that nobody made, so a tool that
// diffs a page against its own last push never reaches a clean state.
//
// The storage body holds what the ADF read dropped, and both bodies
// describe the same page version, so RepairReadBack takes the storage
// body as the oracle. It is a repair and not a conversion: reading the
// page as storage and converting that with Confluence's own
// /contentbody/convert endpoint fixes the code mark, but replaces an
// authored anchor link with an
// __confluenceADFMigrationUnsupportedContentInternalExtension__
// placeholder, which loses more than it repairs.
//
// The page title is read from ri:content-title and NOT from the
// __confluenceMetadata attribute of the link mark: no link mark of the
// measured page carries that attribute.

// pageSlugRe matches the tail of a Confluence page URL that has no
// title slug: the page id, then the end of the href or the start of a
// query or fragment. An href that already carries a slug does not
// match, so a repaired href is never repaired twice.
var pageSlugRe = regexp.MustCompile(`/wiki/spaces/[^/]+/pages/\d+([?#].*)?$`)

// storageAutoClose is xml.HTMLAutoClose without "link". A storage body
// is XHTML, so a void element already closes itself, but the tolerant
// decoder matches its auto-close list on the local name alone: it would
// close <ac:link> at once, and then reject the real </ac:link>.
var storageAutoClose = []string{
	"area", "base", "br", "col", "hr", "img", "input", "meta", "param",
}

// RepairReadBack returns doc with the losses of Confluence's ADF page
// read repaired from storage, which must be the storage-format body of
// the same page version. It restores the code mark on link text and the
// title slug of an internal page link (see the comment above).
//
// The repair is additive: it adds a mark and it lengthens an href, and
// it removes nothing. A storage body that is empty, unparsable, or
// that holds no link returns doc unchanged.
//
// Call it on the document that the read returned, before FromADF, so
// that everything downstream sees the faithful page:
//
//	doc = confluence.RepairReadBack(doc, storageBody)
//	md, err := adfast.ToMarkdown(adfast.FromADF(doc, opts...), opts...)
//
// It is not one of the transforms that MarkdownOptions installs,
// because the storage body is per-page data and not an option.
func RepairReadBack(doc adf.Doc, storage string) adf.Doc {
	if storage == "" {
		return doc
	}
	stored := parseStorageLinks(storage)
	links := linkedText(doc)
	if len(stored) == 0 || len(links) == 0 {
		return doc
	}

	// One pending copy per node, so that a node needing both repairs
	// keeps both.
	fixes := map[*adf.Text]*adf.Text{}
	edit := func(t *adf.Text) *adf.Text {
		if pending, ok := fixes[t]; ok {
			return pending
		}
		pending := *t
		fixes[t] = &pending
		return &pending
	}

	restoreCodeMarks(stored, links, edit)
	restorePageSlugs(stored, links, edit)

	if len(fixes) == 0 {
		return doc
	}
	return adf.Transform(doc, func(n adf.Node) ([]adf.Node, bool) {
		t, ok := n.(*adf.Text)
		if !ok {
			return nil, false
		}
		if pending, ok := fixes[t]; ok {
			return []adf.Node{pending}, true
		}
		return nil, false
	})
}

// restoreCodeMarks adds the code mark to link text that storage wraps
// in <code>. It is skipped for text that already carries a mark the
// code mark excludes in ADF (strong, em, strike): storage cannot have
// produced that combination, so the alignment is wrong there.
func restoreCodeMarks(stored []storageLink, links []linkText, edit func(*adf.Text) *adf.Text) {
	align(stored,
		func(s storageLink) (string, bool) { return s.href + "\x00" + s.text, s.href != "" },
		links,
		func(l linkText) (string, bool) { return l.href + "\x00" + l.node.Text, true },
		func(s storageLink, l linkText) {
			has, excluded := codeState(l.node.Marks)
			if !s.code || has || excluded {
				return
			}
			fixed := edit(l.node)
			fixed.Marks = append(append([]adf.Mark{}, l.node.Marks...), &adf.Code{})
		})
}

// restorePageSlugs appends the title slug to an internal page href that
// the read gave without one. The title comes from the ri:page of the
// storage link, which carries no page id, so the two sides are aligned
// on the link text alone.
func restorePageSlugs(stored []storageLink, links []linkText, edit func(*adf.Text) *adf.Text) {
	align(stored,
		func(s storageLink) (string, bool) { return s.text, s.title != "" },
		links,
		func(l linkText) (string, bool) {
			at := pageSlugRe.FindStringIndex(l.href)
			return l.node.Text, at != nil
		},
		func(s storageLink, l linkText) {
			slug := pageSlug(s.title)
			if slug == "" {
				return
			}
			// The slug goes after the page id and before a query or a
			// fragment, which is where Confluence puts it.
			id := pageSlugRe.FindStringSubmatchIndex(l.href)
			cut := id[1]
			if id[2] >= 0 {
				cut = id[2]
			}
			href := l.href[:cut] + "/" + slug + l.href[cut:]
			fixed := edit(l.node)
			fixed.Marks = withHref(l.node.Marks, href)
		})
}

// align pairs the storage side and the ADF side by key, position for
// position within a key. A key whose two sides hold a different number
// of links is skipped whole: the sides cannot be aligned there, and a
// guessed pairing would corrupt a link. That is also what happens for a
// link the two formats do not describe one for one, such as a link
// whose text ADF splits over several nodes.
func align(
	stored []storageLink, keyOf func(storageLink) (string, bool),
	links []linkText, keyIn func(linkText) (string, bool),
	apply func(storageLink, linkText),
) {
	byKey := map[string][]storageLink{}
	for _, s := range stored {
		if key, ok := keyOf(s); ok {
			byKey[key] = append(byKey[key], s)
		}
	}
	grouped := map[string][]linkText{}
	order := []string{}
	for _, l := range links {
		key, ok := keyIn(l)
		if !ok {
			continue
		}
		if _, seen := grouped[key]; !seen {
			order = append(order, key)
		}
		grouped[key] = append(grouped[key], l)
	}
	for _, key := range order {
		group, side := grouped[key], byKey[key]
		if len(group) != len(side) {
			continue
		}
		for i, l := range group {
			apply(side[i], l)
		}
	}
}

// linkText is one link-carrying text node of the ADF document, in
// document order, with the href of its link mark.
type linkText struct {
	node *adf.Text
	href string
}

// linkedText collects the text nodes that carry a link mark with an
// href, in document order.
func linkedText(doc adf.Doc) []linkText {
	var links []linkText
	for _, top := range doc.Content {
		for n := range adf.Walk(top) {
			t, ok := n.(*adf.Text)
			if !ok {
				continue
			}
			for _, m := range t.Marks {
				if link, isLink := m.(*adf.Link); isLink && link.Href != nil {
					links = append(links, linkText{node: t, href: *link.Href})
					break
				}
			}
		}
	}
	return links
}

// codeState reports whether marks already carry the code mark, and
// whether they carry a mark that ADF strips from code-marked text.
func codeState(marks []adf.Mark) (has, excluded bool) {
	for _, m := range marks {
		switch m.(type) {
		case *adf.Code:
			has = true
		case *adf.Strong, *adf.Em, *adf.Strike:
			excluded = true
		}
	}
	return has, excluded
}

// withHref returns marks with the href of the link mark replaced.
func withHref(marks []adf.Mark, href string) []adf.Mark {
	out := append([]adf.Mark{}, marks...)
	for i, m := range out {
		link, ok := m.(*adf.Link)
		if !ok {
			continue
		}
		relinked := *link
		relinked.Href = &href
		out[i] = &relinked
		break
	}
	return out
}

// pageSlug builds the title slug of a Confluence page URL. The rule was
// measured against the canonical webui link of 1237 live pages, with no
// mismatch: every character outside [A-Za-z0-9._-] becomes a space, runs
// of whitespace collapse, and the words join with "+". A title is
// therefore not URL-encoded but reduced — "Verträge" gives "Vertr+ge".
func pageSlug(title string) string {
	var words strings.Builder
	for _, r := range title {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			words.WriteRune(r)
		case r == '.', r == '_', r == '-':
			words.WriteRune(r)
		default:
			words.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(words.String()), "+")
}

// storageLink is one link of the storage body, in document order. An
// <a> gives href; an <ac:link> around an <ri:page> gives title. Both
// give the link text and whether a <code> element wraps the link.
type storageLink struct {
	href  string
	title string
	text  string
	code  bool
}

// parseStorageLinks reads the links out of a Confluence storage body.
// The body is an XHTML fragment with several roots, unbound namespace
// prefixes (ac, ri) and HTML entities, so the decoder runs in the
// tolerant mode. A body it cannot read gives the links found so far,
// and an unalignable side repairs nothing.
func parseStorageLinks(storage string) []storageLink {
	dec := xml.NewDecoder(strings.NewReader(storage))
	dec.Strict = false
	dec.AutoClose = storageAutoClose
	dec.Entity = xml.HTMLEntity

	var sc linkScanner
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			sc.start(t)
		case xml.EndElement:
			sc.end(elemName(t.Name))
		case xml.CharData:
			if sc.cur != nil && sc.body {
				sc.text.Write(t)
			}
		}
	}
	return sc.links
}

// linkScanner is the state parseStorageLinks threads through the storage
// body's token stream.
type linkScanner struct {
	links []storageLink
	cur   *storageLink
	text  strings.Builder
	depth int  // element depth inside the current link
	body  bool // inside the text-carrying part of the current link
	codes int  // open <code> elements
}

// start opens an element. Inside a link it picks up the page title and
// the text-carrying body; outside one it opens a new link or tracks the
// <code> nesting that decides a link's code flag.
func (sc *linkScanner) start(t xml.StartElement) {
	name := elemName(t.Name)
	switch {
	case sc.cur != nil:
		sc.depth++
		switch name {
		case "ri:page":
			sc.cur.title = attrValue(t, "ri", "content-title")
		case "ac:link-body", "ac:plain-text-link-body":
			sc.body = true
		}
	case name == "code":
		sc.codes++
	case name == "a":
		sc.open(&storageLink{href: attrValue(t, "", "href"), code: sc.codes > 0}, true)
	case name == "ac:link":
		sc.open(&storageLink{code: sc.codes > 0}, false)
	}
}

// open begins a new link. body says whether its text starts right away
// (an <a>) or waits for a link-body element (an <ac:link>).
func (sc *linkScanner) open(link *storageLink, body bool) {
	sc.cur = link
	sc.depth, sc.body = 0, body
	sc.text.Reset()
}

// end closes an element, finishing the current link when its own end tag
// arrives (depth 0).
func (sc *linkScanner) end(name string) {
	switch {
	case sc.cur != nil && sc.depth == 0:
		sc.cur.text = sc.text.String()
		sc.links = append(sc.links, *sc.cur)
		sc.cur, sc.body = nil, false
	case sc.cur != nil:
		sc.depth--
		if name == "ac:link-body" || name == "ac:plain-text-link-body" {
			sc.body = false
		}
	case name == "code" && sc.codes > 0:
		sc.codes--
	}
}

// elemName joins the namespace prefix back onto the local name. The
// storage body declares no prefix, so the tolerant decoder reports the
// literal prefix ("ac", "ri") as the namespace.
func elemName(n xml.Name) string {
	if n.Space == "" {
		return strings.ToLower(n.Local)
	}
	return strings.ToLower(n.Space + ":" + n.Local)
}

// attrValue reads one attribute of a start element by prefix and name.
func attrValue(e xml.StartElement, space, local string) string {
	for _, a := range e.Attr {
		if a.Name.Space == space && strings.EqualFold(a.Name.Local, local) {
			return a.Value
		}
	}
	return ""
}
