package adf

import (
	"encoding/json"
	"maps"
)

// The encode half of the JSON codec: every Node/Mark marshals through
// its wire form — the generic {type, attrs, marks, text, content} shape
// with attrs recombined from the typed fields plus Extra (Extra entries
// win). Map-backed attrs marshal with sorted keys, so the output is
// byte-stable and matches what the pre-typed generic model produced.

// wireNode is the generic JSON shape of a node (field order matters:
// it fixes the serialized key order).
type wireNode struct {
	Type    string         `json:"type"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Marks   []Mark         `json:"marks,omitempty"`
	Text    string         `json:"text,omitempty"`
	Content []Node         `json:"content,omitempty"`
}

// wireMark is the generic JSON shape of a mark (attrs before type,
// matching the historical field order).
type wireMark struct {
	Attrs map[string]any `json:"attrs,omitempty"`
	Type  string         `json:"type"`
}

// attrs is the wire attribute map under construction; the put* helpers
// implement the field ↔ attribute rules documented in nodes.go.
type attrs map[string]any

func (a attrs) str(k, v string) {
	if v != "" {
		a[k] = v
	}
}

func (a attrs) strPtr(k string, v *string) {
	if v != nil {
		a[k] = *v
	}
}

func (a attrs) num(k string, v int) {
	if v != 0 {
		a[k] = v
	}
}

func (a attrs) numPtr(k string, v *int) {
	if v != nil {
		a[k] = *v
	}
}

func (a attrs) floatPtr(k string, v *float64) {
	if v != nil {
		a[k] = *v
	}
}

func (a attrs) boolPtr(k string, v *bool) {
	if v != nil {
		a[k] = *v
	}
}

func (a attrs) floats(k string, v []float64) {
	if v == nil {
		return
	}
	vals := make([]any, len(v))
	for i, f := range v {
		vals[i] = f
	}
	a[k] = vals
}

func (a attrs) rawMap(k string, v map[string]any) {
	if v != nil {
		a[k] = v
	}
}

func (a attrs) strs(k string, v []string) {
	if v == nil {
		return
	}
	vals := make([]any, len(v))
	for i, s := range v {
		vals[i] = s
	}
	a[k] = vals
}

func (a attrs) rawAny(k string, v any) {
	if v != nil {
		a[k] = v
	}
}

// finish overlays extra (which wins) and normalizes empty to nil.
func (a attrs) finish(extra map[string]any) map[string]any {
	maps.Copy(a, extra)
	if len(a) == 0 {
		return nil
	}
	return a
}

// NodeAttrs returns the node's wire attribute map: the typed fields
// recombined with Extra (nil when there are none). RawNode returns its
// Attrs verbatim.
func NodeAttrs(n Node) map[string]any {
	if raw, ok := n.(*RawNode); ok {
		if len(raw.Attrs) == 0 {
			return nil
		}
		return raw.Attrs
	}
	a := attrs{}
	extra, ok := blockNodeAttrs(n, a)
	if !ok {
		extra, ok = extendedNodeAttrs(n, a)
	}
	if !ok {
		extra = leafNodeAttrs(n, a)
	}
	return a.finish(extra)
}

// blockNodeAttrs fills a with the block kinds' typed attributes.
func blockNodeAttrs(n Node, a attrs) (map[string]any, bool) {
	var extra map[string]any
	switch t := n.(type) {
	case *Paragraph:
		extra = t.Extra
	case *Heading:
		a.num("level", t.Level)
		a.str("anchor", t.Anchor)
		extra = t.Extra
	case *Blockquote:
		extra = t.Extra
	case *Rule:
		extra = t.Extra
	case *CodeBlock:
		a.str("language", t.Language)
		extra = t.Extra
	case *BulletList:
		a.boolPtr("tight", t.Tight)
		extra = t.Extra
	case *OrderedList:
		a.numPtr("order", t.Order)
		a.boolPtr("tight", t.Tight)
		extra = t.Extra
	case *ListItem:
		extra = t.Extra
	case *TaskList:
		a.strPtr("localId", t.LocalID)
		extra = t.Extra
	case *TaskItem:
		a.strPtr("localId", t.LocalID)
		a.str("state", t.State)
		extra = t.Extra
	case *DecisionList:
		a.strPtr("localId", t.LocalID)
		extra = t.Extra
	case *DecisionItem:
		a.strPtr("localId", t.LocalID)
		a.str("state", t.State)
		extra = t.Extra
	case *Table:
		a.strs("align", t.Align)
		a.str("layout", t.Layout)
		a.floatPtr("width", t.Width)
		a.boolPtr("isNumberColumnEnabled", t.IsNumberColumnEnabled)
		a.str("localId", t.LocalID)
		a.str("displayMode", t.DisplayMode)
		extra = t.Extra
	case *TableRow:
		extra = t.Extra
	case *TableCell:
		a.num("colspan", t.Colspan)
		a.num("rowspan", t.Rowspan)
		a.floats("colwidth", t.Colwidth)
		extra = t.Extra
	case *TableHeader:
		a.num("colspan", t.Colspan)
		a.num("rowspan", t.Rowspan)
		a.floats("colwidth", t.Colwidth)
		extra = t.Extra
	case *Panel:
		a.str("panelType", t.PanelType)
		extra = t.Extra
	case *Expand:
		a.strPtr("title", t.Title)
		extra = t.Extra
	case *NestedExpand:
		a.strPtr("title", t.Title)
		extra = t.Extra
	default:
		return nil, false
	}
	return extra, true
}

// leafNodeAttrs fills a with the leaf, media, and synthetic kinds'
// typed attributes.
func leafNodeAttrs(n Node, a attrs) map[string]any {
	var extra map[string]any
	switch t := n.(type) {
	case *MediaSingle:
		a.strPtr("layout", t.Layout)
		a.floatPtr("width", t.Width)
		a.strPtr("widthType", t.WidthType)
		extra = t.Extra
	case *MediaGroup:
		extra = t.Extra
	case *Media:
		a.str("type", t.Type)
		a.str("id", t.ID)
		a.str("url", t.URL)
		a.str("alt", t.Alt)
		a.strPtr("collection", t.Collection)
		a.floatPtr("width", t.Width)
		a.floatPtr("height", t.Height)
		a.strPtr("occurrenceKey", t.OccurrenceKey)
		extra = t.Extra
	case *BlockCard:
		a.str("url", t.URL)
		a.rawMap("datasource", t.Datasource)
		extra = t.Extra
	case *EmbedCard:
		a.str("url", t.URL)
		a.str("layout", t.Layout)
		a.floatPtr("width", t.Width)
		extra = t.Extra
	case *InlineCard:
		a.strPtr("url", t.URL)
		extra = t.Extra
	case *Text:
		extra = t.Extra
	case *HardBreak:
		extra = t.Extra
	case *Emoji:
		a.str("shortName", t.ShortName)
		a.str("id", t.ID)
		a.strPtr("text", t.Text)
		extra = t.Extra
	case *Mention:
		a.str("id", t.ID)
		a.strPtr("text", t.Text)
		a.str("accessLevel", t.AccessLevel)
		extra = t.Extra
	case *Status:
		a.strPtr("text", t.Text)
		a.str("color", t.Color)
		a.str("style", t.Style)
		a.str("localId", t.LocalID)
		extra = t.Extra
	case *MediaInline:
		a.str("type", t.Type)
		a.str("id", t.ID)
		a.str("alt", t.Alt)
		a.strPtr("collection", t.Collection)
		extra = t.Extra
	case *ColwidthsHint:
		a.floats("widths", t.Widths)
		extra = t.Extra
	}
	return extra
}

// MarkAttrs returns the mark's wire attribute map (nil when there are
// none). RawMark returns its Attrs verbatim.
func MarkAttrs(m Mark) map[string]any {
	if raw, ok := m.(*RawMark); ok {
		if len(raw.Attrs) == 0 {
			return nil
		}
		return raw.Attrs
	}
	a := attrs{}
	var extra map[string]any
	switch t := m.(type) {
	case *Strong:
		extra = t.Extra
	case *Em:
		extra = t.Extra
	case *Strike:
		extra = t.Extra
	case *Code:
		extra = t.Extra
	case *Underline:
		extra = t.Extra
	case *Link:
		a.strPtr("href", t.Href)
		extra = t.Extra
	case *TextColor:
		a.str("color", t.Color)
		extra = t.Extra
	case *BackgroundColor:
		a.str("color", t.Color)
		extra = t.Extra
	case *SubSup:
		a.str("type", t.Type)
		extra = t.Extra
	default:
		extra = extendedMarkAttrs(m, a)
	}
	return a.finish(extra)
}

// marshalNode serializes a node through its wire form.
func marshalNode(n Node) ([]byte, error) {
	return json.Marshal(wireNode{
		Type:    n.Kind(),
		Attrs:   NodeAttrs(n),
		Marks:   NodeMarks(n),
		Text:    NodeText(n),
		Content: NodeContent(n),
	})
}

// marshalMark serializes a mark through its wire form.
func marshalMark(m Mark) ([]byte, error) {
	return json.Marshal(wireMark{Type: m.Kind(), Attrs: MarkAttrs(m)})
}

// MarshalJSON implements json.Marshaler.
func (n *Paragraph) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *Heading) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *Blockquote) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *Rule) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *CodeBlock) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *BulletList) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *OrderedList) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *ListItem) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *TaskList) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *TaskItem) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *DecisionList) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *DecisionItem) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *Table) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *TableRow) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *TableCell) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *TableHeader) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *Panel) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *Expand) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *NestedExpand) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *MediaSingle) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *MediaGroup) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *Media) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *BlockCard) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *EmbedCard) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *InlineCard) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *Text) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *HardBreak) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *Emoji) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *Mention) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *Status) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *MediaInline) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *ColwidthsHint) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *RawNode) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (m *Strong) MarshalJSON() ([]byte, error) { return marshalMark(m) }

// MarshalJSON implements json.Marshaler.
func (m *Em) MarshalJSON() ([]byte, error) { return marshalMark(m) }

// MarshalJSON implements json.Marshaler.
func (m *Strike) MarshalJSON() ([]byte, error) { return marshalMark(m) }

// MarshalJSON implements json.Marshaler.
func (m *Code) MarshalJSON() ([]byte, error) { return marshalMark(m) }

// MarshalJSON implements json.Marshaler.
func (m *Underline) MarshalJSON() ([]byte, error) { return marshalMark(m) }

// MarshalJSON implements json.Marshaler.
func (m *Link) MarshalJSON() ([]byte, error) { return marshalMark(m) }

// MarshalJSON implements json.Marshaler.
func (m *TextColor) MarshalJSON() ([]byte, error) { return marshalMark(m) }

// MarshalJSON implements json.Marshaler.
func (m *BackgroundColor) MarshalJSON() ([]byte, error) { return marshalMark(m) }

// MarshalJSON implements json.Marshaler.
func (m *SubSup) MarshalJSON() ([]byte, error) { return marshalMark(m) }

// MarshalJSON implements json.Marshaler.
func (m *RawMark) MarshalJSON() ([]byte, error) { return marshalMark(m) }
