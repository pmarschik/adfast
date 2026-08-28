package markdown

import (
	"maps"
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
