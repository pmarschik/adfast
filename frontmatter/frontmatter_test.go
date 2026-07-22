package frontmatter

import (
	"reflect"
	"strings"
	"testing"

	adfast_ast "github.com/pmarschik/adfast/ast"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantKeys []string
	}{
		{name: "valid YAML", input: "status: Open\npriority: High", wantKeys: []string{"status", "priority"}},
		{name: "empty", input: "", wantKeys: nil},
		{name: "nested YAML", input: "parent:\n  key: INFRA-1", wantKeys: []string{"parent"}},
		{name: "fenced block", input: "---\nstatus: Open\n---", wantKeys: []string{"status"}},
		{name: "fenced block trailing newline", input: "---\nstatus: Open\n---\n", wantKeys: []string{"status"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, key := range tt.wantKeys {
				if _, ok := result[key]; !ok {
					t.Errorf("expected key %q in result", key)
				}
			}
		})
	}
}

func TestParseError(t *testing.T) {
	// Strict Parse surfaces malformed YAML as an error.
	if _, err := Parse("status: Open\n\tbadtab: x"); err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}
}

func TestParseLenient(t *testing.T) {
	// The permissive scan recovers keys even when strict YAML fails.
	got := ParseLenient("status: Open\n\tbadtab: x")
	if got["status"] != "Open" {
		t.Errorf("expected status recovered via lenient fallback, got %v", got)
	}
	if got["badtab"] != "x" {
		t.Errorf("expected badtab recovered via lenient fallback, got %v", got)
	}
}

func TestParseNormalizesNestedMaps(t *testing.T) {
	m, err := Parse("parent:\n  child: INFRA-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m["parent"].(map[string]any); !ok {
		t.Errorf("expected nested map[string]any, got %T", m["parent"])
	}
}

func TestRender(t *testing.T) {
	tests := []struct {
		input map[string]any
		name  string
		want  string
		order []string
	}{
		{name: "empty map", input: map[string]any{}, want: ""},
		{name: "single field", input: map[string]any{"status": "Open"}, want: "---\nstatus: Open\n---"},
		{
			name: "nested uses 2-space indent",
			input: map[string]any{
				"release_notes": map[string]any{"publish": true, "category": "bugfix"},
			},
			want: "---\nrelease_notes:\n  category: bugfix\n  publish: true\n---",
		},
		{
			name:  "order list wins over alphabetical",
			input: map[string]any{"status": "Open", "title": "T", "key": "K-1"},
			order: []string{"title", "status", "key"},
			want:  "---\ntitle: T\nstatus: Open\nkey: K-1\n---",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Render(tt.input, tt.order)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPatchUpdateSingleField(t *testing.T) {
	block := "---\nstatus: Open\npriority: High\n---"
	result, err := Patch(block, map[string]any{"status": "In Progress"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "status: In Progress") {
		t.Errorf("expected updated status, got:\n%s", result)
	}
	if !strings.Contains(result, "priority: High") {
		t.Errorf("expected preserved priority, got:\n%s", result)
	}
}

func TestPatchRemoveField(t *testing.T) {
	block := "---\nstatus: Open\npriority: High\n---"
	result, err := Patch(block, map[string]any{"priority": nil}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "priority") {
		t.Errorf("expected priority removed, got:\n%s", result)
	}
	if !strings.Contains(result, "status: Open") {
		t.Errorf("expected status preserved, got:\n%s", result)
	}
}

func TestPatchPreservesUnknownKeys(t *testing.T) {
	block := "---\nstatus: Open\ncustom_field: hello\n---"
	result, err := Patch(block, map[string]any{"status": "Done"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "custom_field: hello") {
		t.Errorf("expected custom_field preserved, got:\n%s", result)
	}
	if !strings.Contains(result, "status: Done") {
		t.Errorf("expected status updated, got:\n%s", result)
	}
}

func TestPatchCreatesBlockWhenNone(t *testing.T) {
	result, err := Patch("", map[string]any{"status": "Open"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "---\nstatus: Open\n---" {
		t.Errorf("unexpected block:\n%s", result)
	}
}

func TestReplace(t *testing.T) {
	old := "---\nstatus: Open\nkeep: me\n---"
	got := Replace(old, map[string]any{"status": "Done"}, nil)
	if got != "---\nstatus: Done\n---" {
		t.Errorf("Replace should discard old content, got:\n%s", got)
	}
}

func TestGetSetRemove(t *testing.T) {
	m := map[string]any{}
	Set(m, []string{"release_notes", "category"}, "bugfix")
	if got := Get(m, []string{"release_notes", "category"}); got != "bugfix" {
		t.Errorf("Get after Set: got %v, want bugfix", got)
	}
	if got := Get(m, []string{"missing", "x"}); got != nil {
		t.Errorf("Get missing path: got %v, want nil", got)
	}
	Remove(m, []string{"release_notes", "category"})
	if _, ok := m["release_notes"]; ok {
		t.Errorf("Remove should prune empty parent, got %v", m)
	}
}

func TestRemoveKeepsNonEmptyParent(t *testing.T) {
	m := map[string]any{"p": map[string]any{"a": 1, "b": 2}}
	Remove(m, []string{"p", "a"})
	parent, ok := m["p"].(map[string]any)
	if !ok {
		t.Fatalf("parent removed unexpectedly: %v", m)
	}
	if _, ok := parent["b"]; !ok {
		t.Errorf("sibling key dropped: %v", parent)
	}
}

func TestKeyOrder(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "inner yaml", input: "b: 1\na: 2\nc: 3", want: []string{"b", "a", "c"}},
		{name: "fenced block", input: "---\nx: 1\ny: 2\n---", want: []string{"x", "y"}},
		{name: "empty", input: "", want: nil},
		{name: "scalar not mapping", input: "hello", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KeyOrder(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseNode(t *testing.T) {
	n := &adfast_ast.Frontmatter{Value: "---\nstatus: Open\n---"}
	m, err := ParseNode(n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["status"] != "Open" {
		t.Errorf("ParseNode: got %v", m)
	}
	got, err := ParseNode(nil)
	if err != nil {
		t.Fatalf("ParseNode(nil): unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ParseNode(nil): want empty map, got %v", got)
	}
}
