package adfast

import (
	"strings"
	"testing"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/convert"
)

const (
	cardHref  = "session-06.pdf"
	cardID    = "1f2e3d4c-5b6a-7988-9a0b-1c2d3e4f5060"
	cardColl  = "contentId-1373470724"
	cardLabel = "Slides"
)

func fileCards() convert.FileCards {
	return convert.FileCards{
		Card: func(href string) (convert.FileCard, bool) {
			if href != cardHref {
				return convert.FileCard{}, false
			}
			return convert.FileCard{ID: cardID, Collection: cardColl}, true
		},
		Link: func(id string) (convert.FileCardLink, bool) {
			if id != cardID {
				return convert.FileCardLink{}, false
			}
			return convert.FileCardLink{Href: cardHref, Label: cardLabel}, true
		},
	}
}

func TestFileCardsPublishALabelledLinkAsACardInATable(t *testing.T) {
	md := "| Session | Slides |\n| --- | --- |\n| 06 | [**" + cardLabel + "**](" + cardHref + ") |\n"

	doc := ToADF(FromMarkdown(md), WithFileCards(fileCards()))
	if links := linkHrefs(doc); len(links) != 0 {
		t.Errorf("the published card kept a link: %v", links)
	}
	cards := mediaInlines(doc)
	if len(cards) != 1 {
		t.Fatalf("got %d media nodes, want 1", len(cards))
	}
	if cards[0].ID != cardID || cards[0].Type != "file" {
		t.Errorf("card = %+v, want id %s of type file", cards[0], cardID)
	}
	if cards[0].Collection == nil || *cards[0].Collection != cardColl {
		t.Errorf("card collection = %v, want %s", cards[0].Collection, cardColl)
	}
}

func TestFileCardsReadACardBackAsItsLink(t *testing.T) {
	md := "[" + cardLabel + "](" + cardHref + ")\n"

	doc := ToADF(FromMarkdown(md), WithFileCards(fileCards()))
	got := ToMarkdown(FromADF(doc, WithFileCards(fileCards())))
	want := "[" + cardLabel + "](" + cardHref + ")"
	if !strings.Contains(got, want) {
		t.Errorf("card did not read back as %s:\n%s", want, got)
	}
}

func TestFileCardsPassTheLinkThroughTheLinkResolver(t *testing.T) {
	remote := "/wiki/download/attachments/42/" + cardHref
	resolver := convert.LinkResolver{
		Encode: func(href string) (string, bool) { return remote, href == cardHref },
		Decode: func(href string) (string, bool) { return cardHref, href == remote },
	}
	// The card resolver reads the encoded href, and gives the encoded one back.
	cards := convert.FileCards{
		Card: func(href string) (convert.FileCard, bool) {
			if href != remote {
				return convert.FileCard{}, false
			}
			return convert.FileCard{ID: cardID}, true
		},
		Link: func(string) (convert.FileCardLink, bool) {
			return convert.FileCardLink{Href: remote}, true
		},
	}

	doc := ToADF(FromMarkdown("[x]("+cardHref+")"), WithLinkResolver(resolver), WithFileCards(cards))
	if got := mediaInlines(doc); len(got) != 1 || got[0].ID != cardID {
		t.Fatalf("media nodes = %+v, wanted the card for the encoded href", got)
	}
	got := ToMarkdown(FromADF(doc, WithLinkResolver(resolver), WithFileCards(cards)))
	// No label of its own and no alt text, so the file name stands in for one.
	if want := "[" + cardHref + "](" + cardHref + ")"; !strings.Contains(got, want) {
		t.Errorf("card did not decode to %s:\n%s", want, got)
	}
}

func TestFileCardsLeaveMissesAndOtherMediaAlone(t *testing.T) {
	inert := convert.FileCards{
		Card: func(string) (convert.FileCard, bool) { return convert.FileCard{}, false },
		Link: func(string) (convert.FileCardLink, bool) { return convert.FileCardLink{}, false },
	}

	doc := ToADF(FromMarkdown("[site](https://example.com)"), WithFileCards(inert))
	if links := linkHrefs(doc); len(links) != 1 || links[0] != "https://example.com" {
		t.Errorf("a card resolver miss changed the hrefs: %v", links)
	}
	if cards := mediaInlines(doc); len(cards) != 0 {
		t.Errorf("a card resolver miss published %d cards", len(cards))
	}

	other := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{
		&adf.Paragraph{Content: []adf.Node{&adf.MediaInline{ID: "other", Type: "file", Alt: "other.pdf"}}},
	}}
	round := ToADF(FromADF(other, WithFileCards(inert)), WithFileCards(inert))
	if cards := mediaInlines(round); len(cards) != 1 || cards[0].ID != "other" {
		t.Errorf("media nodes = %+v, want the untouched other.pdf card", cards)
	}
}

func mediaInlines(doc adf.Doc) []*adf.MediaInline {
	var out []*adf.MediaInline
	for _, top := range doc.Content {
		for node := range adf.Walk(top) {
			if media, ok := node.(*adf.MediaInline); ok {
				out = append(out, media)
			}
		}
	}
	return out
}
