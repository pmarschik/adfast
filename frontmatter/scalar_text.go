package frontmatter

import (
	"gopkg.in/yaml.v3"
)

// ParseSequenceSourceText is Parse with one difference: a plain
// (unquoted, untagged) number, boolean or timestamp that is an ITEM OF A
// SEQUENCE is returned as its source text, a string, instead of as the
// value the YAML decoder resolves it to.
//
// The decoder turns the unquoted `1.0` into the float 1, `2` into an
// int, `0x1F` into 31 and `2024-01-02` into a time.Time, and the spelling
// is gone: the list `[1.0, 2, "3.0", 1.50]` decodes to [1 2 "3.0" 1.5],
// where "1.0" can no longer be told from "1". For a list of names or
// labels (release versions, tags) the text the author typed is the only
// faithful name, so this entry point returns ["1.0" "2" "3.0" "1.50"].
//
// The rules:
//
//   - Only sequence items change, at any depth (a sequence under a key,
//     a sequence inside a mapping inside a sequence, nested sequences).
//     A scalar directly under a key stays typed: `points: 5.0` is still
//     the float 5, and `due: 2024-01-02` is still a time.Time.
//   - Every non-string scalar kind the decoder resolves is covered alike,
//     so a sequence item is either a string, nil or a structure: decimal,
//     hex, octal and underscored ints (`0x1F`, `0o17`, `1_000`), floats
//     (`1.50`, `1e3`, `.inf`), an int too large for int64, booleans
//     (`true`), and timestamps (`2024-01-02`).
//   - A null item (`~`, `null`, or an empty `-` slot) stays nil: an empty
//     slot is not a name. Quoted and block scalars are already strings
//     and are unchanged.
//   - An explicitly tagged item keeps its tag's type (`!!int 1` is the
//     int 1, `!!float 1` the float 1): the tag is the author asking for
//     that type.
//   - Aliases resolve as the decoder resolves them: an aliased sequence
//     yields the same text as the anchored one, an alias item to a scalar
//     yields that scalar's text, and an alias under a key to an anchored
//     sequence item stays typed. Merge keys, key precedence and duplicate
//     keys behave exactly as in Parse, because the decoder itself builds
//     the result.
//
// Everything else matches Parse: the fence handling, the strict error
// (the same error value Parse returns), the empty-block result, and the
// normalization of nested maps. The result is meant for reading. Do not
// feed it back to Render or Patch expecting the original types: a
// substituted item is a string, so it is written back as a quoted string
// ("1.0"). To edit a block and keep its authored text, use
// PatchPreserving, which never reads these values.
func ParseSequenceSourceText(rawBlock string) (map[string]any, error) {
	parsed, err := Parse(rawBlock)
	if err != nil || len(parsed) == 0 {
		return parsed, err
	}
	if withText, ok := decodeWithSequenceText(stripFence(rawBlock)); ok {
		return withText, nil
	}
	// Unreachable for a block Parse accepted; keep Parse's answer.
	return parsed, nil
}

// decodeWithSequenceText decodes inner YAML after marking its sequence
// items as text. ok is false when the node parse or the decode fails.
func decodeWithSequenceText(inner string) (map[string]any, bool) {
	var doc yaml.Node
	if yaml.Unmarshal([]byte(inner), &doc) != nil {
		return nil, false
	}
	markSequenceItemsAsText(&doc)
	var withText map[string]any
	if doc.Decode(&withText) != nil || withText == nil {
		return nil, false
	}
	return normalizeYAMLMap(withText), true
}

// markSequenceItemsAsText retags, in the node tree, every sequence item
// that is (or aliases) a plain non-string scalar as an explicit !!str
// holding its source text, so decoding the tree yields that text. The
// item is replaced by a copy rather than retagged in place: an anchored
// item may also be the target of an alias under a key, which must keep
// decoding typed.
//
// The walk does not follow aliases. Every node sits in the tree exactly
// once, and an alias decodes from the node it names, so visiting the
// tree reaches each sequence once and cannot loop.
func markSequenceItemsAsText(n *yaml.Node) {
	if n.Kind == yaml.SequenceNode {
		for i, item := range n.Content {
			if text, ok := plainNonStringScalarText(resolveAlias(item)); ok {
				n.Content[i] = &yaml.Node{
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Value:  text,
					Line:   item.Line,
					Column: item.Column,
				}
			}
		}
	}
	for _, child := range n.Content {
		if child.Kind != yaml.AliasNode {
			markSequenceItemsAsText(child)
		}
	}
}

// resolveAlias follows an alias node to the node it names.
func resolveAlias(n *yaml.Node) *yaml.Node {
	for n.Kind == yaml.AliasNode && n.Alias != nil {
		n = n.Alias
	}
	return n
}

// plainNonStringScalarText returns the source text of an unquoted,
// untagged scalar the decoder resolves to a number, a boolean or a
// timestamp. An explicit tag sets yaml.TaggedStyle, so a tagged scalar
// never matches.
func plainNonStringScalarText(n *yaml.Node) (string, bool) {
	if n.Kind != yaml.ScalarNode || n.Style != 0 {
		return "", false
	}
	switch n.ShortTag() {
	case "!!int", "!!float", "!!bool", "!!timestamp":
		return n.Value, true
	}
	return "", false
}
