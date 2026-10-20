package adfast

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

func TestLinkResolverRewritesLabelledLinksInTablesBothWays(t *testing.T) {
	const local = "session-06.pdf"
	const remote = "/wiki/download/attachments/42/session-06.pdf"
	resolver := convert.LinkResolver{
		Encode: func(href string) (string, bool) { return remote, href == local },
		Decode: func(href string) (string, bool) { return local, href == remote },
	}
	md := "| Session | Slides |\n| --- | --- |\n| 06 | [**PDF**](session-06.pdf) |\n"

	doc := ToADF(FromMarkdown(md), WithLinkResolver(resolver))
	links := linkHrefs(doc)
	if len(links) != 1 || links[0] != remote {
		t.Fatalf("link hrefs = %v, want [%s]", links, remote)
	}
	got := ToMarkdown(FromADF(doc, WithLinkResolver(resolver)))
	if !strings.Contains(got, "**[PDF](session-06.pdf)**") {
		t.Errorf("resolved table link lost its label, marks, or position:\n%s", got)
	}
}

func TestLinkResolverDecodesABareAutolinkLabelToo(t *testing.T) {
	remote := "https://acme.atlassian.net/browse/PROJ-2"
	const local = "PROJ-2-target.md"
	resolver := convert.LinkResolver{
		Decode: func(href string) (string, bool) { return local, href == remote },
	}
	// A bare autolink in ADF is a text node whose visible text IS its href —
	// the shape Jira's editor makes of a pasted link, with no authored label.
	doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
		&adf.Paragraph{Content: []adf.Node{
			&adf.Text{Text: remote, Marks: []adf.Mark{&adf.Link{Href: &remote}}},
		}},
	}}
	got := ToMarkdown(FromADF(doc, WithLinkResolver(resolver)))
	if strings.Contains(got, remote) {
		t.Errorf("the pre-decode URL survived as the label:\n%s", got)
	}
	if !strings.Contains(got, "["+local+"]("+local+")") {
		t.Errorf("the bare autolink did not come back with a consistent label and path:\n%s", got)
	}
}

// A bare autolink's label IS its href, so a decode that resolves only the
// href leaves the label pointing at the pre-decode spelling. The label must
// follow.
func TestLinkResolverDecodeRewritesTheLabelOfABareLink(t *testing.T) {
	const remote = "https://acme.atlassian.net/browse/PROJ-2"
	const local = "PROJ-2-target.md"
	resolver := convert.LinkResolver{
		Decode: func(href string) (string, bool) { return local, href == remote },
	}
	doc := ToADF(FromMarkdown("<" + remote + ">"))
	got := ToMarkdown(FromADF(doc, WithLinkResolver(resolver)))
	if strings.Contains(got, remote) {
		t.Errorf("the pre-decode href survived in the output:\n%s", got)
	}
	if !strings.Contains(got, local) {
		t.Errorf("the decoded href is missing from the output:\n%s", got)
	}
}

// A bare mailto autolink's visible text is the address alone, with no
// "mailto:" prefix — the straight text-equals-href check used for http(s)
// never fires for it, so it needs its own comparison (and its own strip on
// the rewrite).
func TestLinkResolverDecodeRewritesTheLabelOfABareMailtoLink(t *testing.T) {
	const remoteAddr = "proj-2@acme.example.com"
	const localAddr = "proj-2@new.example.com"
	resolver := convert.LinkResolver{
		Decode: func(href string) (string, bool) { return "mailto:" + localAddr, href == "mailto:"+remoteAddr },
	}
	doc := ToADF(FromMarkdown("<" + remoteAddr + ">"))
	got := ToMarkdown(FromADF(doc, WithLinkResolver(resolver)))
	if strings.Contains(got, remoteAddr) {
		t.Errorf("the pre-decode address survived in the output:\n%s", got)
	}
	if !strings.Contains(got, localAddr) {
		t.Errorf("the decoded address is missing from the output:\n%s", got)
	}
}

// The GOOD case for the one above: an AUTHORED label (one that differs from
// its href) is not a mirror of the href, so a decode must leave it alone.
func TestLinkResolverDecodeLeavesAnAuthoredLabelAlone(t *testing.T) {
	const remote = "https://acme.atlassian.net/browse/PROJ-2"
	const local = "PROJ-2-target.md"
	resolver := convert.LinkResolver{
		Decode: func(href string) (string, bool) { return local, href == remote },
	}
	doc := ToADF(FromMarkdown("[the target](" + remote + ")"))
	got := ToMarkdown(FromADF(doc, WithLinkResolver(resolver)))
	if !strings.Contains(got, "[the target]("+local+")") {
		t.Errorf("an authored label was rewritten:\n%s", got)
	}
}

func TestLinkResolverLeavesMissesAndInlineCardsAlone(t *testing.T) {
	resolver := convert.LinkResolver{
		Encode: func(string) (string, bool) { return "", false },
		Decode: func(string) (string, bool) { return "", false },
	}
	doc := ToADF(FromMarkdown("[site](https://example.com)"), WithLinkResolver(resolver))
	if links := linkHrefs(doc); len(links) != 1 || links[0] != "https://example.com" {
		t.Errorf("ordinary resolver miss changed hrefs: %v", links)
	}

	cardURL := "https://example.com/card"
	card := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
		&adf.Paragraph{Content: []adf.Node{&adf.InlineCard{URL: &cardURL}}},
	}}
	got := ToADF(FromADF(card, WithLinkResolver(resolver)), WithLinkResolver(resolver))
	paragraph, ok := got.Content[0].(*adf.Paragraph)
	if !ok {
		t.Fatalf("inline card container became %T", got.Content[0])
	}
	if _, ok := paragraph.Content[0].(*adf.InlineCard); !ok {
		t.Errorf("inline card became %T", paragraph.Content[0])
	}
}

func linkHrefs(doc adf.Doc) []string {
	var out []string
	for _, top := range doc.Content {
		for node := range adf.Walk(top) {
			for _, mark := range adf.NodeMarks(node) {
				if link, ok := mark.(*adf.Link); ok && link.Href != nil {
					out = append(out, *link.Href)
				}
			}
		}
	}
	return out
}
