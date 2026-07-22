// Package frontmatter provides optional YAML frontmatter access for
// github.com/pmarschik/adfast. The adfast core is deliberately
// YAML-neutral: its FrontmatterProvider treats the leading metadata
// block as opaque bytes and keeps it verbatim on the AST as
// ast.Frontmatter.Value (delimiters included). This module is the seam
// for consumers who want structured access to that block — parse it to
// a map, render a map back to a block, patch or replace it — without
// coupling the core to a YAML implementation.
//
// The module never re-implements frontmatter boundary detection: that
// is the FrontmatterProvider's job on the way in and out of the AST.
// It only turns the raw block ⇄ map. A raw block is the metadata block
// as ast.Frontmatter.Value holds it, i.e. including the leading and
// trailing "---" fence lines; the parse helpers also accept a fenceless
// inner-YAML string.
//
// Ordering is a product concern, so Render/Patch/Replace take the
// top-level key order as a caller parameter (a list of dot-paths); the
// module only provides the mechanism. Pass nil for alphabetical order.
//
// Two patch strategies are offered:
//
//   - Patch merges updates and re-renders under the caller's order list.
//     It is NOT format-preserving (comments and scalar styles are not
//     retained); it is the simple, order-list-driven merge.
//   - PatchPreserving edits only the changed keys on the YAML CST, so
//     the original key order, comments, and scalar styles of untouched
//     keys survive byte-for-byte. Prefer it when you want to preserve
//     authored formatting.
package frontmatter

import (
	"slices"
	"sort"
	"strings"

	adfast_ast "github.com/pmarschik/adfast/ast"
	"gopkg.in/yaml.v3"
)

// Parse turns a raw frontmatter block into a map, using strict YAML
// parsing. rawBlock may include the leading/trailing "---" fence lines
// (as ast.Frontmatter.Value holds it) or be the fenceless inner YAML;
// the fence, if present, is stripped before parsing. Nested map[any]any
// values are normalized to map[string]any recursively. An empty block
// yields an empty map. A YAML syntax error is returned as-is; use
// ParseLenient for the permissive, never-erroring parse.
func Parse(rawBlock string) (map[string]any, error) {
	inner := stripFence(rawBlock)
	if strings.TrimSpace(inner) == "" {
		return map[string]any{}, nil
	}
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(inner), &parsed); err != nil {
		return nil, err
	}
	if parsed == nil {
		return map[string]any{}, nil
	}
	return normalizeYAMLMap(parsed), nil
}

// ParseLenient turns a raw frontmatter block into a map, falling back to
// a permissive line-by-line "key: value" scan when strict YAML parsing
// fails, so it never returns an error. This mirrors the historical
// behavior of Markdown-first tools that must not reject hand-authored,
// slightly-off frontmatter. rawBlock may include or omit the "---"
// fence lines.
func ParseLenient(rawBlock string) map[string]any {
	inner := stripFence(rawBlock)
	if strings.TrimSpace(inner) == "" {
		return map[string]any{}
	}

	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(inner), &parsed); err == nil && parsed != nil {
		return normalizeYAMLMap(parsed)
	}

	// Permissive fallback: line-by-line key:value
	fallback := map[string]any{}
	for rawLine := range strings.SplitSeq(inner, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `'"`)
		if key != "" {
			fallback[key] = val
		}
	}
	return fallback
}

// ParseNode is the bridge over the adfast core: it parses the raw block
// held on an ast.Frontmatter node (delimiters included). It is
// equivalent to Parse(n.Value). A nil node yields an empty map.
func ParseNode(n *adfast_ast.Frontmatter) (map[string]any, error) {
	if n == nil {
		return map[string]any{}, nil
	}
	return Parse(n.Value)
}

// Render serializes a map to a "---"-fenced YAML block with a 2-space
// indent. Top-level keys are ordered by the dot-path order list: keys
// covered by order appear in that order, keys not covered come first
// sorted alphabetically. A nil order sorts every key alphabetically. An
// empty map renders as the empty string.
func Render(m map[string]any, order []string) string {
	if len(m) == 0 {
		return ""
	}
	node := buildOrderedYAMLNode(m, order, "")
	if node == nil {
		return ""
	}
	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		return ""
	}
	return "---\n" + strings.TrimRight(buf.String(), "\n") + "\n---"
}

// Patch merges updates into Parse(rawBlock) and re-renders the block
// under the order list. A nil update value deletes that top-level key.
// Parsing is lenient (see ParseLenient), so a malformed block is
// recovered rather than rejected. The result is a "---"-fenced block
// (or the empty string when nothing remains). Patch is NOT
// format-preserving; use PatchPreserving to retain comments and styles.
func Patch(rawBlock string, updates map[string]any, order []string) (string, error) {
	inner := stripFence(rawBlock)

	existing := map[string]any{}
	if strings.TrimSpace(inner) != "" {
		if err := yaml.Unmarshal([]byte(inner), &existing); err != nil {
			existing = ParseLenient(inner)
		}
		if existing == nil {
			existing = map[string]any{}
		}
		existing = normalizeYAMLMap(existing)
	}

	for k, v := range updates {
		if v == nil {
			delete(existing, k)
		} else {
			existing[k] = v
		}
	}

	return Render(existing, order), nil
}

// Replace discards the raw block's existing content and renders m as the
// new block under the order list (see Render). rawBlock is accepted for
// call-site symmetry with Patch; its contents are not read.
func Replace(rawBlock string, m map[string]any, order []string) string {
	_ = rawBlock
	return Render(m, order)
}

// Get retrieves the nested value at path, or nil if any segment is
// missing or a non-map is encountered along the way.
func Get(m map[string]any, path []string) any {
	var cur any = m
	for _, key := range path {
		sub, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = sub[key]
	}
	return cur
}

// Set sets the nested value at path, creating intermediate maps as
// needed.
func Set(m map[string]any, path []string, v any) {
	cur := m
	for i := range len(path) - 1 {
		key := path[i]
		next, ok := cur[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[key] = next
		}
		cur = next
	}
	cur[path[len(path)-1]] = v
}

// Remove deletes the nested key at path and prunes any parent maps left
// empty by the deletion.
func Remove(m map[string]any, path []string) {
	if len(path) == 0 {
		return
	}
	type frame struct {
		obj map[string]any
		key string
	}
	var stack []frame
	cur := m
	for i := range len(path) - 1 {
		key := path[i]
		next, ok := cur[key].(map[string]any)
		if !ok {
			return
		}
		stack = append(stack, frame{obj: cur, key: key})
		cur = next
	}
	delete(cur, path[len(path)-1])

	// Clean up empty parents.
	for _, v := range slices.Backward(stack) {
		f := v
		child, ok := f.obj[f.key].(map[string]any)
		if !ok {
			break
		}
		if len(child) != 0 {
			break
		}
		delete(f.obj, f.key)
	}
}

// KeyOrder returns the top-level keys of a YAML mapping in their
// authored (insertion) order. rawYAML may include or omit the "---"
// fence lines. A blank or non-mapping input yields nil. This is the
// natural source for the order argument to Render/Patch: read the order
// from an existing block, then render an updated map in that order.
func KeyOrder(rawYAML string) []string {
	inner := stripFence(rawYAML)
	if strings.TrimSpace(inner) == "" {
		return nil
	}
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(inner), &node); err != nil ||
		node.Kind != yaml.DocumentNode || len(node.Content) == 0 {
		return nil
	}
	mapping := node.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return nil
	}
	keys := make([]string, 0, len(mapping.Content)/2)
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		keys = append(keys, mapping.Content[i].Value)
	}
	return keys
}

// stripFence removes the leading and trailing "---" fence lines from a
// standalone frontmatter block, returning the inner YAML. A fenceless
// input (no leading "---" line) is returned unchanged, so callers that
// already hold the inner YAML pay nothing. This is deliberately the
// only boundary handling in the module: splitting a whole document into
// frontmatter and body is the FrontmatterProvider's job.
func stripFence(block string) string {
	trimmed := strings.TrimLeft(block, "\r\n")
	if !strings.HasPrefix(trimmed, "---") {
		return block
	}
	// The dashes must terminate a line to be an opening fence.
	after := trimmed[3:]
	if after != "" && after[0] != '\n' && after[0] != '\r' {
		return block
	}
	_, after, ok := strings.Cut(trimmed, "\n")
	if !ok {
		return ""
	}
	inner := after
	lines := strings.Split(inner, "\n")
	for i, ln := range lines {
		if strings.TrimRight(ln, "\r") == "---" {
			return strings.Join(lines[:i], "\n")
		}
	}
	return strings.TrimRight(inner, "\r\n")
}

// buildOrderedYAMLNode converts a map into a YAML mapping node whose
// keys are ordered per Render's order list. prefix is the dot-joined
// path of the enclosing keys.
func buildOrderedYAMLNode(m map[string]any, order []string, prefix string) *yaml.Node {
	rankOf := func(path string) int {
		for i, p := range order {
			if p == path || strings.HasPrefix(p, path+".") {
				return i
			}
		}
		return -1
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		pi := keys[i]
		pj := keys[j]
		if prefix != "" {
			pi = prefix + "." + pi
			pj = prefix + "." + pj
		}
		ri, rj := rankOf(pi), rankOf(pj)
		if (ri < 0) != (rj < 0) {
			return ri < 0 // unbound keys first
		}
		if ri >= 0 && rj >= 0 && ri != rj {
			return ri < rj
		}
		return keys[i] < keys[j]
	})

	mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, k := range keys {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}
		childPath := k
		if prefix != "" {
			childPath = prefix + "." + k
		}
		var valNode *yaml.Node
		if sub, ok := m[k].(map[string]any); ok {
			valNode = buildOrderedYAMLNode(sub, order, childPath)
		} else {
			valNode = &yaml.Node{}
			if err := valNode.Encode(m[k]); err != nil {
				continue
			}
		}
		mapping.Content = append(mapping.Content, keyNode, valNode)
	}
	return mapping
}

// normalizeYAMLMap ensures all nested maps are map[string]any (not
// map[any]any).
func normalizeYAMLMap(m map[string]any) map[string]any {
	for k, v := range m {
		m[k] = normalizeYAMLValue(v)
	}
	return m
}

func normalizeYAMLValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return normalizeYAMLMap(val)
	case map[any]any:
		out := make(map[string]any, len(val))
		for k, v := range val {
			key, ok := k.(string)
			if !ok {
				continue
			}
			out[key] = normalizeYAMLValue(v)
		}
		return out
	case []any:
		for i, item := range val {
			val[i] = normalizeYAMLValue(item)
		}
		return val
	default:
		return v
	}
}
