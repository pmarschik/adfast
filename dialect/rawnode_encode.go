package dialect

import (
	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/extension"
)

// encodeRawNodeData builds the rawNode directive's "data" attribute: the
// JSON encoding (EncodeJSONAttr — no parallel encoder) of an envelope
// holding the unknown node's Attrs and Text, the two RawNode fields the
// directive's type/Children do not already carry. Absent (ok=false) when
// both are empty, so a node with neither need not spell an empty
// "data={}" on every instance.
func encodeRawNodeData(attrs map[string]any, text string) (string, bool) {
	if len(attrs) == 0 && text == "" {
		return "", false
	}
	envelope := map[string]any{}
	if len(attrs) > 0 {
		envelope["attrs"] = attrs
	}
	if text != "" {
		envelope["text"] = text
	}
	return EncodeJSONAttr(envelope)
}

// decodeRawNodeData reverses encodeRawNodeData. An empty s is the
// valid-empty case (no Attrs, no Text), not corruption. ok is false when s
// is non-empty but does not parse into the envelope shape encodeRawNodeData
// produces — the "fail loudly" signal RawNodeCorrupted reports (see
// rawnode.go): DecodeJSONAttr already degrades a value that is not JSON at
// all to a bare string, so that case surfaces here as "not a map" rather
// than panicking or guessing a shape for it.
func decodeRawNodeData(s string) (attrs map[string]any, text string, ok bool) {
	if s == "" {
		return nil, "", true
	}
	envelope, isMap := DecodeJSONAttr(s).(map[string]any)
	if !isMap {
		return nil, "", false
	}
	if raw, present := envelope["attrs"]; present {
		attrs, isMap = raw.(map[string]any)
		if !isMap {
			return nil, "", false
		}
	}
	if raw, present := envelope["text"]; present {
		t, isString := raw.(string)
		if !isString {
			return nil, "", false
		}
		text = t
	}
	return attrs, text, true
}

// EncodeADF implements extension.Node: a RawBlock with a corrupted payload
// (see RawNodeCorrupted) dissolves into its decoded children — the same
// "a wrapper missing its required payload degrades into its content"
// shape BodiedExtension takes — rather than inventing a node the
// hand-edited attribute no longer describes; the caller has already
// reported this loudly (EncodeADF has no sink of its own).
func (n *RawBlock) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	nodeType := n.Attrs["type"]
	attrs, text, ok := decodeRawNodeData(n.Attrs["data"])
	if nodeType == "" || !ok {
		return ctx.EncodeBlocks(n.Children)
	}
	return []adf.Node{&adf.RawNode{
		Type:    nodeType,
		Attrs:   attrs,
		Text:    text,
		Content: ctx.EncodeBlocks(n.Children),
	}}
}

// EncodeADF implements extension.Node: a corrupted payload drops the node
// — there is no content to dissolve into (see RawBlock.EncodeADF).
func (n *RawLeaf) EncodeADF(_ extension.EncodeContext) []adf.Node {
	nodeType := n.Attrs["type"]
	attrs, text, ok := decodeRawNodeData(n.Attrs["data"])
	if nodeType == "" || !ok {
		return nil
	}
	return []adf.Node{&adf.RawNode{Type: nodeType, Attrs: attrs, Text: text}}
}

// EncodeADF implements extension.Node: a corrupted payload dissolves into
// the decoded inline children (see RawBlock.EncodeADF), or drops when
// there are none.
func (n *RawText) EncodeADF(ctx extension.EncodeContext) []adf.Node {
	nodeType := n.Attrs["type"]
	attrs, text, ok := decodeRawNodeData(n.Attrs["data"])
	if nodeType == "" || !ok {
		return ctx.EncodeInlines(n.Children)
	}
	return []adf.Node{&adf.RawNode{
		Type:    nodeType,
		Attrs:   attrs,
		Text:    text,
		Content: ctx.EncodeInlines(n.Children),
	}}
}
