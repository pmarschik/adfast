// Package debug provides human-readable dumps of the pivot Markdown AST
// (ast.Node trees) and ADF documents (adf.Doc) plus a type-tagged JSON
// encoding of the Markdown AST. It is a debugging aid: the output formats
// are not covered by any compatibility guarantee and may change between
// releases.
package debug

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/pmarschik/adfast/adf"
	"github.com/pmarschik/adfast/ast"
)

// Dump writes an indented tree of a Markdown AST: one line per node with
// its kind name and non-zero kind-specific fields, children indented below.
func Dump(w io.Writer, n ast.Node) {
	dumpNode(w, n, 0)
}

func dumpNode(w io.Writer, n ast.Node, depth int) {
	indent := strings.Repeat("  ", depth)
	if n == nil {
		fmt.Fprintf(w, "%snil\n", indent)
		return
	}
	line := indent + n.Kind()
	var lineSb37 strings.Builder
	for _, f := range nodeFields(n) {
		lineSb37.WriteString(" " + f.key + "=" + formatFieldValue(f.value))
	}
	line += lineSb37.String()
	fmt.Fprintln(w, line)
	for _, c := range ast.Children(n) {
		dumpNode(w, c, depth+1)
	}
}

// MarshalJSON encodes a Markdown AST as type-tagged JSON: every node is an
// object with a "kind" tag, its non-zero kind-specific fields inline, and a
// "children" array when it has children.
func MarshalJSON(n ast.Node) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeNodeJSON(&buf, n); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeNodeJSON(buf *bytes.Buffer, n ast.Node) error {
	if n == nil {
		buf.WriteString("null")
		return nil
	}
	buf.WriteString(`{"kind":`)
	if err := writeJSONValue(buf, n.Kind()); err != nil {
		return err
	}
	for _, f := range nodeFields(n) {
		buf.WriteByte(',')
		if err := writeJSONValue(buf, f.key); err != nil {
			return err
		}
		buf.WriteByte(':')
		if err := writeJSONValue(buf, f.value); err != nil {
			return err
		}
	}
	if kids := ast.Children(n); len(kids) > 0 {
		buf.WriteString(`,"children":[`)
		for i, c := range kids {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeNodeJSON(buf, c); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	}
	buf.WriteByte('}')
	return nil
}

func writeJSONValue(buf *bytes.Buffer, v any) error {
	encoded, err := json.Marshal(v)
	if err != nil {
		return err
	}
	buf.Write(encoded)
	return nil
}

// DumpADF writes an indented tree of an ADF document: one line per node
// with its type, text, attributes (sorted), and marks, children indented
// below.
func DumpADF(w io.Writer, doc adf.Doc) {
	fmt.Fprintf(w, "%s version=%d\n", doc.Type, doc.Version)
	for _, c := range doc.Content {
		dumpADFNode(w, c, 1)
	}
}

func dumpADFNode(w io.Writer, n adf.Node, depth int) {
	line := strings.Repeat("  ", depth) + n.Kind()
	if text := adf.NodeText(n); text != "" {
		line += " text=" + strconv.Quote(text)
	}
	if attrs := adf.NodeAttrs(n); len(attrs) > 0 {
		line += " attrs=" + formatAnyMap(attrs)
	}
	if marks := adf.NodeMarks(n); len(marks) > 0 {
		parts := make([]string, len(marks))
		for i, m := range marks {
			p := m.Kind()
			if attrs := adf.MarkAttrs(m); len(attrs) > 0 {
				p += formatAnyMap(attrs)
			}
			parts[i] = p
		}
		line += " marks=[" + strings.Join(parts, " ") + "]"
	}
	fmt.Fprintln(w, line)
	for _, c := range adf.NodeContent(n) {
		dumpADFNode(w, c, depth+1)
	}
}

// ---------------------------------------------------------------------------
// Field extraction (reflection over the concrete node structs)
// ---------------------------------------------------------------------------

type field struct {
	value any
	key   string
}

// nodeFields lists a node's non-zero kind-specific fields in declaration
// order: exported struct fields except Children, with embedded structs
// (e.g. ast.BlockSpacing) flattened.
func nodeFields(n ast.Node) []field {
	v := reflect.ValueOf(n)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	return structFields(v)
}

func structFields(v reflect.Value) []field {
	var out []field
	t := v.Type()
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() || f.Name == "Children" {
			continue
		}
		fv := v.Field(i)
		if f.Anonymous && fv.Kind() == reflect.Struct {
			out = append(out, structFields(fv)...)
			continue
		}
		if fv.IsZero() {
			continue
		}
		out = append(out, field{key: fieldKey(f.Name), value: fv.Interface()})
	}
	return out
}

// fieldKey lowercases a Go field name into its lowerCamel debug/JSON key:
// the leading uppercase run lowercases, keeping the run's last capital when
// it starts a new word ("URL" → "url", "ColSpan" → "colSpan").
func fieldKey(name string) string {
	r := []rune(name)
	n := 0
	for n < len(r) && unicode.IsUpper(r[n]) {
		n++
	}
	if n > 1 && n < len(r) {
		n--
	}
	for i := range n {
		r[i] = unicode.ToLower(r[i])
	}
	return string(r)
}

func formatFieldValue(v any) string {
	switch x := v.(type) {
	case string:
		return strconv.Quote(x)
	case *bool:
		return strconv.FormatBool(*x)
	case map[string]string:
		keys := slices.Sorted(maps.Keys(x))
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = k + "=" + strconv.Quote(x[k])
		}
		return "{" + strings.Join(parts, " ") + "}"
	default:
		return fmt.Sprint(v)
	}
}

func formatAnyMap(m map[string]any) string {
	keys := slices.Sorted(maps.Keys(m))
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s=%v", k, m[k])
	}
	return "{" + strings.Join(parts, " ") + "}"
}
