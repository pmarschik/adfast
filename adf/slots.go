package adf

// The per-kind half of the generic tree access walk.go builds on: which
// generic slots a kind owns, and how to shallow-copy it.
//
// Both are methods on Node rather than a type switch over the kinds. The
// interface is sealed (adf.go), so requiring them costs no caller
// anything, and it buys what a switch cannot: a kind added to nodes.go
// without a slots/shallowCopy method does not compile as a Node. The
// switch version of this file answered a missing kind with an empty
// nodeSlots, which silently dropped that kind's marks and content from
// every walk.

// nodeSlots is a pointer view of a node's generic slots; nil pointers
// mean the kind has no such slot.
type nodeSlots struct {
	content *[]Node
	marks   *[]Mark
	text    *string
	extra   *map[string]any
}

// ---------------------------------------------------------------------------
// slots — the content-bearing kinds
// ---------------------------------------------------------------------------

func (n *Paragraph) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *Heading) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *Blockquote) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *CodeBlock) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *BulletList) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *OrderedList) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *ListItem) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *TaskList) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *TaskItem) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *DecisionList) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *DecisionItem) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *Table) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *TableRow) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *TableCell) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *TableHeader) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *Panel) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *Expand) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *NestedExpand) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *MediaSingle) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *MediaGroup) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *Caption) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *BlockTaskItem) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *LayoutSection) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *LayoutColumn) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *BodiedExtension) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *MultiBodiedExtension) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *ExtensionFrame) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

func (n *BodiedSyncBlock) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, extra: &n.Extra}
}

// RawNode keeps the generic wire shape verbatim, so it is the one kind
// with every slot filled — and its attribute map is spelled Attrs.
func (n *RawNode) slots() nodeSlots {
	return nodeSlots{content: &n.Content, marks: &n.Marks, text: &n.Text, extra: &n.Attrs}
}

// ---------------------------------------------------------------------------
// slots — the kinds without child content
// ---------------------------------------------------------------------------

func (n *Rule) slots() nodeSlots          { return nodeSlots{marks: &n.Marks, extra: &n.Extra} }
func (n *Media) slots() nodeSlots         { return nodeSlots{marks: &n.Marks, extra: &n.Extra} }
func (n *BlockCard) slots() nodeSlots     { return nodeSlots{marks: &n.Marks, extra: &n.Extra} }
func (n *EmbedCard) slots() nodeSlots     { return nodeSlots{marks: &n.Marks, extra: &n.Extra} }
func (n *InlineCard) slots() nodeSlots    { return nodeSlots{marks: &n.Marks, extra: &n.Extra} }
func (n *HardBreak) slots() nodeSlots     { return nodeSlots{marks: &n.Marks, extra: &n.Extra} }
func (n *Emoji) slots() nodeSlots         { return nodeSlots{marks: &n.Marks, extra: &n.Extra} }
func (n *Mention) slots() nodeSlots       { return nodeSlots{marks: &n.Marks, extra: &n.Extra} }
func (n *Status) slots() nodeSlots        { return nodeSlots{marks: &n.Marks, extra: &n.Extra} }
func (n *MediaInline) slots() nodeSlots   { return nodeSlots{marks: &n.Marks, extra: &n.Extra} }
func (n *ColwidthsHint) slots() nodeSlots { return nodeSlots{marks: &n.Marks, extra: &n.Extra} }

func (n *Date) slots() nodeSlots            { return nodeSlots{marks: &n.Marks, extra: &n.Extra} }
func (n *Placeholder) slots() nodeSlots     { return nodeSlots{marks: &n.Marks, extra: &n.Extra} }
func (n *Extension) slots() nodeSlots       { return nodeSlots{marks: &n.Marks, extra: &n.Extra} }
func (n *InlineExtension) slots() nodeSlots { return nodeSlots{marks: &n.Marks, extra: &n.Extra} }
func (n *SyncBlock) slots() nodeSlots       { return nodeSlots{marks: &n.Marks, extra: &n.Extra} }

// Text is the only typed kind whose value is a string rather than child
// nodes.
func (n *Text) slots() nodeSlots {
	return nodeSlots{marks: &n.Marks, text: &n.Text, extra: &n.Extra}
}

// ---------------------------------------------------------------------------
// shallowCopy
// ---------------------------------------------------------------------------

// Every kind copies the same way — the fields are values, slices and maps
// the caller is expected to keep sharing — so each method is the same two
// statements over a different type.

func (n *Paragraph) shallowCopy() Node     { c := *n; return &c }
func (n *Heading) shallowCopy() Node       { c := *n; return &c }
func (n *Blockquote) shallowCopy() Node    { c := *n; return &c }
func (n *Rule) shallowCopy() Node          { c := *n; return &c }
func (n *CodeBlock) shallowCopy() Node     { c := *n; return &c }
func (n *BulletList) shallowCopy() Node    { c := *n; return &c }
func (n *OrderedList) shallowCopy() Node   { c := *n; return &c }
func (n *ListItem) shallowCopy() Node      { c := *n; return &c }
func (n *TaskList) shallowCopy() Node      { c := *n; return &c }
func (n *TaskItem) shallowCopy() Node      { c := *n; return &c }
func (n *DecisionList) shallowCopy() Node  { c := *n; return &c }
func (n *DecisionItem) shallowCopy() Node  { c := *n; return &c }
func (n *Table) shallowCopy() Node         { c := *n; return &c }
func (n *TableRow) shallowCopy() Node      { c := *n; return &c }
func (n *TableCell) shallowCopy() Node     { c := *n; return &c }
func (n *TableHeader) shallowCopy() Node   { c := *n; return &c }
func (n *Panel) shallowCopy() Node         { c := *n; return &c }
func (n *Expand) shallowCopy() Node        { c := *n; return &c }
func (n *NestedExpand) shallowCopy() Node  { c := *n; return &c }
func (n *MediaSingle) shallowCopy() Node   { c := *n; return &c }
func (n *MediaGroup) shallowCopy() Node    { c := *n; return &c }
func (n *Media) shallowCopy() Node         { c := *n; return &c }
func (n *BlockCard) shallowCopy() Node     { c := *n; return &c }
func (n *EmbedCard) shallowCopy() Node     { c := *n; return &c }
func (n *InlineCard) shallowCopy() Node    { c := *n; return &c }
func (n *Text) shallowCopy() Node          { c := *n; return &c }
func (n *HardBreak) shallowCopy() Node     { c := *n; return &c }
func (n *Emoji) shallowCopy() Node         { c := *n; return &c }
func (n *Mention) shallowCopy() Node       { c := *n; return &c }
func (n *Status) shallowCopy() Node        { c := *n; return &c }
func (n *MediaInline) shallowCopy() Node   { c := *n; return &c }
func (n *ColwidthsHint) shallowCopy() Node { c := *n; return &c }
func (n *RawNode) shallowCopy() Node       { c := *n; return &c }

func (n *Date) shallowCopy() Node                 { c := *n; return &c }
func (n *Placeholder) shallowCopy() Node          { c := *n; return &c }
func (n *Caption) shallowCopy() Node              { c := *n; return &c }
func (n *BlockTaskItem) shallowCopy() Node        { c := *n; return &c }
func (n *LayoutSection) shallowCopy() Node        { c := *n; return &c }
func (n *LayoutColumn) shallowCopy() Node         { c := *n; return &c }
func (n *Extension) shallowCopy() Node            { c := *n; return &c }
func (n *InlineExtension) shallowCopy() Node      { c := *n; return &c }
func (n *BodiedExtension) shallowCopy() Node      { c := *n; return &c }
func (n *MultiBodiedExtension) shallowCopy() Node { c := *n; return &c }
func (n *ExtensionFrame) shallowCopy() Node       { c := *n; return &c }
func (n *SyncBlock) shallowCopy() Node            { c := *n; return &c }
func (n *BodiedSyncBlock) shallowCopy() Node      { c := *n; return &c }
