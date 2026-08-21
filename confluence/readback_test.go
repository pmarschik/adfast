package confluence

import (
	"testing"

	"github.com/pmarschik/adfast/adf"
)

// linked builds a text node with a link mark, the shape Confluence's ADF
// page read gives for a link.
func linked(text, href string, marks ...adf.Mark) *adf.Text {
	return &adf.Text{Text: text, Marks: append([]adf.Mark{&adf.Link{Href: &href}}, marks...)}
}

// para wraps inline nodes in a document with one paragraph.
func para(content ...adf.Node) adf.Doc {
	return adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{&adf.Paragraph{Content: content}}}
}

// firstText returns the nth text node of the document, in document order.
func firstText(t *testing.T, doc adf.Doc, n int) *adf.Text {
	t.Helper()
	at := 0
	for _, top := range doc.Content {
		for node := range adf.Walk(top) {
			text, ok := node.(*adf.Text)
			if !ok {
				continue
			}
			if at == n {
				return text
			}
			at++
		}
	}
	t.Fatalf("document holds fewer than %d text nodes", n+1)
	return nil
}

// linkHref returns the href of the node's link mark.
func linkHref(t *testing.T, node *adf.Text) string {
	t.Helper()
	for _, m := range node.Marks {
		if link, ok := m.(*adf.Link); ok && link.Href != nil {
			return *link.Href
		}
	}
	t.Fatalf("text %q carries no link href", node.Text)
	return ""
}

// TestRepairReadBackCodeMark covers the first measured loss, with the
// storage and ADF shapes read off a live page: <code> wraps the anchor
// in storage, and the ADF read gives the link mark alone.
func TestRepairReadBackCodeMark(t *testing.T) {
	const storage = `<p>Wrap with ` +
		`<code><a href="http://errors.Is">errors.Is</a></code> here.</p>`

	doc := para(&adf.Text{Text: "Wrap with "}, linked("errors.Is", "http://errors.Is"), &adf.Text{Text: " here."})
	got := RepairReadBack(doc, storage)

	has, _ := codeState(firstText(t, got, 1).Marks)
	if !has {
		t.Fatalf("code mark not restored: %#v", firstText(t, got, 1).Marks)
	}
	// The input document is never mutated.
	if has, _ := codeState(firstText(t, doc, 1).Marks); has {
		t.Error("the repair mutated the input document")
	}
	if md := adfToMD(t, got); md != "Wrap with [`errors.Is`](http://errors.Is) here.\n" {
		t.Fatalf("markdown = %q", md)
	}
}

// TestRepairReadBackCodeMarkPositional pins the alignment on the shape
// that makes it necessary: one href, two links, and only the first one
// wrapped in <code>. Storage writes one <a> for each ADF text node, so
// position i on one side is position i on the other.
func TestRepairReadBackCodeMarkPositional(t *testing.T) {
	const href = "https://github.com/rdleal/intervalst"
	const storage = `<p><code><a href="` + href + `">intervalst</a></code>` +
		`<a href="` + href + `"> package</a></p>`

	got := RepairReadBack(para(linked("intervalst", href), linked(" package", href)), storage)

	if has, _ := codeState(firstText(t, got, 0).Marks); !has {
		t.Error("code mark not restored on the wrapped link")
	}
	if has, _ := codeState(firstText(t, got, 1).Marks); has {
		t.Error("code mark added to the unwrapped link")
	}
}

// TestRepairReadBackPageSlug covers the second measured loss: storage
// keeps the page title in ri:content-title, the ADF read keeps the page
// id, and only the two together spell the href that was submitted.
func TestRepairReadBackPageSlug(t *testing.T) {
	const storage = `<p><ac:link><ri:page ri:space-key="PT" ` +
		`ri:content-title="BIN Lookup Knowledge Base" ri:version-at-save="9" />` +
		`<ac:link-body>Page: BIN Lookup Knowledge Base</ac:link-body></ac:link></p>`
	const read = "https://ixolit.atlassian.net/wiki/spaces/PT/pages/443514894"
	const want = read + "/BIN+Lookup+Knowledge+Base"

	got := RepairReadBack(para(linked("Page: BIN Lookup Knowledge Base", read)), storage)
	if href := linkHref(t, firstText(t, got, 0)); href != want {
		t.Fatalf("href = %q, want %q", href, want)
	}
}

// TestRepairReadBackPageSlugPlacement pins where the slug goes when the
// href carries a fragment or a query. That order is Confluence's own
// URL order; it is not measured against a live cross-page anchor link.
func TestRepairReadBackPageSlugPlacement(t *testing.T) {
	const storage = `<ac:link><ri:page ri:content-title="Guide" />` +
		`<ac:link-body>Guide</ac:link-body></ac:link>`
	for _, tc := range []struct{ read, want string }{
		{
			read: "https://x.atlassian.net/wiki/spaces/PT/pages/1#logging",
			want: "https://x.atlassian.net/wiki/spaces/PT/pages/1/Guide#logging",
		},
		{
			read: "https://x.atlassian.net/wiki/spaces/PT/pages/1?focused=true",
			want: "https://x.atlassian.net/wiki/spaces/PT/pages/1/Guide?focused=true",
		},
	} {
		t.Run(tc.read, func(t *testing.T) {
			got := RepairReadBack(para(linked("Guide", tc.read)), storage)
			if href := linkHref(t, firstText(t, got, 0)); href != tc.want {
				t.Fatalf("href = %q, want %q", href, tc.want)
			}
		})
	}
}

// TestRepairReadBackLeavesAlone covers the cases where the repair must
// change nothing. Each one either has nothing to repair or cannot align
// the two sides, and a guess there would corrupt a link.
func TestRepairReadBackLeavesAlone(t *testing.T) {
	const href = "https://x.atlassian.net/wiki/spaces/PT/pages/1"
	cases := []struct {
		name    string
		storage string
		doc     adf.Doc
	}{
		{
			name:    "no storage body",
			storage: "",
			doc:     para(linked("errors.Is", "http://errors.Is")),
		},
		{
			name:    "storage holds no link",
			storage: "<p>plain text</p>",
			doc:     para(linked("errors.Is", "http://errors.Is")),
		},
		{
			name:    "unparsable storage",
			storage: `<p><code><a href="http://errors.Is">errors.Is`,
			doc:     para(linked("errors.Is", "http://errors.Is")),
		},
		{
			name: "more links in storage than in the document",
			storage: `<code><a href="http://errors.Is">errors.Is</a></code>` +
				`<code><a href="http://errors.Is">errors.Is</a></code>`,
			doc: para(linked("errors.Is", "http://errors.Is")),
		},
		{
			name:    "the href already carries a slug",
			storage: `<ac:link><ri:page ri:content-title="Renamed" /><ac:link-body>Guide</ac:link-body></ac:link>`,
			doc:     para(linked("Guide", href+"/Guide")),
		},
		{
			name:    "a title that reduces to nothing",
			storage: `<ac:link><ri:page ri:content-title="—" /><ac:link-body>Guide</ac:link-body></ac:link>`,
			doc:     para(linked("Guide", href)),
		},
		{
			name:    "the link text differs, so the sides cannot be paired",
			storage: `<ac:link><ri:page ri:content-title="Guide" /><ac:link-body>Other</ac:link-body></ac:link>`,
			doc:     para(linked("Guide", href)),
		},
		{
			name:    "code on text that ADF strips the code mark from",
			storage: `<strong><code><a href="http://errors.Is">errors.Is</a></code></strong>`,
			doc:     para(linked("errors.Is", "http://errors.Is", &adf.Strong{})),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := docJSON(t, tc.doc)
			if after := docJSON(t, RepairReadBack(tc.doc, tc.storage)); after != before {
				t.Fatalf("document changed:\n got %s\nwant %s", after, before)
			}
		})
	}
}

// TestRepairReadBackUnparsableStorageKeepsEarlierLinks pins that a body
// which breaks halfway still repairs what it read before the break. The
// repair is per link, so a truncated body is not an all-or-nothing loss.
func TestRepairReadBackUnparsableStorageKeepsEarlierLinks(t *testing.T) {
	const storage = `<p><code><a href="http://errors.Is">errors.Is</a></code></p><p><a href="`

	got := RepairReadBack(para(linked("errors.Is", "http://errors.Is")), storage)
	if has, _ := codeState(firstText(t, got, 0).Marks); !has {
		t.Fatal("the link before the break was not repaired")
	}
}

// TestPageSlug pins the slug rule against real page titles and their
// canonical webui links. The rule held for 1237 live pages with no
// mismatch: a character outside [A-Za-z0-9._-] becomes a space, runs of
// whitespace collapse, and the words join with "+". A title is reduced,
// not URL-encoded, so a non-ASCII letter disappears.
func TestPageSlug(t *testing.T) {
	cases := []struct{ title, want string }{
		{"BIN Lookup Knowledge Base", "BIN+Lookup+Knowledge+Base"},
		{"Client SSH/SFTP key generation and storage / retrieval", "Client+SSH+SFTP+key+generation+and+storage+retrieval"},
		{"Deployment & Testing Pipeline", "Deployment+Testing+Pipeline"},
		{"Schulung/Training - Ergänzende Bedingungen", "Schulung+Training+-+Erg+nzende+Bedingungen"},
		{"Enable BDS (BI Data Source) Access for new Tenants", "Enable+BDS+BI+Data+Source+Access+for+new+Tenants"},
		{"How To’s", "How+To+s"},
		{"How-To's", "How-To+s"},
		{"Alipay / Alipay +", "Alipay+Alipay"},
		{`Mastercard Connect: Promote / Demote "Security Administrator"`, "Mastercard+Connect+Promote+Demote+Security+Administrator"},
		{"StripeV2 PCI (StripeDirect, StripePCI)", "StripeV2+PCI+StripeDirect+StripePCI"},
		{"Why Google Docs?", "Why+Google+Docs"},
		{
			"tcp://active.isonac.service.consul:28010:XXXX; socket_read() failed: Reason: Resource temporarily unavailable",
			"tcp+active.isonac.service.consul+28010+XXXX+socket_read+failed+Reason+Resource+temporarily+unavailable",
		},
		{"", ""},
		{"漢字", ""},
	}
	for _, tc := range cases {
		t.Run(tc.title, func(t *testing.T) {
			if got := pageSlug(tc.title); got != tc.want {
				t.Fatalf("pageSlug(%q) = %q, want %q", tc.title, got, tc.want)
			}
		})
	}
}

// TestParseStorageLinks pins the reader against the storage shapes the
// live page holds, including the entity that the tolerant decoder has to
// resolve and the same-page anchor link, which carries no href at all.
func TestParseStorageLinks(t *testing.T) {
	const storage = `<p><a href="https://docs.pagos.ai/x">pagos.ai &mdash; Overview</a>` +
		`<code><a href="http://errors.Is">errors.Is</a></code>` +
		`<ac:link><ri:page ri:space-key="PT" ri:content-title="One Pager" /><ac:link-body>Page: One Pager</ac:link-body></ac:link>` +
		`<ac:link ac:anchor="logging"><ac:link-body>Logging</ac:link-body></ac:link>` +
		`<ac:link><ri:page ri:content-title="Plain" /><ac:plain-text-link-body><![CDATA[Plain]]></ac:plain-text-link-body></ac:link></p>`

	want := []storageLink{
		{href: "https://docs.pagos.ai/x", text: "pagos.ai — Overview"},
		{href: "http://errors.Is", text: "errors.Is", code: true},
		{title: "One Pager", text: "Page: One Pager"},
		{text: "Logging"},
		{title: "Plain", text: "Plain"},
	}
	got := parseStorageLinks(storage)
	if len(got) != len(want) {
		t.Fatalf("read %d links, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("link %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
