package adf

// The per-kind half of the encoder: each node kind writes its own typed
// fields into the wire attribute map and hands back its Extra overlay.
// NodeAttrs (encode.go) is the whole of the generic side.
//
// These are methods on Node rather than a type switch for the same reason
// slots() is: the interface is sealed, so a kind added to nodes.go without
// a writeAttrs method does not compile. The switch version answered a
// missing kind with an empty attribute map, which dropped every one of
// that kind's attributes from the encoded document in silence.

// ---------------------------------------------------------------------------
// Text flow
// ---------------------------------------------------------------------------

func (n *Paragraph) writeAttrs(attrs) map[string]any  { return n.Extra }
func (n *Blockquote) writeAttrs(attrs) map[string]any { return n.Extra }
func (n *Rule) writeAttrs(attrs) map[string]any       { return n.Extra }
func (n *HardBreak) writeAttrs(attrs) map[string]any  { return n.Extra }
func (n *Text) writeAttrs(attrs) map[string]any       { return n.Extra }

func (n *Heading) writeAttrs(a attrs) map[string]any {
	a.num("level", n.Level)
	a.str("anchor", n.Anchor)
	return n.Extra
}

func (n *CodeBlock) writeAttrs(a attrs) map[string]any {
	a.str("language", n.Language)
	return n.Extra
}

// ---------------------------------------------------------------------------
// Lists
// ---------------------------------------------------------------------------

func (n *ListItem) writeAttrs(attrs) map[string]any { return n.Extra }

func (n *BulletList) writeAttrs(a attrs) map[string]any {
	a.boolPtr("tight", n.Tight)
	return n.Extra
}

func (n *OrderedList) writeAttrs(a attrs) map[string]any {
	a.numPtr("order", n.Order)
	a.boolPtr("tight", n.Tight)
	return n.Extra
}

func (n *TaskList) writeAttrs(a attrs) map[string]any {
	a.strPtr("localId", n.LocalID)
	return n.Extra
}

func (n *TaskItem) writeAttrs(a attrs) map[string]any {
	a.strPtr("localId", n.LocalID)
	a.str("state", n.State)
	return n.Extra
}

func (n *BlockTaskItem) writeAttrs(a attrs) map[string]any {
	a.strPtr("localId", n.LocalID)
	a.str("state", n.State)
	return n.Extra
}

func (n *DecisionList) writeAttrs(a attrs) map[string]any {
	a.strPtr("localId", n.LocalID)
	return n.Extra
}

func (n *DecisionItem) writeAttrs(a attrs) map[string]any {
	a.strPtr("localId", n.LocalID)
	a.str("state", n.State)
	return n.Extra
}

// ---------------------------------------------------------------------------
// Tables
// ---------------------------------------------------------------------------

func (n *TableRow) writeAttrs(attrs) map[string]any { return n.Extra }

func (n *Table) writeAttrs(a attrs) map[string]any {
	a.strs("align", n.Align)
	a.str("layout", n.Layout)
	a.floatPtr("width", n.Width)
	a.boolPtr("isNumberColumnEnabled", n.IsNumberColumnEnabled)
	a.str("localId", n.LocalID)
	a.str("displayMode", n.DisplayMode)
	return n.Extra
}

func (n *TableCell) writeAttrs(a attrs) map[string]any {
	writeCellAttrs(a, n.Colspan, n.Rowspan, n.Colwidth)
	return n.Extra
}

func (n *TableHeader) writeAttrs(a attrs) map[string]any {
	writeCellAttrs(a, n.Colspan, n.Rowspan, n.Colwidth)
	return n.Extra
}

// writeCellAttrs fills the attributes a data cell and a header cell share.
func writeCellAttrs(a attrs, colspan, rowspan int, colwidth []float64) {
	a.num("colspan", colspan)
	a.num("rowspan", rowspan)
	a.floats("colwidth", colwidth)
}

// ---------------------------------------------------------------------------
// Wrappers and layout
// ---------------------------------------------------------------------------

func (n *ExtensionFrame) writeAttrs(attrs) map[string]any { return n.Extra }

func (n *Panel) writeAttrs(a attrs) map[string]any {
	a.str("panelType", n.PanelType)
	return n.Extra
}

func (n *Expand) writeAttrs(a attrs) map[string]any {
	a.strPtr("title", n.Title)
	return n.Extra
}

func (n *NestedExpand) writeAttrs(a attrs) map[string]any {
	a.strPtr("title", n.Title)
	return n.Extra
}

func (n *Caption) writeAttrs(a attrs) map[string]any {
	a.str("localId", n.LocalID)
	return n.Extra
}

func (n *LayoutSection) writeAttrs(a attrs) map[string]any {
	a.str("localId", n.LocalID)
	a.str("columnRuleStyle", n.ColumnRuleStyle)
	return n.Extra
}

func (n *LayoutColumn) writeAttrs(a attrs) map[string]any {
	a.floatPtr("width", n.Width)
	a.str("localId", n.LocalID)
	a.str("valign", n.VAlign)
	return n.Extra
}

// ---------------------------------------------------------------------------
// Media and cards
// ---------------------------------------------------------------------------

func (n *MediaGroup) writeAttrs(attrs) map[string]any { return n.Extra }

func (n *MediaSingle) writeAttrs(a attrs) map[string]any {
	a.strPtr("layout", n.Layout)
	a.floatPtr("width", n.Width)
	a.strPtr("widthType", n.WidthType)
	return n.Extra
}

func (n *Media) writeAttrs(a attrs) map[string]any {
	a.str("type", n.Type)
	a.str("id", n.ID)
	a.str("url", n.URL)
	a.str("alt", n.Alt)
	a.strPtr("collection", n.Collection)
	a.floatPtr("width", n.Width)
	a.floatPtr("height", n.Height)
	a.strPtr("occurrenceKey", n.OccurrenceKey)
	return n.Extra
}

func (n *MediaInline) writeAttrs(a attrs) map[string]any {
	a.str("type", n.Type)
	a.str("id", n.ID)
	a.str("alt", n.Alt)
	a.strPtr("collection", n.Collection)
	return n.Extra
}

func (n *BlockCard) writeAttrs(a attrs) map[string]any {
	a.str("url", n.URL)
	a.rawMap("datasource", n.Datasource)
	return n.Extra
}

func (n *EmbedCard) writeAttrs(a attrs) map[string]any {
	a.str("url", n.URL)
	a.str("layout", n.Layout)
	a.floatPtr("width", n.Width)
	return n.Extra
}

func (n *InlineCard) writeAttrs(a attrs) map[string]any {
	a.strPtr("url", n.URL)
	return n.Extra
}

// ---------------------------------------------------------------------------
// Inline atoms
// ---------------------------------------------------------------------------

func (n *Emoji) writeAttrs(a attrs) map[string]any {
	a.str("shortName", n.ShortName)
	a.str("id", n.ID)
	a.strPtr("text", n.Text)
	return n.Extra
}

func (n *Mention) writeAttrs(a attrs) map[string]any {
	a.str("id", n.ID)
	a.strPtr("text", n.Text)
	a.str("accessLevel", n.AccessLevel)
	return n.Extra
}

func (n *Status) writeAttrs(a attrs) map[string]any {
	a.strPtr("text", n.Text)
	a.str("color", n.Color)
	a.str("style", n.Style)
	a.str("localId", n.LocalID)
	return n.Extra
}

func (n *Date) writeAttrs(a attrs) map[string]any {
	a.str("timestamp", n.Timestamp)
	a.str("localId", n.LocalID)
	return n.Extra
}

func (n *Placeholder) writeAttrs(a attrs) map[string]any {
	a.str("text", n.Text)
	a.str("localId", n.LocalID)
	return n.Extra
}

func (n *ColwidthsHint) writeAttrs(a attrs) map[string]any {
	a.floats("widths", n.Widths)
	return n.Extra
}

// ---------------------------------------------------------------------------
// Extension points
// ---------------------------------------------------------------------------

func (n *Extension) writeAttrs(a attrs) map[string]any {
	extensionAttrs(a, n.ExtensionType, n.ExtensionKey, n.Parameters, n.Text, n.Layout, n.LocalID)
	return n.Extra
}

func (n *InlineExtension) writeAttrs(a attrs) map[string]any {
	extensionAttrs(a, n.ExtensionType, n.ExtensionKey, n.Parameters, n.Text, "", n.LocalID)
	return n.Extra
}

func (n *BodiedExtension) writeAttrs(a attrs) map[string]any {
	extensionAttrs(a, n.ExtensionType, n.ExtensionKey, n.Parameters, n.Text, n.Layout, n.LocalID)
	return n.Extra
}

func (n *MultiBodiedExtension) writeAttrs(a attrs) map[string]any {
	extensionAttrs(a, n.ExtensionType, n.ExtensionKey, n.Parameters, n.Text, n.Layout, n.LocalID)
	return n.Extra
}

func (n *SyncBlock) writeAttrs(a attrs) map[string]any {
	a.str("resourceId", n.ResourceID)
	a.str("localId", n.LocalID)
	return n.Extra
}

func (n *BodiedSyncBlock) writeAttrs(a attrs) map[string]any {
	a.str("resourceId", n.ResourceID)
	a.str("localId", n.LocalID)
	return n.Extra
}

// RawNode keeps the wire attributes verbatim; NodeAttrs short-circuits to
// them so the caller sees the same map, and this method only exists to
// satisfy the interface.
func (n *RawNode) writeAttrs(attrs) map[string]any { return n.Attrs }
