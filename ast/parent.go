package ast

// Every container kind in this package implements Parent over its
// Children field, so Children and SetChildren (ast.go) need no per-kind
// knowledge: one interface dispatch answers for the in-package kinds and
// the foreign ones alike.
//
// Methods rather than the type switches these replace, because a switch
// answered an unlisted container with "no children" — a new kind's
// subtree silently fell out of every walk. Here the omission shows up
// the same way, but in one place: a kind with a Children field and no
// methods simply does not satisfy Parent, and TestEveryContainerIsParent
// (parent_test.go) fails.

// ChildNodes implements Parent.
func (n *Root) ChildNodes() []Node { return n.Children }

// SetChildNodes implements Parent.
func (n *Root) SetChildNodes(kids []Node) { n.Children = kids }

// ChildNodes implements Parent.
func (n *Paragraph) ChildNodes() []Node { return n.Children }

// SetChildNodes implements Parent.
func (n *Paragraph) SetChildNodes(kids []Node) { n.Children = kids }

// ChildNodes implements Parent.
func (n *Heading) ChildNodes() []Node { return n.Children }

// SetChildNodes implements Parent.
func (n *Heading) SetChildNodes(kids []Node) { n.Children = kids }

// ChildNodes implements Parent.
func (n *Blockquote) ChildNodes() []Node { return n.Children }

// SetChildNodes implements Parent.
func (n *Blockquote) SetChildNodes(kids []Node) { n.Children = kids }

// ChildNodes implements Parent.
func (n *FootnoteDef) ChildNodes() []Node { return n.Children }

// SetChildNodes implements Parent.
func (n *FootnoteDef) SetChildNodes(kids []Node) { n.Children = kids }

// ChildNodes implements Parent.
func (n *List) ChildNodes() []Node { return n.Children }

// SetChildNodes implements Parent.
func (n *List) SetChildNodes(kids []Node) { n.Children = kids }

// ChildNodes implements Parent.
func (n *ListItem) ChildNodes() []Node { return n.Children }

// SetChildNodes implements Parent.
func (n *ListItem) SetChildNodes(kids []Node) { n.Children = kids }

// ChildNodes implements Parent.
func (n *Table) ChildNodes() []Node { return n.Children }

// SetChildNodes implements Parent.
func (n *Table) SetChildNodes(kids []Node) { n.Children = kids }

// ChildNodes implements Parent.
func (n *TableRow) ChildNodes() []Node { return n.Children }

// SetChildNodes implements Parent.
func (n *TableRow) SetChildNodes(kids []Node) { n.Children = kids }

// ChildNodes implements Parent.
func (n *TableCell) ChildNodes() []Node { return n.Children }

// SetChildNodes implements Parent.
func (n *TableCell) SetChildNodes(kids []Node) { n.Children = kids }

// ChildNodes implements Parent.
func (n *ContainerDirective) ChildNodes() []Node { return n.Children }

// SetChildNodes implements Parent.
func (n *ContainerDirective) SetChildNodes(kids []Node) { n.Children = kids }

// ChildNodes implements Parent.
func (n *LeafDirective) ChildNodes() []Node { return n.Children }

// SetChildNodes implements Parent.
func (n *LeafDirective) SetChildNodes(kids []Node) { n.Children = kids }

// ChildNodes implements Parent.
func (n *Emphasis) ChildNodes() []Node { return n.Children }

// SetChildNodes implements Parent.
func (n *Emphasis) SetChildNodes(kids []Node) { n.Children = kids }

// ChildNodes implements Parent.
func (n *Strong) ChildNodes() []Node { return n.Children }

// SetChildNodes implements Parent.
func (n *Strong) SetChildNodes(kids []Node) { n.Children = kids }

// ChildNodes implements Parent.
func (n *Delete) ChildNodes() []Node { return n.Children }

// SetChildNodes implements Parent.
func (n *Delete) SetChildNodes(kids []Node) { n.Children = kids }

// ChildNodes implements Parent.
func (n *Link) ChildNodes() []Node { return n.Children }

// SetChildNodes implements Parent.
func (n *Link) SetChildNodes(kids []Node) { n.Children = kids }

// ChildNodes implements Parent.
func (n *Image) ChildNodes() []Node { return n.Children }

// SetChildNodes implements Parent.
func (n *Image) SetChildNodes(kids []Node) { n.Children = kids }

// ChildNodes implements Parent.
func (n *TextDirective) ChildNodes() []Node { return n.Children }

// SetChildNodes implements Parent.
func (n *TextDirective) SetChildNodes(kids []Node) { n.Children = kids }
