package adf

// The extended node kinds: date, placeholder, captions, block task items,
// page layouts, the extension family, and synced blocks. They follow the
// same field ↔ attribute rules as nodes.go.

// Date is an ADF date leaf; Timestamp is the "timestamp" attribute
// (milliseconds since epoch, kept as the wire string).
type Date struct {
	Extra     map[string]any
	Timestamp string
	LocalID   string
	Marks     []Mark
}

// Placeholder is an ADF placeholder leaf (template placeholder text);
// Text is the "text" attribute.
type Placeholder struct {
	Extra   map[string]any
	Text    string
	LocalID string
	Marks   []Mark
}

// Caption is an ADF caption block (the optional second child of a
// mediaSingle); its content is inline nodes.
type Caption struct {
	Extra   map[string]any
	LocalID string
	Content []Node
	Marks   []Mark
}

// BlockTaskItem is an ADF blockTaskItem: a taskItem whose content is
// block nodes instead of inline content.
type BlockTaskItem struct {
	LocalID *string
	Extra   map[string]any
	State   string
	Content []Node
	Marks   []Mark
}

// LayoutSection is an ADF layoutSection (a page-layout row of columns).
type LayoutSection struct {
	Extra           map[string]any
	LocalID         string
	ColumnRuleStyle string
	Content         []Node
	Marks           []Mark
}

// LayoutColumn is an ADF layoutColumn; Width is the column width in
// percent (required by the schema, a pointer for wire fidelity).
type LayoutColumn struct {
	Width   *float64
	Extra   map[string]any
	LocalID string
	VAlign  string
	Content []Node
	Marks   []Mark
}

// Extension is an ADF extension block (a bodiless macro). Parameters is
// the "parameters" attribute verbatim (arbitrary JSON).
type Extension struct {
	Parameters    any
	Extra         map[string]any
	ExtensionType string
	ExtensionKey  string
	Text          string
	Layout        string
	LocalID       string
	Marks         []Mark
}

// InlineExtension is an ADF inlineExtension leaf (an inline macro).
type InlineExtension struct {
	Parameters    any
	Extra         map[string]any
	ExtensionType string
	ExtensionKey  string
	Text          string
	LocalID       string
	Marks         []Mark
}

// BodiedExtension is an ADF bodiedExtension block (a macro with one
// rich-text body).
type BodiedExtension struct {
	Parameters    any
	Extra         map[string]any
	ExtensionType string
	ExtensionKey  string
	Text          string
	Layout        string
	LocalID       string
	Content       []Node
	Marks         []Mark
}

// MultiBodiedExtension is an ADF multiBodiedExtension block (a macro
// with several extensionFrame bodies, e.g. tabs; stage-0 schema).
type MultiBodiedExtension struct {
	Parameters    any
	Extra         map[string]any
	ExtensionType string
	ExtensionKey  string
	Text          string
	Layout        string
	LocalID       string
	Content       []Node
	Marks         []Mark
}

// ExtensionFrame is an ADF extensionFrame: one body of a
// multiBodiedExtension (stage-0 schema).
type ExtensionFrame struct {
	Extra   map[string]any
	Content []Node
	Marks   []Mark
}

// SyncBlock is an ADF syncBlock leaf (a reference to a synced block).
type SyncBlock struct {
	Extra      map[string]any
	ResourceID string
	LocalID    string
	Marks      []Mark
}

// BodiedSyncBlock is an ADF bodiedSyncBlock (the source body of a
// synced block).
type BodiedSyncBlock struct {
	Extra      map[string]any
	ResourceID string
	LocalID    string
	Content    []Node
	Marks      []Mark
}

// Kind implements Node.
func (*Date) Kind() string { return "date" }

// Kind implements Node.
func (*Placeholder) Kind() string { return "placeholder" }

// Kind implements Node.
func (*Caption) Kind() string { return "caption" }

// Kind implements Node.
func (*BlockTaskItem) Kind() string { return "blockTaskItem" }

// Kind implements Node.
func (*LayoutSection) Kind() string { return "layoutSection" }

// Kind implements Node.
func (*LayoutColumn) Kind() string { return "layoutColumn" }

// Kind implements Node.
func (*Extension) Kind() string { return "extension" }

// Kind implements Node.
func (*InlineExtension) Kind() string { return "inlineExtension" }

// Kind implements Node.
func (*BodiedExtension) Kind() string { return "bodiedExtension" }

// Kind implements Node.
func (*MultiBodiedExtension) Kind() string { return "multiBodiedExtension" }

// Kind implements Node.
func (*ExtensionFrame) Kind() string { return "extensionFrame" }

// Kind implements Node.
func (*SyncBlock) Kind() string { return "syncBlock" }

// Kind implements Node.
func (*BodiedSyncBlock) Kind() string { return "bodiedSyncBlock" }

func (*Date) adfNode()                 {}
func (*Placeholder) adfNode()          {}
func (*Caption) adfNode()              {}
func (*BlockTaskItem) adfNode()        {}
func (*LayoutSection) adfNode()        {}
func (*LayoutColumn) adfNode()         {}
func (*Extension) adfNode()            {}
func (*InlineExtension) adfNode()      {}
func (*BodiedExtension) adfNode()      {}
func (*MultiBodiedExtension) adfNode() {}
func (*ExtensionFrame) adfNode()       {}
func (*SyncBlock) adfNode()            {}
func (*BodiedSyncBlock) adfNode()      {}

// MarshalJSON implements json.Marshaler.
func (n *Date) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *Placeholder) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *Caption) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *BlockTaskItem) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *LayoutSection) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *LayoutColumn) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *Extension) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *InlineExtension) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *BodiedExtension) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *MultiBodiedExtension) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *ExtensionFrame) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *SyncBlock) MarshalJSON() ([]byte, error) { return marshalNode(n) }

// MarshalJSON implements json.Marshaler.
func (n *BodiedSyncBlock) MarshalJSON() ([]byte, error) { return marshalNode(n) }
