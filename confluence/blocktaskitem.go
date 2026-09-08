package confluence

import (
	"strings"

	"github.com/pmarschik/adfast/adf"
)

// Multi-block task items: the separator Confluence's downgrade omits.
//
// Confluence does not keep the blockTaskItem kind. It DOWNGRADES the node
// to a plain taskItem and flattens the block body to inline content, and
// the measurement behind UnsupportedKinds (live Cloud site, 2026-09-05)
// pins the join: the blocks are concatenated WITH NO SEPARATOR AT ALL.
// A blockTaskItem holding the paragraphs "FIRSTPARA" and "SECONDPARA" is
// stored as a taskItem with two adjacent text nodes and nothing between
// them, so the stored text reads "FIRSTPARASECONDPARA" — the last word of
// one block runs into the first word of the next. The save reports
// success, so nothing tells the author the prose was mangled.
//
// The remedy is for adfast to supply the join itself, before submission,
// rather than leave the boundary to a product that spells it as the empty
// string. That is a dialect lowering, exactly like LowerAnchors: it makes
// the document wire-safe for Confluence and it must NOT run on the Jira
// leg, where blockTaskItem is kept as sent and a join would be corruption
// of its own.
//
// WHY A SPACE, AND NOT A hardBreak. The same session proved Confluence
// validates the Task node's content strictly: a plain taskItem carrying
// paragraphs is refused with HTTP 400, "Unsupported node type found
// inside Task node: paragraph". A separator this code invents therefore
// has to be a kind the Task node is KNOWN to accept, or the fix turns
// silent corruption into a save that fails outright. Exactly one kind is
// known by measurement rather than by inference: the downgrade's own
// output was text nodes inside a taskItem, so text is accepted. hardBreak
// is plausible — adfast already emits one inside a plain taskItem for a
// markdown line break, and no rejection has ever been reported — but
// plausible is not measured, and there is no live access here to settle
// it. The space is the join that the available evidence proves safe.
//
// WHERE THE SPACE GOES. The measurement also shows the flattening does
// not merge or normalize anything: it left the two source paragraphs as
// two SEPARATE adjacent text nodes. A separator handed to it therefore
// survives verbatim, which is what makes either placement below work.
//
//   - After a paragraph, the space joins that paragraph's own trailing
//     text. It is invisible in every other reading of the document — a
//     trailing space renders as nothing — so if Confluence ever stops
//     downgrading the kind, this lowering leaves no visible trace.
//   - After any other block (a code block, a list, a nested task list),
//     there is no trailing text to extend and no way to add one without
//     editing the block's own content — a space appended inside a code
//     block would be a change to the code. Those get a separator
//     paragraph holding a single space instead, which the same flattening
//     contributes as a lone " " text node.
//
// What Confluence's downgrade makes of a non-paragraph block is NOT
// measured — whether a code block's text inlines, whether a nested list
// survives as anything. The separator is inserted there on the same
// reasoning regardless: if the block contributes text, the text is
// separated; if it contributes nothing, a stray space is the whole cost.

// SeparateBlockTaskItems inserts, between the blocks of every
// blockTaskItem, the separator Confluence's downgrade to a plain taskItem
// omits — a trailing space on a paragraph, or a separator paragraph
// holding one space after any other block kind. Without it the block
// bodies concatenate bare and the stored prose runs together; see the
// package comment above and UnsupportedKinds.
//
// A blockTaskItem with fewer than two blocks has no boundary and is left
// alone, as is a block that already ends in whitespace, which makes the
// transform idempotent. Nested blockTaskItems (a task list inside a task
// item) are separated too.
//
// MarkdownOptions installs it as a document transform for the Confluence
// leg only. It must not be installed on the Jira leg: Jira keeps
// blockTaskItem as sent, so there is no concatenation to defend against
// and the separator would be pure corruption.
func SeparateBlockTaskItems(doc adf.Doc) adf.Doc {
	content, _ := separateTaskBlocks(doc.Content)
	return adf.Doc{Type: doc.Type, Version: doc.Version, Content: content}
}

// separateTaskBlocks rewrites one content slice bottom-up, reporting
// whether anything changed (unchanged slices are returned as-is for
// copy-on-write). It recurses before it rewrites so that a blockTaskItem
// nested inside another one is separated as well — adf.Transform prunes
// the subtree of a handled node, which would miss those.
func separateTaskBlocks(nodes []adf.Node) ([]adf.Node, bool) {
	if len(nodes) == 0 {
		return nodes, false
	}
	changed := false
	out := make([]adf.Node, 0, len(nodes))
	for _, n := range nodes {
		if kids := adf.NodeContent(n); len(kids) > 0 {
			if separated, kidsChanged := separateTaskBlocks(kids); kidsChanged {
				n = adf.WithContent(n, separated)
				changed = true
			}
		}
		if item, ok := n.(*adf.BlockTaskItem); ok {
			if body, bodyChanged := separatedTaskBody(item.Content); bodyChanged {
				spaced := *item
				spaced.Content = body
				n = &spaced
				changed = true
			}
		}
		out = append(out, n)
	}
	if !changed {
		return nodes, false
	}
	return out, true
}

// separatedTaskBody returns the blockTaskItem body with a separator after
// every block but the last.
func separatedTaskBody(blocks []adf.Node) ([]adf.Node, bool) {
	if len(blocks) < 2 {
		return blocks, false
	}
	changed := false
	out := make([]adf.Node, 0, 2*len(blocks)-1)
	for i, b := range blocks {
		if i == len(blocks)-1 {
			out = append(out, b)
			continue
		}
		if p, ok := b.(*adf.Paragraph); ok {
			spaced, added := paragraphEndingInSpace(p)
			out = append(out, spaced)
			changed = changed || added
			continue
		}
		out = append(out, b)
		// A boundary already carrying a whitespace-only paragraph needs
		// nothing more; inserting anyway would stack separators on a
		// second pass.
		if isSeparatorParagraph(blocks[i+1]) {
			continue
		}
		out = append(out, separatorParagraph())
		changed = true
	}
	if !changed {
		return blocks, false
	}
	return out, true
}

// paragraphEndingInSpace returns the paragraph with a trailing space,
// reporting whether one had to be added. A paragraph whose inline run
// already ends in whitespace is returned untouched; one ending in a text
// node grows that node (rather than gaining a second, adjacent one), and
// anything else — a card, a mention, an emoji, a hard break, or no
// content at all — gains a space text node.
func paragraphEndingInSpace(p *adf.Paragraph) (adf.Node, bool) {
	if n := len(p.Content); n > 0 {
		if last, ok := p.Content[n-1].(*adf.Text); ok {
			if strings.TrimRight(last.Text, " \t") != last.Text {
				return p, false
			}
			grown := *last
			grown.Text = last.Text + " "
			spaced := *p
			spaced.Content = append(append(make([]adf.Node, 0, n), p.Content[:n-1]...), &grown)
			return &spaced, true
		}
	}
	spaced := *p
	spaced.Content = append(append(make([]adf.Node, 0, len(p.Content)+1), p.Content...), &adf.Text{Text: " "})
	return &spaced, true
}

// separatorParagraph is the standalone separator used after a block that
// has no trailing text to extend.
func separatorParagraph() adf.Node {
	return &adf.Paragraph{Content: []adf.Node{&adf.Text{Text: " "}}}
}

// isSeparatorParagraph reports whether the block already fills a boundary
// on its own: a paragraph holding nothing but whitespace text, which is
// what separatorParagraph produces and all a second separator beside it
// could contribute.
func isSeparatorParagraph(n adf.Node) bool {
	p, ok := n.(*adf.Paragraph)
	if !ok || len(p.Content) == 0 {
		return false
	}
	for _, kid := range p.Content {
		t, isText := kid.(*adf.Text)
		if !isText || strings.TrimSpace(t.Text) != "" {
			return false
		}
	}
	return true
}
