package adf

import (
	"encoding/json"
	"slices"
	"strconv"
)

// The decode half of the JSON codec: JSON-decoded values become typed
// nodes/marks, unknown kinds become RawNode/RawMark, and known kinds
// keep every attribute the typed fields cannot represent faithfully in
// Extra — DecodeDoc → json.Marshal reproduces the input semantically.

// DecodeOptions configures DecodeDocOpts.
type DecodeOptions struct {
	// Diagnostics receives "unknown-node", "unknown-mark", and
	// "unknown-attr" diagnostics for input the typed model does not
	// know (the content is still kept losslessly), and
	// "depth-exceeded" when nesting beyond the decoder's recursion cap
	// was truncated. Nil drops them.
	Diagnostics func(Diagnostic)
}

// maxDecodeDepth caps the decoder's content recursion. Legitimate
// documents nest a few dozen levels at most; adversarial input (deeply
// nested JSON built to overflow the stack) is truncated at the cap with
// a depth-exceeded diagnostic instead of crashing the process.
const maxDecodeDepth = 1024

// DecodeDoc converts any JSON-decoded ADF value into a typed Doc.
// Accepts Doc, *Doc, map[string]any (from json.Unmarshal), or raw JSON
// bytes ([]byte / json.RawMessage).
func DecodeDoc(v any) (Doc, bool) {
	return DecodeDocOpts(v, DecodeOptions{})
}

// DecodeDocOpts is DecodeDoc with decode diagnostics.
func DecodeDocOpts(v any, opts DecodeOptions) (Doc, bool) {
	d := &decoder{diag: opts.Diagnostics}
	switch t := v.(type) {
	case Doc:
		return t, true
	case *Doc:
		if t == nil {
			return Doc{}, false
		}
		return *t, true
	case map[string]any:
		return d.doc(t), true
	case json.RawMessage:
		return decodeDocBytes(d, t)
	case []byte:
		return decodeDocBytes(d, t)
	}
	return Doc{}, false
}

func decodeDocBytes(d *decoder, data []byte) (Doc, bool) {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil || m == nil {
		return Doc{}, false
	}
	return d.doc(m), true
}

// UnmarshalJSON implements json.Unmarshaler, decoding the generic wire
// shape into typed nodes. Unlike DecodeDoc it honors the document's own
// type/version fields.
func (d *Doc) UnmarshalJSON(data []byte) error {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	dec := &decoder{}
	out := Doc{}
	if t, ok := m["type"].(string); ok {
		out.Type = t
	}
	if v, ok := m["version"].(float64); ok {
		out.Version = int(v)
	}
	out.Content = dec.content(m, 0)
	*d = out
	return nil
}

type decoder struct {
	diag func(Diagnostic)
	// depthReported dedupes the depth-exceeded diagnostic per document.
	depthReported bool
}

func (d *decoder) report(code, message string) {
	if d.diag != nil {
		d.diag(Diagnostic{Code: code, Message: message})
	}
}

// doc decodes a document map, normalizing to type "doc" version 1 (the
// historical DecodeDoc behavior).
func (d *decoder) doc(m map[string]any) Doc {
	return Doc{Type: "doc", Version: 1, Content: d.content(m, 0)}
}

// content decodes a node map's "content" array (non-map entries and a
// non-array value are dropped, matching the pre-typed decoder). Content
// nested beyond maxDecodeDepth is truncated with a depth-exceeded
// diagnostic — unbounded recursion on adversarial nesting would
// overflow the stack, which Go cannot recover from.
func (d *decoder) content(m map[string]any, depth int) []Node {
	arr, ok := m["content"].([]any)
	if !ok {
		return nil
	}
	if depth >= maxDecodeDepth {
		if !d.depthReported {
			d.depthReported = true
			d.report(CodeDepthExceeded, "document nesting exceeds the decode depth cap; deeper content truncated")
		}
		return nil
	}
	var out []Node
	for _, c := range arr {
		if cm, ok := c.(map[string]any); ok {
			out = append(out, d.node(cm, depth+1))
		}
	}
	return out
}

// nodeShape classifies each known kind's generic-slot usage; a document
// using the slots differently falls back to RawNode for losslessness.
type nodeShape int

const (
	shapeBranch   nodeShape = iota // content allowed, no text
	shapeLeaf                      // neither content nor text
	shapeTextLeaf                  // text allowed, no content
)

// nodeDecoder ties one wire type string to its generic-slot shape and its
// typed constructor.
//
// Shape and constructor live in one entry on purpose. They used to be a
// shape map next to a parallel switch, and the two drifted: "image",
// "frontmatter", and "html" carried a shape but had no constructor, so a
// document containing one decoded to a nil Node that blew up the first
// caller to touch it. Here a type string is either absent from the table
// (RawNode, lossless, with an unknown-node diagnostic) or it has both.
type nodeDecoder struct {
	// build receives the attribute reader plus whichever generic slots
	// the shape permits; the other slot is the zero value.
	build func(r *attrReader, content []Node, text string) Node
	shape nodeShape
}

// nodeDecoders is the known node kind set: the decoder's half of the
// typed model. A kind absent here decodes to RawNode.
var nodeDecoders = map[string]nodeDecoder{
	// Text flow.
	"paragraph":  {shape: shapeBranch, build: func(_ *attrReader, c []Node, _ string) Node { return &Paragraph{Content: c} }},
	"blockquote": {shape: shapeBranch, build: func(_ *attrReader, c []Node, _ string) Node { return &Blockquote{Content: c} }},
	"heading": {shape: shapeBranch, build: func(r *attrReader, c []Node, _ string) Node {
		return &Heading{Level: r.intVal("level"), Anchor: r.str("anchor"), Content: c}
	}},
	"codeBlock": {shape: shapeBranch, build: func(r *attrReader, c []Node, _ string) Node {
		return &CodeBlock{Language: r.str("language"), Content: c}
	}},

	// Lists.
	"listItem": {shape: shapeBranch, build: func(_ *attrReader, c []Node, _ string) Node { return &ListItem{Content: c} }},
	"bulletList": {shape: shapeBranch, build: func(r *attrReader, c []Node, _ string) Node {
		return &BulletList{Tight: r.boolPtr("tight"), Content: c}
	}},
	"orderedList": {shape: shapeBranch, build: func(r *attrReader, c []Node, _ string) Node {
		return &OrderedList{Order: r.intPtr("order"), Tight: r.boolPtr("tight"), Content: c}
	}},
	"taskList": {shape: shapeBranch, build: func(r *attrReader, c []Node, _ string) Node {
		return &TaskList{LocalID: r.strPtr("localId"), Content: c}
	}},
	"taskItem": {shape: shapeBranch, build: func(r *attrReader, c []Node, _ string) Node {
		return &TaskItem{LocalID: r.strPtr("localId"), State: r.str("state"), Content: c}
	}},
	"blockTaskItem": {shape: shapeBranch, build: func(r *attrReader, c []Node, _ string) Node {
		return &BlockTaskItem{LocalID: r.strPtr("localId"), State: r.str("state"), Content: c}
	}},
	"decisionList": {shape: shapeBranch, build: func(r *attrReader, c []Node, _ string) Node {
		return &DecisionList{LocalID: r.strPtr("localId"), Content: c}
	}},
	"decisionItem": {shape: shapeBranch, build: func(r *attrReader, c []Node, _ string) Node {
		return &DecisionItem{LocalID: r.strPtr("localId"), State: r.str("state"), Content: c}
	}},

	// Tables.
	"tableRow": {shape: shapeBranch, build: func(_ *attrReader, c []Node, _ string) Node { return &TableRow{Content: c} }},
	"table": {shape: shapeBranch, build: func(r *attrReader, c []Node, _ string) Node {
		return &Table{
			Align:  r.strs("align"),
			Layout: r.str("layout"), Width: r.floatPtr("width"),
			IsNumberColumnEnabled: r.boolPtr("isNumberColumnEnabled"),
			LocalID:               r.str("localId"), DisplayMode: r.str("displayMode"),
			Content: c,
		}
	}},
	"tableCell": {shape: shapeBranch, build: func(r *attrReader, c []Node, _ string) Node {
		return &TableCell{
			Colspan: r.intVal("colspan"), Rowspan: r.intVal("rowspan"),
			Colwidth: r.floats("colwidth"), Content: c,
		}
	}},
	"tableHeader": {shape: shapeBranch, build: func(r *attrReader, c []Node, _ string) Node {
		return &TableHeader{
			Colspan: r.intVal("colspan"), Rowspan: r.intVal("rowspan"),
			Colwidth: r.floats("colwidth"), Content: c,
		}
	}},

	// Wrappers and layout.
	"panel": {shape: shapeBranch, build: func(r *attrReader, c []Node, _ string) Node {
		return &Panel{PanelType: r.str("panelType"), Content: c}
	}},
	"expand": {shape: shapeBranch, build: func(r *attrReader, c []Node, _ string) Node {
		return &Expand{Title: r.strPtr("title"), Content: c}
	}},
	"nestedExpand": {shape: shapeBranch, build: func(r *attrReader, c []Node, _ string) Node {
		return &NestedExpand{Title: r.strPtr("title"), Content: c}
	}},
	"caption": {shape: shapeBranch, build: func(r *attrReader, c []Node, _ string) Node {
		return &Caption{LocalID: r.str("localId"), Content: c}
	}},
	"layoutSection": {shape: shapeBranch, build: func(r *attrReader, c []Node, _ string) Node {
		return &LayoutSection{LocalID: r.str("localId"), ColumnRuleStyle: r.str("columnRuleStyle"), Content: c}
	}},
	"layoutColumn": {shape: shapeBranch, build: func(r *attrReader, c []Node, _ string) Node {
		return &LayoutColumn{Width: r.floatPtr("width"), LocalID: r.str("localId"), VAlign: r.str("valign"), Content: c}
	}},

	// Media and cards.
	"mediaGroup": {shape: shapeBranch, build: func(_ *attrReader, c []Node, _ string) Node { return &MediaGroup{Content: c} }},
	"mediaSingle": {shape: shapeBranch, build: func(r *attrReader, c []Node, _ string) Node {
		return &MediaSingle{
			Layout: r.strPtr("layout"), Width: r.floatPtr("width"),
			WidthType: r.strPtr("widthType"), Content: c,
		}
	}},
	"media": {shape: shapeLeaf, build: func(r *attrReader, _ []Node, _ string) Node {
		return &Media{
			Type: r.str("type"), ID: r.str("id"), URL: r.str("url"), Alt: r.str("alt"),
			Collection: r.strPtr("collection"), Width: r.floatPtr("width"),
			Height: r.floatPtr("height"), OccurrenceKey: r.strPtr("occurrenceKey"),
		}
	}},
	"mediaInline": {shape: shapeLeaf, build: func(r *attrReader, _ []Node, _ string) Node {
		return &MediaInline{
			Type: r.str("type"), ID: r.str("id"),
			Alt: r.str("alt"), Collection: r.strPtr("collection"),
		}
	}},
	"blockCard": {shape: shapeLeaf, build: func(r *attrReader, _ []Node, _ string) Node {
		return &BlockCard{URL: r.str("url"), Datasource: r.rawMap("datasource")}
	}},
	"embedCard": {shape: shapeLeaf, build: func(r *attrReader, _ []Node, _ string) Node {
		return &EmbedCard{URL: r.str("url"), Layout: r.str("layout"), Width: r.floatPtr("width")}
	}},
	"inlineCard": {shape: shapeLeaf, build: func(r *attrReader, _ []Node, _ string) Node {
		return &InlineCard{URL: r.strPtr("url")}
	}},

	// Inline atoms.
	"rule":      {shape: shapeLeaf, build: func(_ *attrReader, _ []Node, _ string) Node { return &Rule{} }},
	"hardBreak": {shape: shapeLeaf, build: func(_ *attrReader, _ []Node, _ string) Node { return &HardBreak{} }},
	"text":      {shape: shapeTextLeaf, build: func(_ *attrReader, _ []Node, t string) Node { return &Text{Text: t} }},
	"emoji": {shape: shapeLeaf, build: func(r *attrReader, _ []Node, _ string) Node {
		return &Emoji{ShortName: r.str("shortName"), ID: r.str("id"), Text: r.strPtr("text")}
	}},
	"mention": {shape: shapeLeaf, build: func(r *attrReader, _ []Node, _ string) Node {
		return &Mention{ID: r.str("id"), Text: r.strPtr("text"), AccessLevel: r.str("accessLevel")}
	}},
	"status": {shape: shapeLeaf, build: func(r *attrReader, _ []Node, _ string) Node {
		return &Status{
			Text: r.strPtr("text"), Color: r.str("color"),
			Style: r.str("style"), LocalID: r.str("localId"),
		}
	}},
	"date": {shape: shapeLeaf, build: func(r *attrReader, _ []Node, _ string) Node {
		return &Date{Timestamp: r.str("timestamp"), LocalID: r.str("localId")}
	}},
	"placeholder": {shape: shapeLeaf, build: func(r *attrReader, _ []Node, _ string) Node {
		return &Placeholder{Text: r.str("text"), LocalID: r.str("localId")}
	}},
	ColwidthsHintType: {shape: shapeLeaf, build: func(r *attrReader, _ []Node, _ string) Node {
		return &ColwidthsHint{Widths: r.floats("widths")}
	}},

	// Extension points.
	"extensionFrame": {shape: shapeBranch, build: func(_ *attrReader, c []Node, _ string) Node { return &ExtensionFrame{Content: c} }},
	"extension": {shape: shapeLeaf, build: func(r *attrReader, _ []Node, _ string) Node {
		return &Extension{
			ExtensionType: r.str("extensionType"), ExtensionKey: r.str("extensionKey"),
			Parameters: r.parameters(), Text: r.str("text"),
			Layout: r.str("layout"), LocalID: r.str("localId"),
		}
	}},
	"inlineExtension": {shape: shapeLeaf, build: func(r *attrReader, _ []Node, _ string) Node {
		return &InlineExtension{
			ExtensionType: r.str("extensionType"), ExtensionKey: r.str("extensionKey"),
			Parameters: r.parameters(), Text: r.str("text"), LocalID: r.str("localId"),
		}
	}},
	"bodiedExtension": {shape: shapeBranch, build: func(r *attrReader, c []Node, _ string) Node {
		return &BodiedExtension{
			ExtensionType: r.str("extensionType"), ExtensionKey: r.str("extensionKey"),
			Parameters: r.parameters(), Text: r.str("text"),
			Layout: r.str("layout"), LocalID: r.str("localId"), Content: c,
		}
	}},
	"multiBodiedExtension": {shape: shapeBranch, build: func(r *attrReader, c []Node, _ string) Node {
		return &MultiBodiedExtension{
			ExtensionType: r.str("extensionType"), ExtensionKey: r.str("extensionKey"),
			Parameters: r.parameters(), Text: r.str("text"),
			Layout: r.str("layout"), LocalID: r.str("localId"), Content: c,
		}
	}},
	"syncBlock": {shape: shapeLeaf, build: func(r *attrReader, _ []Node, _ string) Node {
		return &SyncBlock{ResourceID: r.str("resourceId"), LocalID: r.str("localId")}
	}},
	"bodiedSyncBlock": {shape: shapeBranch, build: func(r *attrReader, c []Node, _ string) Node {
		return &BodiedSyncBlock{ResourceID: r.str("resourceId"), LocalID: r.str("localId"), Content: c}
	}},
}

// strField reads a string key from a decoded JSON map ("" when absent
// or mistyped, matching the pre-typed decoder).
func strField(m map[string]any, key string) string {
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}

// mapField reads an object key from a decoded JSON map (nil when absent
// or mistyped).
func mapField(m map[string]any, key string) map[string]any {
	if mm, ok := m[key].(map[string]any); ok {
		return mm
	}
	return nil
}

// node decodes one node map: typed for known kinds whose shape fits,
// RawNode otherwise.
func (d *decoder) node(m map[string]any, depth int) Node {
	typ := strField(m, "type")
	text := strField(m, "text")
	marks := d.marks(m)
	content := d.content(m, depth)
	attrMap := mapField(m, "attrs")

	dec, known := nodeDecoders[typ]
	if !known {
		d.report(CodeUnknownNode, "unknown ADF node type "+strconv.Quote(typ)+" kept as RawNode")
		return &RawNode{Type: typ, Attrs: attrMap, Marks: marks, Text: text, Content: content}
	}
	if (dec.shape != shapeTextLeaf && text != "") || (dec.shape != shapeBranch && len(content) > 0) {
		// A known kind used with slots its typed struct does not model;
		// RawNode keeps it lossless.
		return &RawNode{Type: typ, Attrs: attrMap, Marks: marks, Text: text, Content: content}
	}

	r := newAttrReader(attrMap)
	n := dec.build(r, content, text)
	s := n.slots()
	if s.marks != nil {
		*s.marks = marks
	}
	if s.extra != nil {
		*s.extra = r.finish(d, typ)
	}
	return n
}

// marks decodes a node map's "marks" array (non-map entries and a
// non-array value are dropped, matching the pre-typed decoder).
func (d *decoder) marks(m map[string]any) []Mark {
	arr, ok := m["marks"].([]any)
	if !ok {
		return nil
	}
	var out []Mark
	for _, mk := range arr {
		if mm, ok := mk.(map[string]any); ok {
			out = append(out, d.mark(mm))
		}
	}
	return out
}

func (d *decoder) mark(m map[string]any) Mark {
	typ := strField(m, "type")
	attrMap := mapField(m, "attrs")
	build, known := markDecoders[typ]
	if !known {
		d.report(CodeUnknownMark, "unknown ADF mark type "+strconv.Quote(typ)+" kept as RawMark")
		return &RawMark{Type: typ, Attrs: attrMap}
	}
	r := newAttrReader(attrMap)
	mk := build(r)
	mk.setExtra(r.finish(d, typ+" mark"))
	return mk
}

// markDecoders is the known mark kind set, the mark counterpart of
// nodeDecoders. A kind absent here decodes to RawMark.
var markDecoders = map[string]func(r *attrReader) Mark{
	"strong":    func(*attrReader) Mark { return &Strong{} },
	"em":        func(*attrReader) Mark { return &Em{} },
	"strike":    func(*attrReader) Mark { return &Strike{} },
	"code":      func(*attrReader) Mark { return &Code{} },
	"underline": func(*attrReader) Mark { return &Underline{} },
	"link":      func(r *attrReader) Mark { return &Link{Href: r.strPtr("href")} },
	"textColor": func(r *attrReader) Mark { return &TextColor{Color: r.str("color")} },
	"backgroundColor": func(r *attrReader) Mark {
		return &BackgroundColor{Color: r.str("color")}
	},
	"subsup":       func(r *attrReader) Mark { return &SubSup{Type: r.str("type")} },
	"alignment":    func(r *attrReader) Mark { return &Alignment{Align: r.str("align")} },
	"indentation":  func(r *attrReader) Mark { return &Indentation{Level: r.intVal("level")} },
	"fontSize":     func(r *attrReader) Mark { return &FontSize{Size: r.str("fontSize")} },
	"dataConsumer": func(r *attrReader) Mark { return &DataConsumer{Sources: r.strs("sources")} },
	"breakout": func(r *attrReader) Mark {
		return &Breakout{Mode: r.str("mode"), Width: r.floatPtr("width")}
	},
	"border": func(r *attrReader) Mark {
		return &Border{Color: r.str("color"), Size: r.intVal("size")}
	},
	"annotation": func(r *attrReader) Mark {
		return &Annotation{ID: r.str("id"), AnnotationType: r.str("annotationType")}
	},
	"fragment": func(r *attrReader) Mark {
		return &Fragment{LocalID: r.str("localId"), Name: r.str("name")}
	},
}

// attrReader lifts known attributes into typed values, keeping every
// value the typed field cannot represent faithfully (zero values where
// encoding only emits non-zero, type mismatches, non-integral numbers)
// in extra so re-encoding is lossless.
type attrReader struct {
	src   map[string]any
	known map[string]bool
	extra map[string]any
}

func newAttrReader(src map[string]any) *attrReader {
	return &attrReader{src: src, known: map[string]bool{}}
}

// keep routes a known key's raw value to extra for encode fidelity.
func (r *attrReader) keep(k string, v any) {
	if r.extra == nil {
		r.extra = map[string]any{}
	}
	r.extra[k] = v
}

// str reads a string attribute ("" when absent, empty, or mistyped —
// the zero and mistyped forms stay in extra).
func (r *attrReader) str(k string) string {
	r.known[k] = true
	v, ok := r.src[k]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	r.keep(k, v)
	return ""
}

// strPtr reads a presence-sensitive string attribute.
func (r *attrReader) strPtr(k string) *string {
	r.known[k] = true
	v, ok := r.src[k]
	if !ok {
		return nil
	}
	if s, ok := v.(string); ok {
		return &s
	}
	r.keep(k, v)
	return nil
}

// intVal reads an integer attribute (accepting the float64 form JSON
// produces, truncating like the historical IntAttr; zero and
// non-integral raw values stay in extra).
func (r *attrReader) intVal(k string) int {
	r.known[k] = true
	v, ok := r.src[k]
	if !ok {
		return 0
	}
	if f, ok := v.(float64); ok {
		i := int(f)
		if float64(i) != f || i == 0 {
			r.keep(k, v)
		}
		return i
	}
	r.keep(k, v)
	return 0
}

// intPtr reads a presence-sensitive integer attribute.
func (r *attrReader) intPtr(k string) *int {
	r.known[k] = true
	v, ok := r.src[k]
	if !ok {
		return nil
	}
	if f, ok := v.(float64); ok {
		i := int(f)
		if float64(i) != f {
			r.keep(k, v)
		}
		return &i
	}
	r.keep(k, v)
	return nil
}

// floatPtr reads a presence-sensitive number attribute.
func (r *attrReader) floatPtr(k string) *float64 {
	r.known[k] = true
	v, ok := r.src[k]
	if !ok {
		return nil
	}
	if f, ok := v.(float64); ok {
		return &f
	}
	r.keep(k, v)
	return nil
}

// boolVal reads a bool attribute (false-but-present stays in extra).

// boolPtr reads a presence-sensitive bool attribute.
func (r *attrReader) boolPtr(k string) *bool {
	r.known[k] = true
	v, ok := r.src[k]
	if !ok {
		return nil
	}
	if b, ok := v.(bool); ok {
		return &b
	}
	r.keep(k, v)
	return nil
}

// floats reads a number-array attribute (non-nil even when empty; any
// non-number element keeps the whole raw array in extra instead).
func (r *attrReader) floats(k string) []float64 {
	r.known[k] = true
	v, ok := r.src[k]
	if !ok {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		r.keep(k, v)
		return nil
	}
	out := make([]float64, 0, len(arr))
	for _, e := range arr {
		f, ok := e.(float64)
		if !ok {
			r.keep(k, v)
			return nil
		}
		out = append(out, f)
	}
	return out
}

// strs reads a string-array attribute (non-nil even when empty; any
// non-string element keeps the whole raw array in extra instead).
func (r *attrReader) strs(k string) []string {
	r.known[k] = true
	v, ok := r.src[k]
	if !ok {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		r.keep(k, v)
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		s, ok := e.(string)
		if !ok {
			r.keep(k, v)
			return nil
		}
		out = append(out, s)
	}
	return out
}

// parameters reads the extension "parameters" attribute verbatim: it is
// the one modeled attribute of arbitrary JSON shape, so it keeps its own
// reader rather than a keyed one (nil when absent; a present-but-null
// value stays in extra).
func (r *attrReader) parameters() any {
	const k = "parameters"
	r.known[k] = true
	v, ok := r.src[k]
	if !ok {
		return nil
	}
	if v == nil {
		r.keep(k, v)
		return nil
	}
	return v
}

// rawMap reads a map attribute verbatim.
func (r *attrReader) rawMap(k string) map[string]any {
	r.known[k] = true
	v, ok := r.src[k]
	if !ok {
		return nil
	}
	if mm, ok := v.(map[string]any); ok {
		return mm
	}
	r.keep(k, v)
	return nil
}

// finish routes the unconsumed (unmodeled) attributes to extra with an
// unknown-attr diagnostic each, and returns the extra map.
func (r *attrReader) finish(d *decoder, kind string) map[string]any {
	var unknown []string
	for k := range r.src {
		if !r.known[k] {
			unknown = append(unknown, k)
		}
	}
	slices.Sort(unknown)
	for _, k := range unknown {
		r.keep(k, r.src[k])
		d.report(CodeUnknownAttr, "unmodeled attribute "+strconv.Quote(k)+" on "+kind+" kept in Extra")
	}
	return r.extra
}
