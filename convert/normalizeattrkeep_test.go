// This file is an external test package for normalizetotality_test.go's
// reason: the rules below separate Normalize from NormalizeFormat, and
// the facade exposes only the format leg, so a test driven through it
// could not tell them apart.
package convert_test

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/convert"
	"github.com/pmarschik/adfast/dialect"
)

// inlineAttrs spells the middle inline node of a normalized inlineTree
// as "kind{name=value …}", attribute names sorted. An inline node that
// normalized away spells "gone".
func inlineAttrs(t *testing.T, n ast.Node) string {
	t.Helper()
	root, ok := n.(*ast.Root)
	if !ok {
		t.Fatalf("normalize returned %T, want *ast.Root", n)
	}
	para, ok := root.Children[0].(*ast.Paragraph)
	if !ok {
		t.Fatalf("normalize returned a %T block, want *ast.Paragraph", root.Children[0])
	}
	for _, kid := range para.Children {
		attrs, ok := inlineAttrMap(kid)
		if !ok {
			continue
		}
		var parts []string
		for _, name := range slices.Sorted(maps.Keys(attrs)) {
			parts = append(parts, name+"="+attrs[name])
		}
		return fmt.Sprintf("%s{%s}", kid.Kind(), strings.Join(parts, " "))
	}
	return "gone"
}

// inlineAttrMap reads the attribute map of the dialect kinds this file
// drives, and answers false for anything else (the text leaves around
// them).
func inlineAttrMap(n ast.Node) (map[string]string, bool) {
	switch v := n.(type) {
	case *dialect.MediaInline:
		return v.Attrs, true
	case *dialect.Underline:
		return v.Attrs, true
	case *dialect.Emoji:
		return v.Attrs, true
	}
	return nil, false
}

// TestOnlyTheFormatLegKeepsAnAttributeTheKindCannotRead: the two legs
// answer differently on purpose, the same split
// TestOnlyTheFormatLegKeepsADirectiveTheKindCannotRead draws one case
// earlier. An attribute a kind cannot read has no ADF node to land in,
// so the encode leg goes on dropping it — that is what keeps
// ToADF(NormalizeFormat(n)) equal to ToADF(n) even though the format leg
// writes it back. Only the format leg is total; Normalize is not, and
// must not become so here.
func TestOnlyTheFormatLegKeepsAnAttributeTheKindCannotRead(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		inline     func() ast.Node
		name       string
		wantFormat string
		wantEncode string
	}{
		{
			// The atom half: the re-derived node carries the leftovers.
			name: "an atom carries what its payload does not read",
			inline: func() ast.Node {
				return &dialect.MediaInline{Attrs: map[string]string{"x": "1"}}
			},
			wantFormat: "mediaInline{x=1}",
			wantEncode: "mediaInline{}",
		},
		{
			// The mark half: nothing to carry it, so the node is kept.
			name: "a mark keeps the node that holds the attribute",
			inline: func() ast.Node {
				return &dialect.Underline{
					Attrs:    map[string]string{"a": "b"},
					Children: []ast.Node{&ast.Text{Value: "x"}},
				}
			},
			wantFormat: "underline{a=b}",
			wantEncode: "underline{}",
		},
		{
			// The emoji projection defers only on the format leg; the
			// encode leg has no text to lose and projects as before.
			name: "an emoji defers its projection only where the text matters",
			inline: func() ast.Node {
				return &dialect.Emoji{Attrs: map[string]string{"shortName": ":smile:", "zz": "1"}}
			},
			wantFormat: "emoji{shortName=:smile: zz=1}",
			wantEncode: "gone",
		},
		{
			// The bare-name half: only the leg that writes markdown
			// back has a reason to re-spell the default type.
			name: "only the format leg re-spells the default media type",
			inline: func() ast.Node {
				return &dialect.MediaInline{Attrs: map[string]string{"type": "file"}}
			},
			wantFormat: "mediaInline{type=file}",
			wantEncode: "mediaInline{}",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := inlineAttrs(t, convert.NormalizeFormat(inlineTree(tc.inline()))); got != tc.wantFormat {
				t.Errorf("NormalizeFormat\n got  %q\n want %q", got, tc.wantFormat)
			}
			if got := inlineAttrs(t, convert.Normalize(inlineTree(tc.inline()))); got != tc.wantEncode {
				t.Errorf("Normalize\n got  %q\n want %q", got, tc.wantEncode)
			}
		})
	}
}
