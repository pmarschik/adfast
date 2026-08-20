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
