package adf

// Generic tree access over the typed node kinds: the helpers every
// walk-style caller (NormalizeTextNewlines, the jira submodule's doc
// rewrites, debug dumps) builds on, written against the per-kind slot
// view slots.go declares.

// NodeContent returns the node's child nodes (nil for leaf kinds).
func NodeContent(n Node) []Node {
	if s := n.slots(); s.content != nil {
		return *s.content
	}
	return nil
}

// NodeText returns the node's text value ("" for kinds without one).
func NodeText(n Node) string {
	if s := n.slots(); s.text != nil {
		return *s.text
	}
	return ""
}

// NodeMarks returns the node's marks.
func NodeMarks(n Node) []Mark {
	if s := n.slots(); s.marks != nil {
		return *s.marks
	}
	return nil
}

// WithContent returns a shallow copy of n with its content replaced;
// kinds without a content slot return n unchanged.
func WithContent(n Node, content []Node) Node {
	if n.slots().content == nil {
		return n
	}
	c := n.shallowCopy()
	*c.slots().content = content
	return c
}

// AddMarks appends marks to the node's mark list (a no-op for the kinds
// without a marks slot).
func AddMarks(n Node, marks ...Mark) {
	if len(marks) == 0 {
		return
	}
	if s := n.slots(); s.marks != nil {
		*s.marks = append(*s.marks, marks...)
	}
}

// HasExtra reports whether the node's Extra map (RawNode: Attrs) holds
// the key — i.e. the wire document carried an attribute the typed
// fields do not model faithfully.
func HasExtra(n Node, key string) bool {
	s := n.slots()
	if s.extra == nil || *s.extra == nil {
		return false
	}
	_, ok := (*s.extra)[key]
	return ok
}

// SetExtra stores an unmodeled attribute on the node (RawNode: Attrs);
// it participates in encoding like any other Extra entry.
func SetExtra(n Node, key string, value any) {
	s := n.slots()
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
	s := n.slots()
	if s.extra == nil || *s.extra == nil {
		return false
	}
	v, ok := (*s.extra)[key].(bool)
	return ok && v
}
