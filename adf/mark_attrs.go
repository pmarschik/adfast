package adf

// The per-kind halves of the mark codec: writeAttrs is the encoder's
// (typed fields into the wire attribute map, Extra returned as the
// overlay), setExtra the decoder's (where the unmodeled attributes
// land). MarkAttrs (encode.go) and decoder.mark (decode.go) hold the
// generic side.
//
// Methods on Mark rather than type switches, for the reason node_attrs.go
// gives: the interface is sealed, so a mark kind added to marks.go without
// them does not compile, where the switches used to drop an unlisted
// kind's attributes without a word.

// ---------------------------------------------------------------------------
// writeAttrs — the marks that style a run of text
// ---------------------------------------------------------------------------

func (m *Strong) writeAttrs(attrs) map[string]any    { return m.Extra }
func (m *Em) writeAttrs(attrs) map[string]any        { return m.Extra }
func (m *Strike) writeAttrs(attrs) map[string]any    { return m.Extra }
func (m *Code) writeAttrs(attrs) map[string]any      { return m.Extra }
func (m *Underline) writeAttrs(attrs) map[string]any { return m.Extra }

func (m *Link) writeAttrs(a attrs) map[string]any {
	a.strPtr("href", m.Href)
	return m.Extra
}

func (m *TextColor) writeAttrs(a attrs) map[string]any {
	a.str("color", m.Color)
	return m.Extra
}

func (m *BackgroundColor) writeAttrs(a attrs) map[string]any {
	a.str("color", m.Color)
	return m.Extra
}

func (m *SubSup) writeAttrs(a attrs) map[string]any {
	a.str("type", m.Type)
	return m.Extra
}

func (m *Annotation) writeAttrs(a attrs) map[string]any {
	a.str("id", m.ID)
	a.str("annotationType", m.AnnotationType)
	return m.Extra
}

func (m *FontSize) writeAttrs(a attrs) map[string]any {
	a.str("fontSize", m.Size)
	return m.Extra
}

// ---------------------------------------------------------------------------
// writeAttrs — the marks that decorate a whole block
// ---------------------------------------------------------------------------

func (m *Alignment) writeAttrs(a attrs) map[string]any {
	a.str("align", m.Align)
	return m.Extra
}

func (m *Indentation) writeAttrs(a attrs) map[string]any {
	a.num("level", m.Level)
	return m.Extra
}

func (m *Breakout) writeAttrs(a attrs) map[string]any {
	a.str("mode", m.Mode)
	a.floatPtr("width", m.Width)
	return m.Extra
}

func (m *Border) writeAttrs(a attrs) map[string]any {
	a.str("color", m.Color)
	a.num("size", m.Size)
	return m.Extra
}

func (m *DataConsumer) writeAttrs(a attrs) map[string]any {
	a.strs("sources", m.Sources)
	return m.Extra
}

func (m *Fragment) writeAttrs(a attrs) map[string]any {
	a.str("localId", m.LocalID)
	a.str("name", m.Name)
	return m.Extra
}

// RawMark keeps the wire attributes verbatim; MarkAttrs short-circuits to
// them so the caller sees the same map, and this method only exists to
// satisfy the interface.
func (m *RawMark) writeAttrs(attrs) map[string]any { return m.Attrs }

// ---------------------------------------------------------------------------
// setExtra
// ---------------------------------------------------------------------------

// setExtra is the mark counterpart of a node's slots().extra: it takes
// every attribute the typed fields do not model, which the decoder parks
// there so re-encoding stays lossless.

func (m *Strong) setExtra(e map[string]any)          { m.Extra = e }
func (m *Em) setExtra(e map[string]any)              { m.Extra = e }
func (m *Strike) setExtra(e map[string]any)          { m.Extra = e }
func (m *Code) setExtra(e map[string]any)            { m.Extra = e }
func (m *Underline) setExtra(e map[string]any)       { m.Extra = e }
func (m *Link) setExtra(e map[string]any)            { m.Extra = e }
func (m *TextColor) setExtra(e map[string]any)       { m.Extra = e }
func (m *BackgroundColor) setExtra(e map[string]any) { m.Extra = e }
func (m *SubSup) setExtra(e map[string]any)          { m.Extra = e }
func (m *Annotation) setExtra(e map[string]any)      { m.Extra = e }
func (m *FontSize) setExtra(e map[string]any)        { m.Extra = e }
func (m *Alignment) setExtra(e map[string]any)       { m.Extra = e }
func (m *Indentation) setExtra(e map[string]any)     { m.Extra = e }
func (m *Breakout) setExtra(e map[string]any)        { m.Extra = e }
func (m *Border) setExtra(e map[string]any)          { m.Extra = e }
func (m *DataConsumer) setExtra(e map[string]any)    { m.Extra = e }
func (m *Fragment) setExtra(e map[string]any)        { m.Extra = e }

// RawMark spells its attribute map Attrs and keeps it verbatim, so its
// "extra" is the whole map.
func (m *RawMark) setExtra(e map[string]any) { m.Attrs = e }
