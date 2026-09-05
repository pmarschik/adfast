package dialect

import (
	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/extension"
)

// This file implements the one rule that keeps a dialect directive NAME
// from eating a word of prose: an inline directive that carries nothing
// but its name is not a directive, it is the text ":name".
//
// # Why the inline position needs a rule the other two do not
//
// The text form is the only directive form whose spelling collides with
// ordinary prose. A container fence (":::info") and a leaf line
// ("::media") both need their own line and a doubled colon, so an author
// writes one only on purpose. ":name" needs neither: it is a colon in
// the middle of a sentence, which is why goldmark-directive (and
// remark-directive before it, measured) reads "see :media in the log",
// "a:media b" and "deploy:status" as directives. PlainTextOf's doc
// comment already names that hazard for the plaintext projection —
// "a bare intraword colon such as 'deploy:status' or
// 'auth0:user:created' parses as a directive, and dropping it would
// silently delete text from the middle of a sentence" — and this is the
// same hazard on the ADF and formatter legs.
//
// A directive name the dialect does NOT know already degrades correctly:
// ":foo" stays a generic ast.TextDirective, renders back verbatim and
// flattens to the literal ":foo" on the ADF leg. Only a name the dialect
// DOES know was promoted, and every promotion was damage, in one of two
// shapes:
//
//   - The word disappeared. ":status", ":date", ":mention", ":emoji",
//     ":u", ":color" and the rest carry their payload in a label or an
//     attribute, so a payload-less one has nothing to encode: the ADF
//     leg dropped the node and — because the prettier formatter
//     canonicalizes through the same re-derivation — so did the md→md
//     leg. "see :status in the log" formatted to "see  in the log". That
//     is a formatter deleting an author's text, which convert's
//     NormalizeFormat exists to forbid.
//
//   - The word turned into a broken reference, silently. ":media"
//     promoted to a mediaInline with no id and no collection: the
//     markdown round-tripped byte-identically, so no diff showed
//     anything, and only the pushed ADF carried a media node addressing
//     no attachment.
//
// Both shapes have one cause, and it is upstream of either leg: the
// promotion accepted a directive with no payload at all. So the guard
// sits at the promotion, where the node is still a word of prose, rather
// than in the two places that later fail to make sense of it.
//
// # The predicate
//
// "Nothing but its name" is no label and no attributes — the same
// predicate markdown's foldableNode uses to decide that a directive may
// be folded back into a URL literal as its ":name" source, for exactly
// this reason. It is a general property, not a list of names: every
// registered text name is guarded the same way, and a name stays a
// directive the moment the author writes ANY payload, whether that is
// ":media{id=…}" or ":status[Done]".
//
// The empty forms ":name[]" and ":name{}" are bare too, and that one is
// worth spelling out because the parse COULD tell them apart:
// goldmark-directive leaves Attrs nil when there is no attribute block
// and stores a non-nil empty map for an explicit "{}". Reading that
// difference as intent would break the renderer, which writes ":name{}"
// as a separator when a bare-rendering directive is followed by an
// emphasis or brace hazard (markdown.needsPunctTrail) and drops the
// block otherwise. The empty block is therefore a rendering artifact,
// not an author's payload — treating it as payload would mean the
// formatter turns ":media{}" into ":media" and so deletes a directive.
// It carries nothing either way, so it is nothing either way.
//
// # What this costs on the ADF leg
//
// A directive that renders bare now re-parses as prose, and the one node
// that still renders bare comes from ADF: a mediaInline with neither id
// nor collection. That chip does not survive a trip through markdown any
// more — it comes back as the word ":media". Nothing addressable is
// lost, because the chip addressed nothing (see SourcelessMedia), and
// the reader gets a word instead of an attachment reference that resolves
// to no attachment. TestSourcelessChipFromADFGetsTheTail measures it.
//
// # Scope
//
// Only the dialect's own text constructors are guarded. A user
// registration (or the confluence macro set, whose ":toc" and friends
// are meaningful with no parameters at all) supplies its own
// constructors and keeps whatever meaning it wants for a bare name.

// bareDirective is what a KNOWN inline directive name degrades to when
// the author wrote nothing but the name: the literal text ":name", on
// every leg.
//
// It is deliberately unexported. It is not a directive kind — it is the
// absence of one — so it takes dialect.Visitor's documented VisitOther
// escape rather than a method of its own, and consumers that need the
// written name off a tree read it the way they always have, from a
// parse with markdown.WithGenericDirectives.
//
// Children hold that literal text so the generic walks see it: ast.PlainText
// falls through to the children of a kind it does not know, and an image
// alt is built from exactly that walk ("![Over:media](x.png)" used to
// read back as "Over"). RenderMarkdown does not spend them, because the
// renderer has no literal-text primitive to spend them through — it
// writes the bare directive form of Name, which is the same bytes and
// re-parses to this same node.
type bareDirective struct {
	// Name is the directive name the author wrote, without the colon.
	Name string
	// Children hold the single literal ":Name" text node (see above).
	Children []ast.Node
}

// newBareDirective builds the fallback for one directive name.
func newBareDirective(name string) *bareDirective {
	return &bareDirective{Name: name, Children: []ast.Node{&ast.Text{Value: ":" + name}}}
}

// Kind implements ast.Node.
func (*bareDirective) Kind() string { return "bareDirective" }

// ChildNodes implements ast.Parent.
func (n *bareDirective) ChildNodes() []ast.Node { return n.Children }

// SetChildNodes implements ast.Parent.
func (n *bareDirective) SetChildNodes(kids []ast.Node) { n.Children = kids }

// MarkdownLead implements extension.InlineLead.
func (*bareDirective) MarkdownLead() byte { return ':' }

// RenderMarkdown implements extension.Node: the bare directive form,
// which is the literal ":name" the author typed.
func (n *bareDirective) RenderMarkdown(ctx extension.RenderContext) {
	ctx.WriteTextDirective(n.Name, nil, nil)
}

// EncodeADF implements extension.Node: the literal text, under whatever
// marks enclose it — the same degradation an unknown directive name
// gets from convert's flattener.
func (n *bareDirective) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	return ctx.EncodeInlines(n.Children)
}

// guardBareText re-wraps every text-directive constructor in regs with
// bareTextGuard, in place, and returns regs. Wrapping the assembled set
// is what makes the rule a property of the dialect rather than a note on
// each kind: a text name added later is guarded by construction.
func guardBareText(regs []extension.Registration) []extension.Registration {
	for i := range regs {
		if len(regs[i].Texts) == 0 {
			continue
		}
		guarded := make(map[string]func(*ast.TextDirective) extension.Node, len(regs[i].Texts))
		for name, ctor := range regs[i].Texts {
			guarded[name] = bareTextGuard(ctor)
		}
		regs[i].Texts = guarded
	}
	return regs
}

// bareTextGuard returns ctor with the bare-name rule in front of it.
//
// A note for whoever reads extension.Registration.Validate next: the
// prototype it builds is an empty directive, which is precisely the bare
// shape, so every guarded constructor hands it a *bareDirective and the
// structural check no longer reaches the real kinds. TestTextPrototypes
// in this package runs that same check against the kinds themselves, on
// a directive that carries a payload.
func bareTextGuard(ctor func(*ast.TextDirective) extension.Node) func(*ast.TextDirective) extension.Node {
	return func(d *ast.TextDirective) extension.Node {
		if len(d.Attrs) == 0 && len(d.Children) == 0 {
			return newBareDirective(d.Name)
		}
		return ctor(d)
	}
}
