package frontmatter

import (
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// PatchPreserving merges updates into a raw frontmatter block while
// preserving the authored formatting of everything it does not touch:
// the original top-level key order, comments (head/line/foot), and the
// scalar style (quoting, block/flow) of untouched keys all survive
// byte-for-byte. It works on the YAML concrete syntax tree (yaml.Node)
// rather than a plain map, editing only the keys named in updates:
//
//   - a nil update value deletes that top-level key (and its value);
//   - an existing key's value is replaced in place (its own comments and
//     style are not retained — the value changed);
//   - a new key is appended after the existing keys.
//
// rawBlock may include or omit the "---" fence lines; the result is a
// "---"-fenced block (or the empty string when nothing remains). Only
// top-level keys are addressed; a map value is written as a fresh
// subtree. Prefer this over Patch when the block is hand-authored and
// its formatting matters; use Patch for the order-list-driven merge.
func PatchPreserving(rawBlock string, updates map[string]any) (string, error) {
	inner := stripFence(rawBlock)

	var doc yaml.Node
	haveDoc := false
	if strings.TrimSpace(inner) != "" {
		if err := yaml.Unmarshal([]byte(inner), &doc); err != nil {
			return "", err
		}
		haveDoc = doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 &&
			doc.Content[0].Kind == yaml.MappingNode
	}

	var mapping *yaml.Node
	if haveDoc {
		mapping = doc.Content[0]
	} else {
		mapping = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{mapping}}
	}

	for _, k := range sortedKeys(updates) {
		v := updates[k]
		idx := mappingIndex(mapping, k)
		if v == nil {
			if idx >= 0 {
				mapping.Content = append(mapping.Content[:idx], mapping.Content[idx+2:]...)
			}
			continue
		}
		valNode := &yaml.Node{}
		if err := valNode.Encode(v); err != nil {
			return "", err
		}
		if idx >= 0 {
			mapping.Content[idx+1] = valNode
		} else {
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}
			mapping.Content = append(mapping.Content, keyNode, valNode)
		}
	}

	if len(mapping.Content) == 0 {
		return "", nil
	}

	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return "---\n" + strings.TrimRight(buf.String(), "\n") + "\n---", nil
}

// mappingIndex returns the index of the key node for key in a mapping
// node's Content (key at i, value at i+1), or -1 if absent.
func mappingIndex(mapping *yaml.Node, key string) int {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return i
		}
	}
	return -1
}

// sortedKeys returns the keys of m in deterministic (lexical) order, so
// appending multiple new keys in one PatchPreserving call is
// byte-stable.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
