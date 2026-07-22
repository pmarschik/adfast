package adf

// The extended mark kinds: block marks (alignment, indentation,
// breakout, dataConsumer, fragment), media borders, inline comments
// (annotation), and fontSize. Same field ↔ attribute rules as marks.go.

// Alignment is the ADF alignment block mark on paragraphs/headings;
// Align is "center" or "end".
type Alignment struct {
	Extra map[string]any
	Align string
}

// Indentation is the ADF indentation block mark on paragraphs/headings;
// Level is 1–6.
type Indentation struct {
	Extra map[string]any
	Level int
}

// Breakout is the ADF breakout block mark (wide/full-width rendering on
// codeBlocks, expands, and layoutSections); Width is the optional pixel
// width.
type Breakout struct {
	Width *float64
	Extra map[string]any
	Mode  string
}

// Border is the ADF border mark on media; Size is 1–3, Color a hex
// value.
type Border struct {
	Extra map[string]any
	Color string
	Size  int
}

// Annotation is the ADF annotation mark (inline comments). Confluence
// inline-comment threads are anchored by these marks: submitting a body
// without them orphans the threads (they are not re-inserted
// server-side), so the markdown mapping exists precisely to let
// annotated text travel through markdown edits without severing the
// threads.
type Annotation struct {
	Extra          map[string]any
	ID             string
	AnnotationType string
}

// DataConsumer is the ADF dataConsumer block mark: it lets extensions
// consume data from fragment-marked nodes; Sources lists the fragment
// localIds.
type DataConsumer struct {
	Extra   map[string]any
	Sources []string
}

// Fragment is the ADF fragment block mark: a stable reference to a
// table or extension.
type Fragment struct {
	Extra   map[string]any
	LocalID string
	Name    string
}

// FontSize is the ADF fontSize mark; Size is the "fontSize" attribute
// (the schema currently enumerates "small").
type FontSize struct {
	Extra map[string]any
	Size  string
}

// Kind implements Mark.
func (*Alignment) Kind() string { return "alignment" }

// Kind implements Mark.
func (*Indentation) Kind() string { return "indentation" }

// Kind implements Mark.
func (*Breakout) Kind() string { return "breakout" }

// Kind implements Mark.
func (*Border) Kind() string { return "border" }

// Kind implements Mark.
func (*Annotation) Kind() string { return "annotation" }

// Kind implements Mark.
func (*DataConsumer) Kind() string { return "dataConsumer" }

// Kind implements Mark.
func (*Fragment) Kind() string { return "fragment" }

// Kind implements Mark.
func (*FontSize) Kind() string { return "fontSize" }

func (*Alignment) adfMark()    {}
func (*Indentation) adfMark()  {}
func (*Breakout) adfMark()     {}
func (*Border) adfMark()       {}
func (*Annotation) adfMark()   {}
func (*DataConsumer) adfMark() {}
func (*Fragment) adfMark()     {}
func (*FontSize) adfMark()     {}

// MarshalJSON implements json.Marshaler.
func (m *Alignment) MarshalJSON() ([]byte, error) { return marshalMark(m) }

// MarshalJSON implements json.Marshaler.
func (m *Indentation) MarshalJSON() ([]byte, error) { return marshalMark(m) }

// MarshalJSON implements json.Marshaler.
func (m *Breakout) MarshalJSON() ([]byte, error) { return marshalMark(m) }

// MarshalJSON implements json.Marshaler.
func (m *Border) MarshalJSON() ([]byte, error) { return marshalMark(m) }

// MarshalJSON implements json.Marshaler.
func (m *Annotation) MarshalJSON() ([]byte, error) { return marshalMark(m) }

// MarshalJSON implements json.Marshaler.
func (m *DataConsumer) MarshalJSON() ([]byte, error) { return marshalMark(m) }

// MarshalJSON implements json.Marshaler.
func (m *Fragment) MarshalJSON() ([]byte, error) { return marshalMark(m) }

// MarshalJSON implements json.Marshaler.
func (m *FontSize) MarshalJSON() ([]byte, error) { return marshalMark(m) }
