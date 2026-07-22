package adf

// The known mark kinds. The same field ↔ attribute rules as nodes.go
// apply: plain fields encode iff non-zero (zero-but-present decoded
// values stay in Extra), pointer fields model presence-sensitive
// attributes, and Extra always wins on encode.

// Strong is the ADF strong (bold) mark.
type Strong struct {
	Extra map[string]any
}

// Em is the ADF em (italic) mark.
type Em struct {
	Extra map[string]any
}

// Strike is the ADF strike (strikethrough) mark.
type Strike struct {
	Extra map[string]any
}

// Code is the ADF code (inline code) mark; it is exclusive in ADF —
// strong/em/strike are stripped from code-marked text.
type Code struct {
	Extra map[string]any
}

// Underline is the ADF underline mark.
type Underline struct {
	Extra map[string]any
}

// Link is the ADF link mark. Href is a pointer because a link mark
// without href is not a markdown link (while an empty-but-present href
// keeps its mark, like remark's [x]()).
type Link struct {
	Href  *string
	Extra map[string]any
}

// TextColor is the ADF textColor mark; Color is the hex value.
type TextColor struct {
	Extra map[string]any
	Color string
}

// BackgroundColor is the ADF backgroundColor mark.
type BackgroundColor struct {
	Extra map[string]any
	Color string
}

// SubSup is the ADF subsup mark; Type is "sub" or "sup" (anything else,
// including absent, projects as "sub", matching the measured behavior).
type SubSup struct {
	Extra map[string]any
	Type  string
}

// RawMark is the lossless escape hatch for unknown mark kinds.
type RawMark struct {
	Attrs map[string]any
	Type  string
}

// Kind implements Mark.
func (*Strong) Kind() string { return "strong" }

// Kind implements Mark.
func (*Em) Kind() string { return "em" }

// Kind implements Mark.
func (*Strike) Kind() string { return "strike" }

// Kind implements Mark.
func (*Code) Kind() string { return "code" }

// Kind implements Mark.
func (*Underline) Kind() string { return "underline" }

// Kind implements Mark.
func (*Link) Kind() string { return "link" }

// Kind implements Mark.
func (*TextColor) Kind() string { return "textColor" }

// Kind implements Mark.
func (*BackgroundColor) Kind() string { return "backgroundColor" }

// Kind implements Mark.
func (*SubSup) Kind() string { return "subsup" }

// Kind implements Mark.
func (m *RawMark) Kind() string { return m.Type }

func (*Strong) adfMark()          {}
func (*Em) adfMark()              {}
func (*Strike) adfMark()          {}
func (*Code) adfMark()            {}
func (*Underline) adfMark()       {}
func (*Link) adfMark()            {}
func (*TextColor) adfMark()       {}
func (*BackgroundColor) adfMark() {}
func (*SubSup) adfMark()          {}
func (*RawMark) adfMark()         {}

// HasMark reports whether marks contains a mark of the given kind.
func HasMark(marks []Mark, kind string) bool {
	for _, m := range marks {
		if m.Kind() == kind {
			return true
		}
	}
	return false
}

// FindMark returns the first mark of type M, or (zero, false).
func FindMark[M Mark](marks []Mark) (M, bool) {
	for _, m := range marks {
		if typed, ok := m.(M); ok {
			return typed, true
		}
	}
	var zero M
	return zero, false
}
