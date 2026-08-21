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
	shapeUnknown  nodeShape = iota
	shapeBranch             // content allowed, no text
	shapeLeaf               // neither content nor text
	shapeTextLeaf           // text allowed, no content
)

var nodeShapes = map[string]nodeShape{
	"paragraph": shapeBranch, "heading": shapeBranch, "blockquote": shapeBranch,
	"codeBlock": shapeBranch, "bulletList": shapeBranch, "orderedList": shapeBranch,
	"listItem": shapeBranch, "taskList": shapeBranch, "taskItem": shapeBranch,
	"decisionList": shapeBranch, "decisionItem": shapeBranch, "table": shapeBranch,
	"tableRow": shapeBranch, "tableCell": shapeBranch, "tableHeader": shapeBranch,
	"panel": shapeBranch, "expand": shapeBranch, "nestedExpand": shapeBranch,
	"mediaSingle": shapeBranch, "mediaGroup": shapeBranch, "image": shapeBranch,
	"caption": shapeBranch, "blockTaskItem": shapeBranch,
	"layoutSection": shapeBranch, "layoutColumn": shapeBranch,
	"bodiedExtension": shapeBranch, "multiBodiedExtension": shapeBranch,
	"extensionFrame": shapeBranch, "bodiedSyncBlock": shapeBranch,
	"rule": shapeLeaf, "media": shapeLeaf, "blockCard": shapeLeaf,
	"embedCard": shapeLeaf, "inlineCard": shapeLeaf, "hardBreak": shapeLeaf,
	"emoji": shapeLeaf, "mention": shapeLeaf, "status": shapeLeaf,
	"mediaInline": shapeLeaf, ColwidthsHintType: shapeLeaf,
	"date": shapeLeaf, "placeholder": shapeLeaf, "extension": shapeLeaf,
	"inlineExtension": shapeLeaf, "syncBlock": shapeLeaf,
	"text": shapeTextLeaf, "frontmatter": shapeTextLeaf, "html": shapeTextLeaf,
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

	shape := nodeShapes[typ]
	if shape == shapeUnknown {
		d.report(CodeUnknownNode, "unknown ADF node type "+strconv.Quote(typ)+" kept as RawNode")
		return &RawNode{Type: typ, Attrs: attrMap, Marks: marks, Text: text, Content: content}
	}
	if (shape != shapeTextLeaf && text != "") || (shape != shapeBranch && len(content) > 0) {
		// A known kind used with slots its typed struct does not model;
		// RawNode keeps it lossless.
		return &RawNode{Type: typ, Attrs: attrMap, Marks: marks, Text: text, Content: content}
	}

	r := newAttrReader(attrMap)
	n := decodeBlockKind(typ, r, content)
	if n == nil {
		n = decodeLeafKind(typ, r, text)
	}
	s := slotsOf(n)
	if s.marks != nil {
		*s.marks = marks
	}
	if s.extra != nil {
		*s.extra = r.finish(d, typ)
	}
	return n
}

// decodeBlockKind builds the content-bearing kinds (nil when typ is not
// one of them).
func decodeBlockKind(typ string, r *attrReader, content []Node) Node {
	switch typ {
	case "paragraph":
		return &Paragraph{Content: content}
	case "heading":
		return &Heading{Level: r.intVal("level"), Anchor: r.str("anchor"), Content: content}
	case "blockquote":
		return &Blockquote{Content: content}
	case "codeBlock":
		return &CodeBlock{Language: r.str("language"), Content: content}
	case "bulletList":
		return &BulletList{Tight: r.boolPtr("tight"), Content: content}
	case "orderedList":
		return &OrderedList{Order: r.intPtr("order"), Tight: r.boolPtr("tight"), Content: content}
	case "listItem":
		return &ListItem{Content: content}
	case "taskList":
		return &TaskList{LocalID: r.strPtr("localId"), Content: content}
	case "taskItem":
		return &TaskItem{LocalID: r.strPtr("localId"), State: r.str("state"), Content: content}
	case "decisionList":
		return &DecisionList{LocalID: r.strPtr("localId"), Content: content}
	case "decisionItem":
		return &DecisionItem{LocalID: r.strPtr("localId"), State: r.str("state"), Content: content}
	case "table":
		return &Table{
			Align:  r.strs("align"),
			Layout: r.str("layout"), Width: r.floatPtr("width"),
			IsNumberColumnEnabled: r.boolPtr("isNumberColumnEnabled"),
			LocalID:               r.str("localId"), DisplayMode: r.str("displayMode"),
			Content: content,
		}
	case "tableRow":
		return &TableRow{Content: content}
	case "tableCell":
		return &TableCell{Colspan: r.intVal("colspan"), Rowspan: r.intVal("rowspan"), Colwidth: r.floats("colwidth"), Content: content}
	case "tableHeader":
		return &TableHeader{Colspan: r.intVal("colspan"), Rowspan: r.intVal("rowspan"), Colwidth: r.floats("colwidth"), Content: content}
	case "panel":
		return &Panel{PanelType: r.str("panelType"), Content: content}
	case "expand":
		return &Expand{Title: r.strPtr("title"), Content: content}
	case "nestedExpand":
		return &NestedExpand{Title: r.strPtr("title"), Content: content}
	case "mediaSingle":
		return &MediaSingle{Layout: r.strPtr("layout"), Width: r.floatPtr("width"), WidthType: r.strPtr("widthType"), Content: content}
	case "mediaGroup":
		return &MediaGroup{Content: content}
	}
	return decodeExtendedBlockKind(typ, r, content)
}

// decodeExtendedBlockKind builds the extended content-bearing kinds
// (nil when typ is not one of them).
func decodeExtendedBlockKind(typ string, r *attrReader, content []Node) Node {
	switch typ {
	case "caption":
		return &Caption{LocalID: r.str("localId"), Content: content}
	case "blockTaskItem":
		return &BlockTaskItem{LocalID: r.strPtr("localId"), State: r.str("state"), Content: content}
	case "layoutSection":
		return &LayoutSection{LocalID: r.str("localId"), ColumnRuleStyle: r.str("columnRuleStyle"), Content: content}
	case "layoutColumn":
		return &LayoutColumn{Width: r.floatPtr("width"), LocalID: r.str("localId"), VAlign: r.str("valign"), Content: content}
	case "bodiedExtension":
		return &BodiedExtension{
			ExtensionType: r.str("extensionType"), ExtensionKey: r.str("extensionKey"),
			Parameters: r.anyVal("parameters"), Text: r.str("text"),
			Layout: r.str("layout"), LocalID: r.str("localId"), Content: content,
		}
	case "multiBodiedExtension":
		return &MultiBodiedExtension{
			ExtensionType: r.str("extensionType"), ExtensionKey: r.str("extensionKey"),
			Parameters: r.anyVal("parameters"), Text: r.str("text"),
			Layout: r.str("layout"), LocalID: r.str("localId"), Content: content,
		}
	case "extensionFrame":
		return &ExtensionFrame{Content: content}
	case "bodiedSyncBlock":
		return &BodiedSyncBlock{ResourceID: r.str("resourceId"), LocalID: r.str("localId"), Content: content}
	}
	return nil
}

// decodeLeafKind builds the leaf and text-leaf kinds.
func decodeLeafKind(typ string, r *attrReader, text string) Node {
	switch typ {
	case "rule":
		return &Rule{}
	case "media":
		return &Media{
			Type: r.str("type"), ID: r.str("id"), URL: r.str("url"), Alt: r.str("alt"),
			Collection: r.strPtr("collection"), Width: r.floatPtr("width"),
			Height: r.floatPtr("height"), OccurrenceKey: r.strPtr("occurrenceKey"),
		}
	case "blockCard":
		return &BlockCard{URL: r.str("url"), Datasource: r.rawMap("datasource")}
	case "embedCard":
		return &EmbedCard{URL: r.str("url"), Layout: r.str("layout"), Width: r.floatPtr("width")}
	case "inlineCard":
		return &InlineCard{URL: r.strPtr("url")}
	case "hardBreak":
		return &HardBreak{}
	case "emoji":
		return &Emoji{ShortName: r.str("shortName"), ID: r.str("id"), Text: r.strPtr("text")}
	case "mention":
		return &Mention{ID: r.str("id"), Text: r.strPtr("text"), AccessLevel: r.str("accessLevel")}
	case "status":
		return &Status{Text: r.strPtr("text"), Color: r.str("color"), Style: r.str("style"), LocalID: r.str("localId")}
	case "mediaInline":
		return &MediaInline{Type: r.str("type"), ID: r.str("id"), Alt: r.str("alt"), Collection: r.strPtr("collection")}
	case ColwidthsHintType:
		return &ColwidthsHint{Widths: r.floats("widths")}
	case "date":
		return &Date{Timestamp: r.str("timestamp"), LocalID: r.str("localId")}
	case "placeholder":
		return &Placeholder{Text: r.str("text"), LocalID: r.str("localId")}
	case "extension":
		return &Extension{
			ExtensionType: r.str("extensionType"), ExtensionKey: r.str("extensionKey"),
			Parameters: r.anyVal("parameters"), Text: r.str("text"),
			Layout: r.str("layout"), LocalID: r.str("localId"),
		}
	case "inlineExtension":
		return &InlineExtension{
			ExtensionType: r.str("extensionType"), ExtensionKey: r.str("extensionKey"),
			Parameters: r.anyVal("parameters"), Text: r.str("text"), LocalID: r.str("localId"),
		}
	case "syncBlock":
		return &SyncBlock{ResourceID: r.str("resourceId"), LocalID: r.str("localId")}
	case "text":
		return &Text{Text: text}
	}
	return nil
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
	r := newAttrReader(attrMap)
	mk := decodeMarkKind(typ, r)
	if mk == nil {
		d.report(CodeUnknownMark, "unknown ADF mark type "+strconv.Quote(typ)+" kept as RawMark")
		return &RawMark{Type: typ, Attrs: attrMap}
	}
	setMarkExtra(mk, r.finish(d, typ+" mark"))
	return mk
}

func decodeMarkKind(typ string, r *attrReader) Mark {
	switch typ {
	case "strong":
		return &Strong{}
	case "em":
		return &Em{}
	case "strike":
		return &Strike{}
	case "code":
		return &Code{}
	case "underline":
		return &Underline{}
	case "link":
		return &Link{Href: r.strPtr("href")}
	case "textColor":
		return &TextColor{Color: r.str("color")}
	case "backgroundColor":
		return &BackgroundColor{Color: r.str("color")}
	case "subsup":
		return &SubSup{Type: r.str("type")}
	case "alignment":
		return &Alignment{Align: r.str("align")}
	case "indentation":
		return &Indentation{Level: r.intVal("level")}
	case "breakout":
		return &Breakout{Mode: r.str("mode"), Width: r.floatPtr("width")}
	case "border":
		return &Border{Color: r.str("color"), Size: r.intVal("size")}
	case "annotation":
		return &Annotation{ID: r.str("id"), AnnotationType: r.str("annotationType")}
	case "dataConsumer":
		return &DataConsumer{Sources: r.strs("sources")}
	case "fragment":
		return &Fragment{LocalID: r.str("localId"), Name: r.str("name")}
	case "fontSize":
		return &FontSize{Size: r.str("fontSize")}
	}
	return nil
}

func setMarkExtra(m Mark, extra map[string]any) {
	switch t := m.(type) {
	case *Strong:
		t.Extra = extra
	case *Em:
		t.Extra = extra
	case *Strike:
		t.Extra = extra
	case *Code:
		t.Extra = extra
	case *Underline:
		t.Extra = extra
	case *Link:
		t.Extra = extra
	case *TextColor:
		t.Extra = extra
	case *BackgroundColor:
		t.Extra = extra
	case *SubSup:
		t.Extra = extra
	default:
		setExtendedMarkExtra(m, extra)
	}
}

func setExtendedMarkExtra(m Mark, extra map[string]any) {
	switch t := m.(type) {
	case *Alignment:
		t.Extra = extra
	case *Indentation:
		t.Extra = extra
	case *Breakout:
		t.Extra = extra
	case *Border:
		t.Extra = extra
	case *Annotation:
		t.Extra = extra
	case *DataConsumer:
		t.Extra = extra
	case *Fragment:
		t.Extra = extra
	case *FontSize:
		t.Extra = extra
	}
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

// anyVal reads an attribute of arbitrary JSON shape verbatim (nil when
// absent; a present-but-null value stays in extra).
//
//nolint:unparam // keyed like the other readers; only "parameters" today
func (r *attrReader) anyVal(k string) any {
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
