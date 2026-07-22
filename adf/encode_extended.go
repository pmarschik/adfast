package adf

// The encode half for the extended kinds (see nodes_extended.go and
// marks_extended.go): typed fields recombine with Extra exactly like
// encode.go's switches.

// extendedNodeAttrs fills a with the extended kinds' typed attributes
// (ok is false when n is not one of them).
func extendedNodeAttrs(n Node, a attrs) (map[string]any, bool) {
	var extra map[string]any
	switch t := n.(type) {
	case *Date:
		a.str("timestamp", t.Timestamp)
		a.str("localId", t.LocalID)
		extra = t.Extra
	case *Placeholder:
		a.str("text", t.Text)
		a.str("localId", t.LocalID)
		extra = t.Extra
	case *Caption:
		a.str("localId", t.LocalID)
		extra = t.Extra
	case *BlockTaskItem:
		a.strPtr("localId", t.LocalID)
		a.str("state", t.State)
		extra = t.Extra
	case *LayoutSection:
		a.str("localId", t.LocalID)
		a.str("columnRuleStyle", t.ColumnRuleStyle)
		extra = t.Extra
	case *LayoutColumn:
		a.floatPtr("width", t.Width)
		a.str("localId", t.LocalID)
		a.str("valign", t.VAlign)
		extra = t.Extra
	case *Extension:
		extensionAttrs(a, t.ExtensionType, t.ExtensionKey, t.Parameters, t.Text, t.Layout, t.LocalID)
		extra = t.Extra
	case *InlineExtension:
		extensionAttrs(a, t.ExtensionType, t.ExtensionKey, t.Parameters, t.Text, "", t.LocalID)
		extra = t.Extra
	case *BodiedExtension:
		extensionAttrs(a, t.ExtensionType, t.ExtensionKey, t.Parameters, t.Text, t.Layout, t.LocalID)
		extra = t.Extra
	case *MultiBodiedExtension:
		extensionAttrs(a, t.ExtensionType, t.ExtensionKey, t.Parameters, t.Text, t.Layout, t.LocalID)
		extra = t.Extra
	case *ExtensionFrame:
		extra = t.Extra
	case *SyncBlock:
		a.str("resourceId", t.ResourceID)
		a.str("localId", t.LocalID)
		extra = t.Extra
	case *BodiedSyncBlock:
		a.str("resourceId", t.ResourceID)
		a.str("localId", t.LocalID)
		extra = t.Extra
	default:
		return nil, false
	}
	return extra, true
}

// extensionAttrs fills the shared extension-family attributes.
func extensionAttrs(a attrs, extensionType, extensionKey string, parameters any, text, layout, localID string) {
	a.str("extensionType", extensionType)
	a.str("extensionKey", extensionKey)
	a.rawAny("parameters", parameters)
	a.str("text", text)
	a.str("layout", layout)
	a.str("localId", localID)
}

// extendedMarkAttrs fills a with the extended mark kinds' typed
// attributes (nil extra for foreign marks).
func extendedMarkAttrs(m Mark, a attrs) map[string]any {
	var extra map[string]any
	switch t := m.(type) {
	case *Alignment:
		a.str("align", t.Align)
		extra = t.Extra
	case *Indentation:
		a.num("level", t.Level)
		extra = t.Extra
	case *Breakout:
		a.str("mode", t.Mode)
		a.floatPtr("width", t.Width)
		extra = t.Extra
	case *Border:
		a.str("color", t.Color)
		a.num("size", t.Size)
		extra = t.Extra
	case *Annotation:
		a.str("id", t.ID)
		a.str("annotationType", t.AnnotationType)
		extra = t.Extra
	case *DataConsumer:
		a.strs("sources", t.Sources)
		extra = t.Extra
	case *Fragment:
		a.str("localId", t.LocalID)
		a.str("name", t.Name)
		extra = t.Extra
	case *FontSize:
		a.str("fontSize", t.Size)
		extra = t.Extra
	}
	return extra
}
