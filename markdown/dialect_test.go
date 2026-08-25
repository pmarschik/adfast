package markdown

import (
	"strings"
	"testing"

	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/convert"
	"github.com/pmarschik/adfast/dialect"
)

// TestParse_DialectPromotion pins the single-name-table design: the
// goldmark parse produces only GENERIC directive nodes (no per-kind
// goldmark node registry), and dialect.Registrations() is the one place
// directive names bind to typed kinds — Parse promotes them on the
// lifted AST with their documented attributes bound.
func TestParse_DialectPromotion(t *testing.T) {
	src := ":::info\npanel body\n:::\n\n" +
		"::media[shot.png]{#b577 collection height=\"512\" layout=\"align-start\" type=\"file\" width=\"772\" group=\"true\"}\n\n" +
		"::jql[project = X]{cloudId=\"c1\" columns=\"summary,status\"}\n\n" +
		"a :status[Ready]{color=\"green\"} b :mention[@P]{#712020:abc} c\n"
	root := Parse([]byte(src))

	if panel := firstNode[*dialect.Panel](t, root); panel.PanelType != "info" {
		t.Errorf("panel not promoted: %+v", panel)
	}
	wantMediaAttrs(t, firstNode[*dialect.Media](t, root))
	if jql := firstNode[*dialect.JQL](t, root); jql.CloudID != "c1" || jql.Columns != "summary,status" {
		t.Errorf("jql attrs not bound: %+v", jql)
	}
	if status := firstNode[*dialect.Status](t, root); status.Color != "green" || ast.PlainText(status.Children) != "Ready" {
		t.Errorf("status not bound: %+v", status)
	}
	if mention := firstNode[*dialect.Mention](t, root); mention.AccountID != "712020:abc" {
		t.Errorf("mention id not bound: %+v", mention)
	}
}

// wantMediaAttrs pins every attribute the ::media directive binds — the
// widest attribute surface in the dialect, so it carries its own helper.
func wantMediaAttrs(t *testing.T, media *dialect.Media) {
	t.Helper()
	if media.ID != "b577" || media.MediaType != "file" || media.Width != 772 ||
		media.Height != 512 || !media.Group || media.Layout != "align-start" {
		t.Errorf("media attrs not bound: %+v", media)
	}
}

// firstNode returns the first node of kind T in the tree and fails the
// test when the parse promoted none. Each promotion assertion above
// wants exactly one node of its own kind, and the search is the same for
// all of them — one generic walk instead of a five-case type switch that
// has to grow with the dialect.
func firstNode[T ast.Node](t *testing.T, root ast.Node) T {
	t.Helper()
	found, ok := searchNode[T](root)
	if !ok {
		t.Fatalf("%T was not promoted", found)
	}
	return found
}

// searchNode is firstNode's depth-first search, without the test hook.
func searchNode[T ast.Node](n ast.Node) (T, bool) {
	if typed, ok := n.(T); ok {
		return typed, true
	}
	for _, c := range ast.Children(n) {
		if typed, ok := searchNode[T](c); ok {
			return typed, true
		}
	}
	var zero T
	return zero, false
}

// TestParse_GoldmarkTreeStaysGeneric asserts the goldmark layer carries
// no typed dialect nodes: known and unknown directive names alike parse
// as the generic goldmark-directive kinds (promotion happens after the
// lift, from dialect.Registrations()).
func TestParse_GoldmarkTreeStaysGeneric(t *testing.T) {
	src := ":::info\nx\n:::\n\n::media[y]\n\na :status[z] b\n" +
		":::custom\nx\n:::\n\n::unknownleaf[y]\n\na :unknowntext[z] b\n"
	tree := NewParser().Parse(text.NewReader([]byte(src)))
	found := map[string]int{}
	mustWalk(tree, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if entering {
			switch k := n.Kind().String(); k {
			case "ContainerDirective", "LeafDirective", "TextDirective":
				found[k]++
			}
		}
		return gast.WalkContinue, nil
	})
	if found["ContainerDirective"] != 2 || found["LeafDirective"] != 2 || found["TextDirective"] != 2 {
		t.Errorf("expected 2 generic nodes per position, got %v", found)
	}
}

// TestDialect_UnknownStaysGeneric: unknown directive names keep their
// generic AST kinds and degrade through the pipeline like remark.
func TestDialect_UnknownStaysGeneric(t *testing.T) {
	src := ":::custom\nx\n:::\n\n::unknownleaf[y]\n\na :unknowntext[z] b\n"
	root := Parse([]byte(src))
	found := 0
	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		switch n.(type) {
		case *ast.ContainerDirective, *ast.LeafDirective, *ast.TextDirective:
			found++
		}
		for _, c := range ast.Children(n) {
			walk(c)
		}
	}
	walk(root)
	if found != 3 {
		t.Errorf("expected 3 generic directive nodes, got %d", found)
	}
	// And the full pipeline degrades them like remark (round trip).
	out := Render(convert.FromADF(convert.ToADF(root)))
	if out == "" {
		t.Error("degradation pipeline broke")
	}
}

func mustWalk(root gast.Node, fn gast.Walker) {
	if err := gast.Walk(root, fn); err != nil {
		panic(err)
	}
}

// TestRender_UnregisteredDirectivesRoundTrip proves the generic
// directive RENDER path is live: a consumer using Parse→Render directly
// (no ADF trip — where generic directives degrade at encode instead)
// gets unregistered directives serialized back in their source forms,
// like remark-stringify.
func TestRender_UnregisteredDirectivesRoundTrip(t *testing.T) {
	src := ":::custom[label]\nbody text\n:::\n\n" +
		"::unknownleaf[y]{#id key=\"v\"}\n\n" +
		"a :unknowntext[z]{k=\"w\"} b\n"
	out := Render(Parse([]byte(src)))
	for _, want := range []string{":::custom", "body text", "::unknownleaf[y]{#id key=\"v\"}", ":unknowntext[z]{k=\"w\"}"} {
		if !strings.Contains(out, want) {
			t.Errorf("generic directive lost in Parse→Render: want %q in %q", want, out)
		}
	}
	// Stable: rendering the re-parse reproduces the output.
	if again := Render(Parse([]byte(out))); again != out {
		t.Errorf("generic directive render not stable:\n%q\n%q", out, again)
	}
}
