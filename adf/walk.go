package adf

// Generic tree access over the typed node kinds: the slot view every
// walk-style helper (NormalizeTextNewlines, the jira submodule's doc
// rewrites, debug dumps) builds on.

// nodeSlots is a pointer view of a node's generic slots; nil pointers
// mean the kind has no such slot.
type nodeSlots struct {
	content *[]Node
	marks   *[]Mark
	text    *string
	extra   *map[string]any
}

func slotsOf(n Node) nodeSlots {
	if s, ok := containerSlots(n); ok {
		return s
	}
	return leafSlots(n)
}

// containerSlots covers the content-bearing kinds.
func containerSlots(n Node) (nodeSlots, bool) {
	switch t := n.(type) {
	case *Paragraph:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *Heading:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *Blockquote:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *CodeBlock:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *BulletList:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *OrderedList:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *ListItem:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *TaskList:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *TaskItem:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *DecisionList:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *DecisionItem:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *Table:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *TableRow:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *TableCell:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *TableHeader:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *Panel:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *Expand:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *NestedExpand:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *MediaSingle:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *MediaGroup:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *RawNode:
		return nodeSlots{content: &t.Content, marks: &t.Marks, text: &t.Text, extra: &t.Attrs}, true
	}
	return extendedContainerSlots(n)
}

// extendedContainerSlots covers the extended content-bearing kinds.
func extendedContainerSlots(n Node) (nodeSlots, bool) {
	switch t := n.(type) {
	case *Caption:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *BlockTaskItem:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *LayoutSection:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *LayoutColumn:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *BodiedExtension:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *MultiBodiedExtension:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *ExtensionFrame:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	case *BodiedSyncBlock:
		return nodeSlots{content: &t.Content, marks: &t.Marks, extra: &t.Extra}, true
	}
	return nodeSlots{}, false
}

// leafSlots covers the kinds without child content.
func leafSlots(n Node) nodeSlots {
	switch t := n.(type) {
	case *Rule:
		return nodeSlots{marks: &t.Marks, extra: &t.Extra}
	case *Media:
		return nodeSlots{marks: &t.Marks, extra: &t.Extra}
	case *BlockCard:
		return nodeSlots{marks: &t.Marks, extra: &t.Extra}
	case *EmbedCard:
		return nodeSlots{marks: &t.Marks, extra: &t.Extra}
	case *InlineCard:
		return nodeSlots{marks: &t.Marks, extra: &t.Extra}
	case *Text:
		return nodeSlots{marks: &t.Marks, text: &t.Text, extra: &t.Extra}
	case *HardBreak:
		return nodeSlots{marks: &t.Marks, extra: &t.Extra}
	case *Emoji:
		return nodeSlots{marks: &t.Marks, extra: &t.Extra}
	case *Mention:
		return nodeSlots{marks: &t.Marks, extra: &t.Extra}
	case *Status:
		return nodeSlots{marks: &t.Marks, extra: &t.Extra}
	case *MediaInline:
		return nodeSlots{marks: &t.Marks, extra: &t.Extra}
	case *ColwidthsHint:
		return nodeSlots{marks: &t.Marks, extra: &t.Extra}
	case *Date:
		return nodeSlots{marks: &t.Marks, extra: &t.Extra}
	case *Placeholder:
		return nodeSlots{marks: &t.Marks, extra: &t.Extra}
	case *Extension:
		return nodeSlots{marks: &t.Marks, extra: &t.Extra}
	case *InlineExtension:
		return nodeSlots{marks: &t.Marks, extra: &t.Extra}
	case *SyncBlock:
		return nodeSlots{marks: &t.Marks, extra: &t.Extra}
	}
	return nodeSlots{}
}

// NodeContent returns the node's child nodes (nil for leaf kinds).
func NodeContent(n Node) []Node {
	if s := slotsOf(n); s.content != nil {
		return *s.content
	}
	return nil
}

// NodeText returns the node's text value ("" for kinds without one).
func NodeText(n Node) string {
	if s := slotsOf(n); s.text != nil {
		return *s.text
	}
	return ""
}

// NodeMarks returns the node's marks.
func NodeMarks(n Node) []Mark {
	if s := slotsOf(n); s.marks != nil {
		return *s.marks
	}
	return nil
}

// WithContent returns a shallow copy of n with its content replaced;
// kinds without a content slot return n unchanged.
func WithContent(n Node, content []Node) Node {
	if c := copyContainer(n); c != nil {
		if s := slotsOf(c); s.content != nil {
			*s.content = content
		}
		return c
	}
	return n
}

// copyNode shallow-copies any node kind (containers and leaves; RawNode
// included). Foreign kinds are returned as-is.
func copyNode(n Node) Node {
	if c := copyContainer(n); c != nil {
		return c
	}
	if c := copyLeaf(n); c != nil {
		return c
	}
	return n
}

// copyLeaf shallow-copies a leaf node kind (nil otherwise).
func copyLeaf(n Node) Node {
	switch t := n.(type) {
	case *Rule:
		c := *t
		return &c
	case *Media:
		c := *t
		return &c
	case *BlockCard:
		c := *t
		return &c
	case *EmbedCard:
		c := *t
		return &c
	case *InlineCard:
		c := *t
		return &c
	case *Text:
		c := *t
		return &c
	case *HardBreak:
		c := *t
		return &c
	case *Emoji:
		c := *t
		return &c
	case *Mention:
		c := *t
		return &c
	case *Status:
		c := *t
		return &c
	case *MediaInline:
		c := *t
		return &c
	case *ColwidthsHint:
		c := *t
		return &c
	case *Date:
		c := *t
		return &c
	case *Placeholder:
		c := *t
		return &c
	case *Extension:
		c := *t
		return &c
	case *InlineExtension:
		c := *t
		return &c
	case *SyncBlock:
		c := *t
		return &c
	}
	return nil
}

// copyContainer shallow-copies a content-bearing node (nil otherwise).
func copyContainer(n Node) Node {
	if c := copyStructuralContainer(n); c != nil {
		return c
	}
	return copyWrapperContainer(n)
}

// copyStructuralContainer covers the text-flow and list/table kinds.
func copyStructuralContainer(n Node) Node {
	switch t := n.(type) {
	case *Paragraph:
		c := *t
		return &c
	case *Heading:
		c := *t
		return &c
	case *Blockquote:
		c := *t
		return &c
	case *CodeBlock:
		c := *t
		return &c
	case *BulletList:
		c := *t
		return &c
	case *OrderedList:
		c := *t
		return &c
	case *ListItem:
		c := *t
		return &c
	case *TaskList:
		c := *t
		return &c
	case *TaskItem:
		c := *t
		return &c
	case *DecisionList:
		c := *t
		return &c
	case *DecisionItem:
		c := *t
		return &c
	case *Table:
		c := *t
		return &c
	case *TableRow:
		c := *t
		return &c
	case *TableCell:
		c := *t
		return &c
	case *TableHeader:
		c := *t
		return &c
	}
	return nil
}

// copyWrapperContainer covers the wrapper and synthetic kinds.
func copyWrapperContainer(n Node) Node {
	switch t := n.(type) {
	case *Panel:
		c := *t
		return &c
	case *Expand:
		c := *t
		return &c
	case *NestedExpand:
		c := *t
		return &c
	case *MediaSingle:
		c := *t
		return &c
	case *MediaGroup:
		c := *t
		return &c
	case *RawNode:
		c := *t
		return &c
	case *Caption:
		c := *t
		return &c
	case *BlockTaskItem:
		c := *t
		return &c
	case *LayoutSection:
		c := *t
		return &c
	case *LayoutColumn:
		c := *t
		return &c
	case *BodiedExtension:
		c := *t
		return &c
	case *MultiBodiedExtension:
		c := *t
		return &c
	case *ExtensionFrame:
		c := *t
		return &c
	case *BodiedSyncBlock:
		c := *t
		return &c
	}
	return nil
}

// AddMarks appends marks to the node's mark list (a no-op for foreign
// node kinds).
func AddMarks(n Node, marks ...Mark) {
	if len(marks) == 0 {
		return
	}
	if s := slotsOf(n); s.marks != nil {
		*s.marks = append(*s.marks, marks...)
	}
}

// HasExtra reports whether the node's Extra map (RawNode: Attrs) holds
// the key — i.e. the wire document carried an attribute the typed
// fields do not model faithfully.
func HasExtra(n Node, key string) bool {
	s := slotsOf(n)
	if s.extra == nil || *s.extra == nil {
		return false
	}
	_, ok := (*s.extra)[key]
	return ok
}

// SetExtra stores an unmodeled attribute on the node (RawNode: Attrs);
// it participates in encoding like any other Extra entry.
func SetExtra(n Node, key string, value any) {
	s := slotsOf(n)
	if s.extra == nil {
		return
	}
	if *s.extra == nil {
		*s.extra = map[string]any{}
	}
	(*s.extra)[key] = value
}

// ExtraBool reads a bool Extra entry (false when absent or not a bool).
func ExtraBool(n Node, key string) bool {
	s := slotsOf(n)
	if s.extra == nil || *s.extra == nil {
		return false
	}
	v, ok := (*s.extra)[key].(bool)
	return ok && v
}
