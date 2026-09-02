package markdown

import (
	"fmt"
	"maps"
	"strconv"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/dialect"
)

// directiveAttrs returns the attribute payload of the first block of a
// parsed document, for each container form the dialect can produce: the
// generic ast.ContainerDirective and the two typed containers that keep
// a raw attribute payload (panel, expand).
func directiveAttrs(t *testing.T, root ast.Node) map[string]string {
	t.Helper()
	kids := ast.Children(root)
	if len(kids) == 0 {
		t.Fatalf("document has no blocks: %#v", root)
	}
	switch n := kids[0].(type) {
	case *ast.ContainerDirective:
		return n.Attrs
	case *dialect.Panel:
		return n.Attrs
	case *dialect.Expand:
		return n.Attrs
	default:
		t.Fatalf("first block is not a container directive: %T", kids[0])
		return nil
	}
}

// A container directive keeps its attribute block across a
// markdown → AST → markdown round trip: whatever the renderer writes,
// Parse reads back as the same attribute map. The generic container form
// dropped every attribute before this test (it rendered ":::sidebar" for
// ":::sidebar{a=\"1\"}"), and the two typed containers that hold a raw
// attribute payload — panel and expand — dropped theirs the same way, so
// any re-render of an authored document silently deleted what the
// directive was configured with.
//
// The assertion is the round trip rather than a golden fence line: the
// spelling the renderer picks is free to change, but Parse must read it
// back unchanged, and a second render must be a fixed point.
func TestRender_ContainerDirectiveKeepsItsAttributes(t *testing.T) {
	cases := []struct {
		want map[string]string
		name string
		src  string
	}{
		{
			name: "a quoted value",
			src:  ":::sidebar{color=\"green\"}\nbody\n:::\n",
			want: map[string]string{"color": "green"},
		},
		{
			name: "a value carrying a single quote",
			src:  ":::sidebar{note=\"it's here\"}\nbody\n:::\n",
			want: map[string]string{"note": "it's here"},
		},
		{
			name: "a value carrying a double quote",
			src:  ":::sidebar{payload='{\"k\":\"v\"}'}\nbody\n:::\n",
			want: map[string]string{"payload": `{"k":"v"}`},
		},
		{
			name: "the #id shorthand",
			src:  ":::sidebar{#intro}\nbody\n:::\n",
			want: map[string]string{"id": "intro"},
		},
		{
			name: "the .class shorthand",
			src:  ":::sidebar{.warn}\nbody\n:::\n",
			want: map[string]string{"class": "warn"},
		},
		{
			name: "several attributes",
			src:  ":::sidebar{#intro .warn a=\"1\" b=\"2\" bare}\nbody\n:::\n",
			want: map[string]string{"id": "intro", "class": "warn", "a": "1", "b": "2", "bare": ""},
		},
		{
			name: "a panel",
			src:  ":::info{color=\"green\" #p1}\nbody\n:::\n",
			want: map[string]string{"color": "green", "id": "p1"},
		},
		{
			name: "an expand with a label",
			src:  ":::expand[Title]{#x .open}\nbody\n:::\n",
			want: map[string]string{"id": "x", "class": "open"},
		},
		{
			name: "a nested container",
			src:  "::::sidebar{a=\"1\"}\n:::note{b=\"2\"}\nbody\n:::\n::::\n",
			want: map[string]string{"a": "1"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := directiveAttrs(t, Parse([]byte(tc.src))); !maps.Equal(got, tc.want) {
				t.Fatalf("the source itself does not parse to the expected attrs: got %v, want %v", got, tc.want)
			}

			out := Render(Parse([]byte(tc.src)))
			got := directiveAttrs(t, Parse([]byte(out)))
			if !maps.Equal(got, tc.want) {
				t.Errorf("attrs lost in the round trip through %q: got %v, want %v", out, got, tc.want)
			}
			if again := Render(Parse([]byte(out))); again != out {
				t.Errorf("render is not a fixed point: %q then %q", out, again)
			}
		})
	}
}

// A nested container keeps the attributes of every level, not only of the
// outermost one.
func TestRender_NestedContainerDirectiveKeepsItsAttributes(t *testing.T) {
	src := "::::sidebar{a=\"1\"}\n:::note{b=\"2\"}\nbody\n:::\n::::\n"
	out := Render(Parse([]byte(src)))

	outer, ok := ast.Children(Parse([]byte(out)))[0].(*ast.ContainerDirective)
	if !ok {
		t.Fatalf("outer container did not survive: %q", out)
	}
	if !maps.Equal(outer.Attrs, map[string]string{"a": "1"}) {
		t.Errorf("outer attrs: got %v, want map[a:1]", outer.Attrs)
	}
	inner, ok := ast.Children(outer)[0].(*dialect.Panel)
	if !ok {
		t.Fatalf("inner panel did not survive: %q", out)
	}
	if !maps.Equal(inner.Attrs, map[string]string{"b": "2"}) {
		t.Errorf("inner attrs: got %v, want map[b:2]", inner.Attrs)
	}
}

// PIN (preserved behavior, not a fix): the quote style writeDirectiveAttrValue
// picks for every shape a value can take, and exactly what Parse gives back for
// it. The dialect has no escape inside a quoted attribute value, so a value
// carrying BOTH quote characters has no lossless spelling at all: it falls back
// to double quotes with each " written as &quot;, which dialect.DecodeJSONAttr
// decodes (the JSON payload the fallback exists for) but the attribute parse
// does not. This test exists so that the contract stated on
// writeDirectiveAttrValue can never drift from the behavior again.
func TestRender_DirectiveAttrValueQuoting(t *testing.T) {
	cases := []struct {
		name string
		// value is the attribute value handed to the renderer.
		value string
		// wantRendered is the whole leaf directive line it writes.
		wantRendered string
		// wantParsedBack is what Parse reads back for the attribute —
		// equal to value for every spelling that is lossless.
		wantParsedBack string
	}{
		{
			name:           "no quote at all",
			value:          "green",
			wantRendered:   "::x{k=\"green\"}\n",
			wantParsedBack: "green",
		},
		{
			name:           "a single quote only, so double quotes hold it",
			value:          "it's",
			wantRendered:   "::x{k=\"it's\"}\n",
			wantParsedBack: "it's",
		},
		{
			name:           "a double quote only, so single quotes hold it",
			value:          `{"k":"v"}`,
			wantRendered:   "::x{k='{\"k\":\"v\"}'}\n",
			wantParsedBack: `{"k":"v"}`,
		},
		{
			name:  "both quote characters, which no spelling holds",
			value: `{"k":"it's"}`,
			// LOSSY on purpose: neither quote can enclose the value and
			// the dialect has no escape, so the double quotes are written
			// as &quot; and Parse hands the reference back verbatim.
			wantRendered:   "::x{k=\"{&quot;k&quot;:&quot;it's&quot;}\"}\n",
			wantParsedBack: `{&quot;k&quot;:&quot;it's&quot;}`,
		},
		{
			name:           "empty, which renders as a bare attribute name",
			value:          "",
			wantRendered:   "::x{k}\n",
			wantParsedBack: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := &ast.Root{Children: []ast.Node{
				&ast.LeafDirective{Name: "x", Attrs: map[string]string{"k": tc.value}},
			}}
			out := Render(root)
			if out != tc.wantRendered {
				t.Fatalf("rendered %q, want %q", out, tc.wantRendered)
			}

			leaf, ok := ast.Children(Parse([]byte(out)))[0].(*ast.LeafDirective)
			if !ok {
				t.Fatalf("%q does not re-parse as a leaf directive", out)
			}
			if got := leaf.Attrs["k"]; got != tc.wantParsedBack {
				t.Errorf("Parse read back %q, want %q", got, tc.wantParsedBack)
			}
			if again := Render(Parse([]byte(out))); again != out {
				t.Errorf("render is not a fixed point: %q then %q", out, again)
			}
		})
	}
}

// FIX: an id whose value the {#id} shortcut cannot spell falls back to the
// long form. The shortcut token ends at the first attribute-boundary byte
// (space, tab, CR, LF, a brace, or either quote character), so a value
// carrying one is either truncated ({#a b} re-parses as id="a" plus a bare
// attribute "b") or invalidates the whole block ({#a}b} leaves the
// directive with no attributes at all). The renderer wrote the shortcut
// unconditionally, so every such id was lost on re-parse.
//
// The class half of each case is a PIN (preserved behavior): the renderer
// never wrote the {.class} shortcut, so a class already took the long
// form. It is asserted here so the two shorthand-shaped keys can never
// drift apart.
//
// The hazard is shared by all three directive forms, because they all
// serialize their attributes through writeDirectiveAttrs — so each case
// runs against the text, leaf and container forms.
func TestRender_DirectiveShorthandFallsBackWhenItCannotSpellTheValue(t *testing.T) {
	values := []struct {
		name  string
		value string
	}{
		{name: "a space", value: "a b"},
		{name: "a closing brace", value: "a}b"},
		{name: "a double quote", value: `a"b`},
		{name: "a single quote", value: "a'b"},
		{name: "an opening brace", value: "a{b"},
		{name: "a tab", value: "a\tb"},
		{name: "a leading space", value: " ab"},
		{name: "a trailing space", value: "ab "},
		{name: "only a space", value: " "},
		{name: "spellable, so the shortcut is kept", value: "intro"},
	}
	keys := []string{"id", "class"}

	for _, v := range values {
		for _, key := range keys {
			t.Run(v.name+" in the "+key, func(t *testing.T) {
				want := map[string]string{key: v.value}
				for _, form := range directiveForms {
					got, out := form.roundTrip(t, want)
					if !maps.Equal(got, want) {
						t.Errorf("%s form: attrs lost in the round trip through %q: got %v, want %v",
							form.name, out, got, want)
					}
				}
			})
		}
	}
}

// FIX: the shortcut is still taken when it can spell the value, so the
// fallback does not cost the compact spelling in the common case.
func TestRender_DirectiveIDKeepsTheShorthandWhenItSpellsTheValue(t *testing.T) {
	root := &ast.Root{Children: []ast.Node{
		&ast.LeafDirective{Name: "x", Attrs: map[string]string{"id": "intro"}},
	}}
	if out := Render(root); out != "::x{#intro}\n" {
		t.Errorf("rendered %q, want \"::x{#intro}\\n\"", out)
	}
}

// FIX: an id explicitly set to the empty string used to be dropped
// outright — the shortcut needs a non-empty token, and the long-form loop
// skipped the id key unconditionally, so nothing was written for it. It
// now takes the bare-key form every other empty-valued attribute takes.
func TestRender_DirectiveEmptyIDIsNotDropped(t *testing.T) {
	root := &ast.Root{Children: []ast.Node{
		&ast.LeafDirective{Name: "x", Attrs: map[string]string{"id": ""}},
	}}
	out := Render(root)
	if out != "::x{id}\n" {
		t.Fatalf("rendered %q, want \"::x{id}\\n\"", out)
	}
	leaf, ok := ast.Children(Parse([]byte(out)))[0].(*ast.LeafDirective)
	if !ok {
		t.Fatalf("%q does not re-parse as a leaf directive", out)
	}
	if !maps.Equal(leaf.Attrs, map[string]string{"id": ""}) {
		t.Errorf("Parse read back %v, want map[id:]", leaf.Attrs)
	}
}

// FIX: an unspellable id sits beside the other attributes without
// disturbing them — the long-form id joins the sorted key run.
func TestRender_DirectiveUnspellableIDKeepsItsNeighbours(t *testing.T) {
	want := map[string]string{"id": "a b", "class": "warn", "a": "1", "bare": ""}
	for _, form := range directiveForms {
		got, out := form.roundTrip(t, want)
		if !maps.Equal(got, want) {
			t.Errorf("%s form: attrs lost in the round trip through %q: got %v, want %v",
				form.name, out, got, want)
		}
	}
}

// FIX: an attribute value carrying a line ending is dropped, and the rest
// of the block is written as usual.
//
// A quoted value is read up to its own quote character and stops dead at a
// CR or an LF (scanAttrValue), and an unterminated value does not spoil one
// attribute — it invalidates the WHOLE block, so the directive re-parsed
// with none of its attributes, or, in the text form, as ordinary paragraph
// text. Neither spelling can hold such a value: the shortcut refuses it
// (CR and LF are attribute-boundary bytes) and the long form has no escape
// for a line ending inside quotes. So the attribute drops, the way an
// unwritable heading id drops, and the attributes around it survive.
//
// The hazard is shared by all three directive forms, because they all
// serialize their attributes through writeDirectiveAttrs.
func TestRender_DirectiveAttrValueWithALineEndingIsDropped(t *testing.T) {
	values := []struct {
		name  string
		value string
	}{
		{name: "an LF", value: "a\nb"},
		{name: "a CR", value: "a\rb"},
		{name: "a CRLF", value: "a\r\nb"},
		{name: "a leading LF", value: "\nab"},
		{name: "a trailing LF", value: "ab\n"},
		{name: "only an LF", value: "\n"},
		{name: "only a CR", value: "\r"},
		{name: "an LF beside a quote the fallback would escape", value: "a\n\"b'c"},
	}
	// The id key takes the shortcut path, so it is covered alongside the
	// ordinary key rather than assumed to behave like it.
	keys := []string{"k", "id", "class"}

	for _, v := range values {
		for _, key := range keys {
			t.Run(v.name+" in the "+key, func(t *testing.T) {
				attrs := map[string]string{key: v.value, "keep": "1", "bare": ""}
				want := map[string]string{"keep": "1", "bare": ""}
				for _, form := range directiveForms {
					got, out := form.roundTrip(t, attrs)
					if !maps.Equal(got, want) {
						t.Errorf("%s form: round trip through %q: got %v, want %v",
							form.name, out, got, want)
					}
				}
			})
		}
	}
}

// FIX: dropping the only attribute drops the block with it, rather than
// writing an empty "{}" — which re-parses as no attributes anyway, so the
// next render would write nothing and the spelling would not be a fixed
// point.
func TestRender_DirectiveDropsTheBlockWhenNoAttributeSurvives(t *testing.T) {
	root := &ast.Root{Children: []ast.Node{
		&ast.LeafDirective{Name: "x", Attrs: map[string]string{"k": "a\nb"}},
	}}
	out := Render(root)
	if out != "::x\n" {
		t.Fatalf("rendered %q, want \"::x\\n\"", out)
	}
	leaf, ok := ast.Children(Parse([]byte(out)))[0].(*ast.LeafDirective)
	if !ok {
		t.Fatalf("%q does not re-parse as a leaf directive", out)
	}
	if len(leaf.Attrs) != 0 {
		t.Errorf("Parse read back %v, want no attributes", leaf.Attrs)
	}
}

// PIN (preserved behavior): every byte a quoted value CAN hold still
// round-trips, so the line-ending guard does not cost the values that were
// always writable. A tab and the brace characters are attribute boundaries
// only outside quotes; the quotes themselves are handled by
// writeDirectiveAttrValue's quote choice.
func TestRender_DirectiveAttrValueKeepsWhatQuotesCanHold(t *testing.T) {
	values := []struct {
		name  string
		value string
	}{
		{name: "a space", value: "a b"},
		{name: "a tab", value: "a\tb"},
		{name: "braces", value: "a{b}c"},
		{name: "a double quote", value: `a"b`},
		{name: "a single quote", value: "a'b"},
		{name: "an equals sign", value: "a=b"},
	}

	for _, v := range values {
		t.Run(v.name, func(t *testing.T) {
			want := map[string]string{"k": v.value, "keep": "1"}
			for _, form := range directiveForms {
				got, out := form.roundTrip(t, want)
				if !maps.Equal(got, want) {
					t.Errorf("%s form: round trip through %q: got %v, want %v",
						form.name, out, got, want)
				}
			}
		})
	}
}

// FIX: an attribute KEY the block cannot spell is dropped, and the other
// attributes of the same block survive intact.
//
// The renderer wrote every map key verbatim, so a key carrying an
// attribute-boundary byte, an "=", or nothing at all corrupted the block
// the same way an unspellable id did: a key runs to the first boundary
// byte or to the "=" that opens its value (scanAttrKeyValue), so
// {a b="1"} splits into a bare "a" plus b="1", and a key holding a brace,
// a quote or an "=" leaves the block malformed and the directive with no
// attributes at all.
//
// A key opening with "#" or "." is the third shape, and it is the parser's
// dispatch rather than the key scan that decides it: the block reader
// branches on the FIRST byte of each attribute, so such a key is read as
// the id/class shortcut. It either invalidates the block (#k="v") or,
// worse, silently arrives under a different name (a bare #k comes back as
// id="k").
//
// The hazard is shared by all three directive forms, because they all
// serialize their attributes through writeDirectiveAttrs.
func TestRender_DirectiveUnspellableAttrKeyIsDropped(t *testing.T) {
	keys := []struct {
		name string
		key  string
	}{
		{name: "a space", key: "a b"},
		{name: "a tab", key: "a\tb"},
		{name: "an opening brace", key: "a{b"},
		{name: "a closing brace", key: "a}b"},
		{name: "a double quote", key: `a"b`},
		{name: "a single quote", key: "a'b"},
		{name: "an equals sign", key: "a=b"},
		{name: "an LF", key: "a\nb"},
		{name: "a CR", key: "a\rb"},
		{name: "a leading space", key: " ab"},
		{name: "a trailing space", key: "ab "},
		{name: "nothing at all", key: ""},
		{name: "a leading hash, which opens the id shortcut", key: "#ab"},
		{name: "a leading dot, which opens the class shortcut", key: ".ab"},
		{name: "only a hash", key: "#"},
		{name: "only a dot", key: "."},
	}
	// A bare key and a valued one reach the parser through different
	// branches, so both are exercised: the empty value is where a
	// shortcut-shaped key is read back under the wrong NAME rather than
	// taking the block down.
	values := []struct {
		name  string
		value string
	}{
		{name: "with a value", value: "1"},
		{name: "bare", value: ""},
	}

	for _, k := range keys {
		for _, v := range values {
			t.Run(k.name+" "+v.name, func(t *testing.T) {
				attrs := map[string]string{k.key: v.value, "keep": "1", "id": "intro"}
				want := map[string]string{"keep": "1", "id": "intro"}
				for _, form := range directiveForms {
					got, out := form.roundTrip(t, attrs)
					if !maps.Equal(got, want) {
						t.Errorf("%s form: round trip through %q: got %v, want %v",
							form.name, out, got, want)
					}
				}
			})
		}
	}
}

// PIN (preserved behavior): a key that CAN be spelled is still written, so
// the guard does not swallow the ordinary ones. The shortcut markers only
// matter as the first byte, and the boundary set is a set of bytes, so a
// multi-byte rune never trips it.
func TestRender_DirectiveSpellableAttrKeyIsKept(t *testing.T) {
	keys := []string{
		"k",
		"data-source",
		"data_source",
		"a#b",
		"a.b",
		"a#",
		"a.",
		"9lives",
		"Größe",
		"id",
		"class",
	}

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			want := map[string]string{key: "1", "keep": "2"}
			for _, form := range directiveForms {
				got, out := form.roundTrip(t, want)
				if !maps.Equal(got, want) {
					t.Errorf("%s form: round trip through %q: got %v, want %v",
						form.name, out, got, want)
				}
			}
		})
	}
}

// FIX: an unspellable key does not take the id shortcut down with it — the
// block is still written, and the id still uses the compact spelling.
func TestRender_DirectiveUnspellableAttrKeyKeepsTheIDShorthand(t *testing.T) {
	root := &ast.Root{Children: []ast.Node{
		&ast.LeafDirective{Name: "x", Attrs: map[string]string{"a b": "1", "id": "intro"}},
	}}
	if out := Render(root); out != "::x{#intro}\n" {
		t.Errorf("rendered %q, want \"::x{#intro}\\n\"", out)
	}
}

// FIX: a text directive whose attributes are ALL dropped still closes its
// form with the inert "{}" block, the way a directive that never had any
// does.
//
// The text form asks whether the node carries attributes, not whether the
// renderer wrote a block for them, so a map that drops away entirely left
// the form open: ":x" ran straight into a following "{y}", which the
// re-parse then read as the directive's own attribute block and swallowed
// the text. Dropping an unwritable attribute must not cost the content
// next to the directive.
func TestRender_TextDirectiveClosesTheFormWhenEveryAttributeIsDropped(t *testing.T) {
	cases := []struct {
		attrs map[string]string
		name  string
	}{
		{name: "no attributes at all", attrs: nil},
		{name: "only an unwritable value", attrs: map[string]string{"k": "a\nb"}},
		{name: "only an unwritable key", attrs: map[string]string{"a b": "1"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := &ast.Root{Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{
				&ast.TextDirective{Name: "x", Attrs: tc.attrs},
				&ast.Text{Value: "{y}"},
			}}}}
			out := Render(root)
			if out != ":x{}{y}\n" {
				t.Fatalf("rendered %q, want \":x{}{y}\\n\"", out)
			}
			assertRenderFixedPoint(t, out)
			para, ok := ast.Children(Parse([]byte(out)))[0].(*ast.Paragraph)
			if !ok {
				t.Fatalf("%q does not re-parse as a paragraph", out)
			}
			kids := ast.Children(para)
			if len(kids) != 2 {
				t.Fatalf("%q re-parsed to %d inline nodes, want the directive and the text beside it", out, len(kids))
			}
			dir, ok := kids[0].(*ast.TextDirective)
			if !ok {
				t.Fatalf("%q does not re-parse as a text directive", out)
			}
			if len(dir.Attrs) != 0 {
				t.Errorf("Parse read back %v, want no attributes", dir.Attrs)
			}
			if text, ok := kids[1].(*ast.Text); !ok || text.Value != "{y}" {
				t.Errorf("the text beside the directive did not survive: %#v", kids[1])
			}
		})
	}
}

// directiveForm round-trips an attribute map through one of the three
// directive forms: build a node carrying attrs, Render it, Parse the
// output back, and return the attributes that survived alongside the
// rendered text (for the failure message). It also asserts that a second
// render is a fixed point, since an unstable spelling is its own defect.
type directiveForm struct {
	roundTrip func(t *testing.T, attrs map[string]string) (got map[string]string, rendered string)
	name      string
}

var directiveForms = []directiveForm{
	{name: "text", roundTrip: roundTripTextDirectiveAttrs},
	{name: "leaf", roundTrip: roundTripLeafDirectiveAttrs},
	{name: "container", roundTrip: roundTripContainerDirectiveAttrs},
}

func roundTripTextDirectiveAttrs(t *testing.T, attrs map[string]string) (got map[string]string, rendered string) {
	t.Helper()
	root := &ast.Root{Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{
		&ast.TextDirective{Name: "x", Attrs: maps.Clone(attrs)},
	}}}}
	out := Render(root)
	assertRenderFixedPoint(t, out)
	para, ok := ast.Children(Parse([]byte(out)))[0].(*ast.Paragraph)
	if !ok {
		t.Fatalf("%q does not re-parse as a paragraph", out)
	}
	dir, ok := ast.Children(para)[0].(*ast.TextDirective)
	if !ok {
		t.Fatalf("%q does not re-parse as a text directive", out)
	}
	return dir.Attrs, out
}

func roundTripLeafDirectiveAttrs(t *testing.T, attrs map[string]string) (got map[string]string, rendered string) {
	t.Helper()
	root := &ast.Root{Children: []ast.Node{
		&ast.LeafDirective{Name: "x", Attrs: maps.Clone(attrs)},
	}}
	out := Render(root)
	assertRenderFixedPoint(t, out)
	leaf, ok := ast.Children(Parse([]byte(out)))[0].(*ast.LeafDirective)
	if !ok {
		t.Fatalf("%q does not re-parse as a leaf directive", out)
	}
	return leaf.Attrs, out
}

func roundTripContainerDirectiveAttrs(t *testing.T, attrs map[string]string) (got map[string]string, rendered string) {
	t.Helper()
	root := &ast.Root{Children: []ast.Node{&ast.ContainerDirective{
		Name:     "sidebar",
		Attrs:    maps.Clone(attrs),
		Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{&ast.Text{Value: "body"}}}},
	}}}
	out := Render(root)
	assertRenderFixedPoint(t, out)
	return directiveAttrs(t, Parse([]byte(out))), out
}

func assertRenderFixedPoint(t *testing.T, out string) {
	t.Helper()
	if again := Render(Parse([]byte(out))); again != out {
		t.Errorf("render is not a fixed point: %q then %q", out, again)
	}
}

// ---------------------------------------------------------------------------
// Directive names
// ---------------------------------------------------------------------------

// FIX: a directive name the dialect cannot spell panics instead of being
// written, and a spellable name still renders (the second half of the
// same test, so neither a revert of the guard nor a guard that refuses
// everything can pass).
//
// The name is the one part of a directive that has no degradation. An
// unspellable ATTRIBUTE is dropped and the directive survives; a name
// cannot be dropped, and writing it anyway destroys the node: the form
// re-parses as an ordinary paragraph, or — when the name carries a line
// ending — as a directive under the truncated prefix with its attribute
// block stranded on the next line. Sanitizing it silently renames the
// author's directive. No parse can produce such a name (the parser
// refuses the whole directive instead), so every occurrence comes from a
// caller that built the node by hand, and a panic naming the offender is
// strictly better than a corrupt document.
//
// The hazard is shared by all three directive forms, because each writes
// the name verbatim into its marker.
func TestRender_UnspellableDirectiveNamePanics(t *testing.T) {
	names := []struct {
		why  string
		name string
	}{
		{why: "a space", name: "a b"},
		{why: "a closing brace", name: "a}b"},
		{why: "an opening brace", name: "a{b"},
		{why: "an opening bracket", name: "a[b"},
		{why: "a closing bracket", name: "a]b"},
		{why: "a colon", name: "a:b"},
		{why: "the empty name", name: ""},
		{why: "an LF", name: "a\nb"},
		{why: "a CR", name: "a\rb"},
		{why: "a tab", name: "a\tb"},
		{why: "a double quote", name: `a"b`},
		{why: "a single quote", name: "a'b"},
		{why: "an equals sign", name: "a=b"},
		{why: "a dot", name: "a.b"},
		{why: "a hash", name: "a#b"},
		{why: "a slash", name: "a/b"},
		{why: "a leading hyphen", name: "-a"},
		{why: "a leading underscore", name: "_a"},
		{why: "a trailing hyphen", name: "a-"},
		{why: "a trailing underscore", name: "a_"},
		{why: "only a hyphen", name: "-"},
		{why: "a multi-byte rune", name: "ä"},
	}

	for _, n := range names {
		t.Run(n.why, func(t *testing.T) {
			for _, form := range directiveNameForms {
				assertDirectiveNamePanics(t, form, n.name)
			}
		})
	}

	// The good case, in the same test: a name the parser accepts is
	// rendered and read back unchanged, attributes included.
	t.Run("a spellable name still renders", func(t *testing.T) {
		// Names outside the dialect: a registered name re-parses as its
		// typed node, and what is under test here is the generic form.
		for _, name := range []string{"a", "9", "A9", "sidebar", "a-b", "a_b", "a1-b2_c3"} {
			for _, form := range directiveNameForms {
				out := form.render(name)
				if got := reparsedDirectiveName(t, out); got != name {
					t.Errorf("%s form: %q re-parsed as %s, want the directive name %q",
						form.name, out, reparseShape(out), name)
				}
				assertRenderFixedPoint(t, out)
			}
		}
	})
}

// FIX (differential): nameSpells agrees with the parser on every
// candidate name, so the guard neither writes a name that corrupts the
// node nor panics on a name the parser would have accepted.
//
// nameSpells duplicates goldmark-directive's name grammar
// (scanDirectiveName), which that package does not export, so this test
// is what keeps the copy honest: it asks the PARSER what a name can be,
// by rendering nothing and instead reading a leaf marker back, and
// compares that verdict to the predicate. An upstream grammar change
// fails here rather than drifting into a false panic.
func TestRender_DirectiveNameSpellsMatchesTheParser(t *testing.T) {
	for _, name := range directiveNameCandidates() {
		if got, want := nameSpells(name), parserAcceptsDirectiveName(t, name); got != want {
			t.Errorf("nameSpells(%q) = %v, but the parser %s the name",
				name, got, map[bool]string{true: "accepts", false: "rejects"}[want])
		}
	}
}

// directiveNameCandidates enumerates the names the differential test
// checks: every ASCII byte in each of the three positions that carry a
// rule (opening, interior, closing), the one-byte names, and a few
// multi-byte runes.
func directiveNameCandidates() []string {
	var out []string
	for b := range 128 {
		c := string(rune(b))
		out = append(out, c, "a"+c+"b", "a"+c, c+"a", "ab"+c+"cd")
	}
	return append(out, "ä", "a€b", "aäb", "日本", "a b", "")
}

// parserAcceptsDirectiveName reports whether a leaf marker written with
// this name reads back as that same directive, attributes intact — the
// definition of a name the renderer may write. The marker is assembled
// as text here on purpose: the parser, not the renderer, is the
// authority being measured.
func parserAcceptsDirectiveName(t *testing.T, name string) bool {
	t.Helper()
	src := "::" + name + "{k=\"1\"}\n"
	kids := ast.Children(Parse([]byte(src)))
	if len(kids) == 0 {
		return false
	}
	leaf, ok := kids[0].(*ast.LeafDirective)
	return ok && leaf.Name == name && maps.Equal(leaf.Attrs, map[string]string{"k": "1"})
}

// directiveNameForm renders one of the three directive forms carrying a
// given name (with an attribute, so a truncated name shows up as a lost
// attribute block too).
type directiveNameForm struct {
	render func(name string) string
	name   string
}

var directiveNameForms = []directiveNameForm{
	{name: "text", render: renderTextDirectiveName},
	{name: "leaf", render: renderLeafDirectiveName},
	{name: "container", render: renderContainerDirectiveName},
}

func renderTextDirectiveName(name string) string {
	return Render(&ast.Root{Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{
		&ast.TextDirective{Name: name, Attrs: map[string]string{"k": "1"}},
	}}}})
}

func renderLeafDirectiveName(name string) string {
	return Render(&ast.Root{Children: []ast.Node{
		&ast.LeafDirective{Name: name, Attrs: map[string]string{"k": "1"}},
	}})
}

func renderContainerDirectiveName(name string) string {
	return Render(&ast.Root{Children: []ast.Node{&ast.ContainerDirective{
		Name:     name,
		Attrs:    map[string]string{"k": "1"},
		Children: []ast.Node{&ast.Paragraph{Children: []ast.Node{&ast.Text{Value: "body"}}}},
	}}})
}

// assertDirectiveNamePanics requires the render of an unspellable name to
// panic, with a message that names the offender and says the node was
// built rather than parsed (whoever reads the stack needs to know where
// to look).
//
// When the render does NOT panic, the failure reports what it wrote and
// what that re-parses to: the guard's mutation does not produce a diff,
// it produces a silently corrupted document, and that is what the
// failure has to show.
func assertDirectiveNamePanics(t *testing.T, form directiveNameForm, name string) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("%s form: panicked with %T (%v), want a diagnostic string", form.name, r, r)
		}
		if !strings.Contains(msg, strconv.Quote(name)) {
			t.Errorf("%s form: the panic does not name the offending directive name %q: %s", form.name, name, msg)
		}
		if !strings.Contains(msg, "BUILT BY A CALLER") {
			t.Errorf("%s form: the panic does not say the node was built rather than parsed: %s", form.name, msg)
		}
	}()
	out := form.render(name)
	t.Errorf("%s form: rendering the name %q did not panic — it wrote %q, which re-parses as %s (the corruption the guard exists to stop)",
		form.name, name, out, reparseShape(out))
}

// reparsedDirectiveName returns the name of the directive out re-parses
// to, in whichever position it sits, or "" when it is not a directive at
// all.
func reparsedDirectiveName(t *testing.T, out string) string {
	t.Helper()
	kids := ast.Children(Parse([]byte(out)))
	if len(kids) == 0 {
		return ""
	}
	switch n := kids[0].(type) {
	case *ast.LeafDirective:
		return n.Name
	case *ast.ContainerDirective:
		return n.Name
	case *ast.Paragraph:
		inline := ast.Children(n)
		if len(inline) == 0 {
			return ""
		}
		if d, ok := inline[0].(*ast.TextDirective); ok {
			return d.Name
		}
	}
	return ""
}

// reparseShape describes what out re-parses to, for a failure message:
// the node kind, plus the name and attributes when it is still a
// directive. A lost directive shows up as a bare paragraph here.
func reparseShape(out string) string {
	kids := ast.Children(Parse([]byte(out)))
	if len(kids) == 0 {
		return "an empty document"
	}
	node := kids[0]
	if para, ok := node.(*ast.Paragraph); ok {
		if inline := ast.Children(para); len(inline) > 0 {
			node = inline[0]
		}
	}
	switch n := node.(type) {
	case *ast.LeafDirective:
		return fmt.Sprintf("%T with name %q and attrs %v", n, n.Name, n.Attrs)
	case *ast.ContainerDirective:
		return fmt.Sprintf("%T with name %q and attrs %v", n, n.Name, n.Attrs)
	case *ast.TextDirective:
		return fmt.Sprintf("%T with name %q and attrs %v", n, n.Name, n.Attrs)
	default:
		return fmt.Sprintf("%T", n)
	}
}
