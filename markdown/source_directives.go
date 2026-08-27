package markdown

import (
	"maps"

	directive "github.com/pmarschik/goldmark-directive"
	gast "github.com/yuin/goldmark/ast"
)

// DirectiveLevel names a directive's nesting form, and carries the colon
// count that opens it. The values are the dialect's own vocabulary (see
// skill/assets/references/syntax.md) and are stable: a consumer that speaks
// JSON to an editor sends them over the wire.
type DirectiveLevel int

const (
	// DirectiveText is an inline `:name[label]{attrs}` run.
	DirectiveText DirectiveLevel = 1
	// DirectiveLeaf is a standalone `::name[label]{attrs}` line.
	DirectiveLeaf DirectiveLevel = 2
	// DirectiveContainer is a `:::name` … `:::` block.
	DirectiveContainer DirectiveLevel = 3
)

// Attr is one attribute of a directive AS WRITTEN in the source.
//
// The directive parser reports attributes as an unordered map, which is the
// right shape for reading a value and the wrong one for rewriting it: two
// occurrences of `.class` fold into one entry, a later `k=v` overwrites an
// earlier one, and neither the quoting nor the order survives. An Attr is
// therefore ONE WRITTEN OCCURRENCE — `{.a .b}` yields two Attrs, both keyed
// "class" — so a rewriter can splice exactly the bytes it means.
//
// All three spans are TIGHT. See Directive.AttrSpans for why.
type Attr struct {
	// Key is the attribute name the parser records. For the `#id` and
	// `.class` shorthands it is "id" or "class", which is NOT written in
	// the source; KeySpan is then the zero Span.
	Key string
	// Value is the attribute value the parser records, with quoting and
	// escaping already resolved. A bare key (`{collection}`) has an empty
	// Value.
	Value string
	// Span covers the whole occurrence as written — `color="green"`,
	// `#abc`, `.warn`, or a bare `collection` — with the shorthand marker
	// inside it and the separating whitespace outside.
	Span Span
	// KeySpan covers the key as written, or is the zero Span for a `#id`
	// or `.class` shorthand, whose key is spelled nowhere. The zero value
	// is unambiguous: at least `:n{` precedes any real key, so no key can
	// begin at offset 0.
	KeySpan Span
	// ValueSpan covers the value as written with the QUOTES OUTSIDE it, so
	// splicing a new value in keeps the quoting the old one needed. For a
	// shorthand it covers the text after the `#` or `.` marker.
	//
	// It is the zero Span for a bare key, which has no value written at
	// all. An explicitly empty value (`k=""`) instead has an empty span at
	// the offset where the value would go, so an Edit there inserts one.
	ValueSpan Span
}

// Directive is one dialect directive of a Markdown source.
//
// It is the parser's verdict on what a directive is — the same verdict the
// conversion path reaches — so a `:::info` inside a code block is not one
// and does not appear here.
//
// Directives nested inside a TEXT directive's label are NOT reported.
// goldmark parses a label against its own detached buffer, so positions
// found under one do not address this source and must never leak into a
// span.
//
// The field order is the one govet's fieldalignment wants, not the reading
// order.
type Directive struct {
	// Attrs are the attributes exactly as the dialect grammar reads them.
	// It is nil when the directive has no `{…}` block; it is the parser's
	// own map and must not be modified.
	Attrs map[string]string
	// Name is the directive name without its colons ("info", "media", …).
	Name string
	// AttrSpans locates every attribute as written, in written order. It
	// is nil when the directive has no `{…}` block, and ALSO nil when the
	// block was empty (`{}`) or could not be re-read into the same
	// attributes the parser recorded — see Source.Directives for that
	// fallback. AttrsSpan tells the three apart. Attrs is always
	// authoritative; AttrSpans only says where the bytes are.
	AttrSpans []Attr
	// Span covers the directive's FULL extent: the whole marker line for a
	// leaf, the whole run for a text directive, and — for a container —
	// the opening fence through the end of the matching closing fence.
	//
	// It is NOT widened to whole lines the way Source.CodeSpans and
	// Source.Headings widen theirs, and the difference is deliberate: this
	// extent is the parser's own (ContainerDirective.Span, and the
	// CloseFence that closes it), and it stops just BEFORE the line
	// terminator. A directive is the unit a consumer replaces wholesale,
	// and a span that swallowed the newline would join the directive to
	// the block after it on every such edit.
	//
	// An unclosed container — what the buffer looks like while the block
	// is still being typed — has no closing fence at all. Its Stop is the
	// end of the enclosing container, or the end of the source.
	Span Span
	// AttrsSpan covers the `{…}` attribute block WITH ITS BRACES, so a
	// caller can replace the block wholesale or insert an attribute just
	// inside the closing brace. It is the zero Span when the directive has
	// no block, or when the block could not be re-read (see AttrSpans).
	AttrsSpan Span
	// Level is DirectiveContainer, DirectiveLeaf, or DirectiveText.
	Level DirectiveLevel
}

// Directives returns every dialect directive in src, in document order. See
// Source.Directives for the exact rule.
//
// This is the one-shot form, for a caller that only needs to read. A caller
// that also edits takes a Source, so the spans and the splice share one
// parse and one buffer.
func Directives(src []byte) []Directive { return NewSource(src).Directives() }

// Directives returns every dialect directive in the source, in document
// order: ascending Start, with a container emitted BEFORE the directives
// nested inside it.
//
// Unlike the other views, directive spans NEST rather than tile — a
// container's Span covers its children's — so the result is a []Directive
// and not a Spans, whose contract forbids an overlap.
//
// # Attribute spans, and when they are absent
//
// The parser hands back an unordered map and keeps its attribute grammar
// unexported, so the written occurrences are re-read here from the bytes
// between the braces. The re-read is CHECKED: the attributes it recovers
// are folded back into a map with the parser's own rules (a later key wins,
// repeated `.class` values join with a space) and compared against what the
// parser recorded. On any disagreement the directive keeps its Attrs and
// reports NO spans — AttrSpans nil, AttrsSpan zero — because a span that
// might point at the wrong bytes is worse for a rewriter than no span at
// all.
func (s *Source) Directives() []Directive {
	if s.directivesDone {
		return s.directives
	}
	s.directives, s.directivesDone = collectDirectives(s.doc, s.src, len(s.src)), true
	return s.directives
}

// collectDirectives walks the goldmark tree in document order and emits one
// Directive per directive node. enclosingEnd is the extent an unclosed
// container nested here falls back to — the source length at the top level.
//
// The traversal is written out rather than delegated to gast.Walk because
// it threads that fallback down the tree, and because a text directive's
// label is parsed against its own detached source: goldmark keeps those
// inlines off the node's child list, so they are never reached here and
// their offsets can never leak out.
func collectDirectives(n gast.Node, src []byte, enclosingEnd int) []Directive {
	var out []Directive
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		childEnd := enclosingEnd
		switch v := c.(type) {
		case *directive.ContainerDirective:
			childEnd = closeFenceEnd(v, enclosingEnd)
			out = append(out, newDirective(
				src, DirectiveContainer, v.Name, v.Attrs,
				Span{Start: v.Span.Start, Stop: childEnd},
			))
		case *directive.LeafDirective:
			out = append(out, newDirective(
				src, DirectiveLeaf, v.Name, v.Attrs,
				Span{Start: v.Span.Start, Stop: v.Span.Stop},
			))
		case *directive.TextDirective:
			out = append(out, newDirective(
				src, DirectiveText, v.Name, v.Attrs,
				Span{Start: v.Span.Start, Stop: v.Span.Stop},
			))
		}
		out = append(out, collectDirectives(c, src, childEnd)...)
	}
	return out
}

// closeFenceEnd resolves a container directive's full extent.
// ContainerDirective.Span covers the OPENING FENCE LINE ONLY — the block's
// end is not known when it opens — so the extent ends at the matching
// CloseFence, which the parser emits as the container's next sibling. An
// unclosed container emits no CloseFence at all (a real input while the
// block is still being typed) and falls back to the enclosing extent.
func closeFenceEnd(cd *directive.ContainerDirective, fallback int) int {
	if fence, ok := cd.NextSibling().(*directive.CloseFence); ok {
		return fence.Span.Stop
	}
	return fallback
}

// newDirective assembles one Directive, resolving its attribute spans when
// they can be recovered from the source.
func newDirective(
	src []byte, level DirectiveLevel, name string, attrs map[string]string, span Span,
) Directive {
	d := Directive{Name: name, Attrs: attrs, Span: span, Level: level}
	if attrs == nil {
		return d
	}
	open := directiveAttrsOpen(src, span.Start)
	if open < 0 {
		return d
	}
	written, stop, ok := scanAttrSpans(src, open)
	if !ok || stop > span.Stop || !sameAttrs(written, attrs) {
		return d
	}
	d.AttrSpans = written
	d.AttrsSpan = Span{Start: open, Stop: stop}
	return d
}

// directiveAttrsOpen returns the offset of the `{` that opens the attribute
// block of the directive starting at start, or -1 when there is none.
//
// It re-walks the prefix the directive parser walked — optional indent,
// the colon run, the name, an optional `[label]` — because the parser
// records the block's CONTENTS and not its position. The grammar mirrored
// here is the package's own (scanDirectiveMarkerLine and the text
// directive's Parse), which keeps its scanners unexported; sameAttrs is
// what proves the mirror still agrees with it.
func directiveAttrsOpen(src []byte, start int) int {
	i := start
	if i < 0 || i >= len(src) {
		return -1
	}
	for i < len(src) && i-start < 3 && src[i] == ' ' {
		i++
	}
	colons := 0
	for i < len(src) && src[i] == ':' {
		colons++
		i++
	}
	if colons == 0 {
		return -1
	}
	nameEnd := scanDirectiveName(src, i)
	if nameEnd < 0 {
		return -1
	}
	i = nameEnd
	if next, ok := scanDirectiveLabel(src, i); ok {
		i = next
	}
	if i < len(src) && src[i] == '{' {
		return i
	}
	return -1
}

// scanDirectiveName returns the offset one past the name starting at src[i],
// or -1 when there is no valid name. A name is alphanumeric and may contain
// `-` and `_` runs, but may not END in one.
func scanDirectiveName(src []byte, i int) int {
	if i >= len(src) || !isDirectiveAlnumByte(src[i]) {
		return -1
	}
	end := i + 1
	for end < len(src) && (isDirectiveAlnumByte(src[end]) || src[end] == '-' || src[end] == '_') {
		end++
	}
	if src[end-1] == '-' || src[end-1] == '_' {
		return -1
	}
	return end
}

// scanDirectiveLabel returns the offset one past a complete `[label]`
// starting at src[i]. The label ends at the first `]` on the same line.
func scanDirectiveLabel(src []byte, i int) (next int, ok bool) {
	if i >= len(src) || src[i] != '[' {
		return i, false
	}
	j := i + 1
	for j < len(src) && src[j] != ']' && src[j] != '\n' && src[j] != '\r' {
		j++
	}
	if j >= len(src) || src[j] != ']' {
		return i, false
	}
	return j + 1, true
}

// scanAttrSpans reads the `{…}` block opening at src[open] into one Attr per
// written occurrence, and returns the offset one past the closing `}`.
//
// It mirrors the directive package's scanDirectiveAttributes byte for byte,
// down to which malformed blocks it rejects; the only addition is that it
// records where each piece was. Duplicate keys and repeated `.class` values
// are kept SEPARATE here — folding them is sameAttrs's job.
func scanAttrSpans(src []byte, open int) (attrs []Attr, next int, ok bool) {
	if open >= len(src) || src[open] != '{' {
		return nil, open, false
	}
	j := open + 1
	for {
		for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
			j++
		}
		if j >= len(src) || src[j] == '\n' || src[j] == '\r' {
			return nil, open, false
		}
		if src[j] == '}' {
			return attrs, j + 1, true
		}
		var (
			a     Attr
			after int
			valid bool
		)
		switch src[j] {
		case '#', '.':
			a, after, valid = scanAttrShorthandSpan(src, j)
		default:
			a, after, valid = scanAttrKeyValueSpan(src, j)
		}
		if !valid {
			return nil, open, false
		}
		attrs = append(attrs, a)
		j = after
	}
}

// scanAttrShorthandSpan reads a `#id` or `.class` shorthand at src[j].
func scanAttrShorthandSpan(src []byte, j int) (a Attr, next int, ok bool) {
	marker := src[j]
	start := j
	j++
	vStart := j
	for j < len(src) && !isAttrBoundary(src[j]) {
		j++
	}
	if j == vStart {
		return Attr{}, 0, false
	}
	key := "class"
	if marker == '#' {
		key = "id"
	}
	return Attr{
		Key:   key,
		Value: string(src[vStart:j]),
		Span:  Span{Start: start, Stop: j},
		// KeySpan stays zero: "id" and "class" are spelled nowhere.
		ValueSpan: Span{Start: vStart, Stop: j},
	}, j, true
}

// scanAttrKeyValueSpan reads a bare key or a `key=value` pair at src[j].
func scanAttrKeyValueSpan(src []byte, j int) (a Attr, next int, ok bool) {
	start := j
	for j < len(src) && !isAttrBoundary(src[j]) && src[j] != '=' {
		j++
	}
	if j == start {
		return Attr{}, 0, false
	}
	a = Attr{Key: string(src[start:j]), KeySpan: Span{Start: start, Stop: j}}
	if j >= len(src) || src[j] != '=' {
		// A bare key has no value written, so ValueSpan stays zero.
		a.Span = Span{Start: start, Stop: j}
		return a, j, true
	}
	value, valueSpan, after, valid := scanAttrValueSpan(src, j+1)
	if !valid {
		return Attr{}, 0, false
	}
	a.Value, a.ValueSpan = value, valueSpan
	a.Span = Span{Start: start, Stop: after}
	return a, after, true
}

// scanAttrValueSpan reads an attribute value at src[j], just past the `=`.
// A quoted value's span EXCLUDES its quotes, so a splice keeps them.
func scanAttrValueSpan(src []byte, j int) (value string, span Span, next int, ok bool) {
	if j < len(src) && (src[j] == '"' || src[j] == '\'') {
		quote := src[j]
		j++
		vStart := j
		for j < len(src) && src[j] != quote && src[j] != '\n' && src[j] != '\r' {
			j++
		}
		if j >= len(src) || src[j] != quote {
			return "", Span{}, 0, false
		}
		return string(src[vStart:j]), Span{Start: vStart, Stop: j}, j + 1, true
	}
	vStart := j
	for j < len(src) && !isAttrBoundary(src[j]) {
		j++
	}
	return string(src[vStart:j]), Span{Start: vStart, Stop: j}, j, true
}

// isAttrBoundary reports whether c ends an unquoted key or value. Mirrors
// the directive package's own predicate.
func isAttrBoundary(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' ||
		c == '{' || c == '}' || c == '"' || c == '\''
}

// sameAttrs reports whether the written occurrences fold back into exactly
// the map the parser recorded, under the parser's own rules: a later key
// overwrites an earlier one, and repeated `.class` values join with a space.
//
// This is the check that makes mirroring an unexported grammar safe. If the
// directive package ever changes how it reads a block, the fold stops
// matching and the spans are dropped rather than pointing somewhere wrong.
func sameAttrs(written []Attr, parsed map[string]string) bool {
	folded := make(map[string]string, len(written))
	for _, a := range written {
		if a.Key == "class" && a.KeySpan == (Span{}) {
			if existing, found := folded["class"]; found {
				folded["class"] = existing + " " + a.Value
				continue
			}
		}
		folded[a.Key] = a.Value
	}
	return maps.Equal(folded, parsed)
}
