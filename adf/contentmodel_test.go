package adf

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestListItemAllows(t *testing.T) {
	tests := []struct {
		kind string
		want bool
	}{
		{"paragraph", true},
		{"bulletList", true},
		{"orderedList", true},
		{"mediaSingle", true},
		{"codeBlock", true},
		{"taskList", true},
		{"unsupportedBlock", true},
		{"extension", true},

		{"blockquote", false},
		{"table", false},
		{"heading", false},
		{"rule", false},
		{"panel", false},
		{"expand", false},
		{"nestedExpand", false},
		{"mediaGroup", false},
		{"decisionList", false},
		{"layoutSection", false},
		{"bodiedExtension", false},
		{"blockTaskItem", false},
		{"", false},
		{"someFutureKind", false},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			if got := ListItemAllows(tt.kind); got != tt.want {
				t.Errorf("ListItemAllows(%q) = %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}

func TestListItemViolationsReachesEveryNestingLevel(t *testing.T) {
	innerItem := &ListItem{Content: []Node{
		&Paragraph{},
		&Blockquote{},
	}}
	doc := wireDoc(&BulletList{Content: []Node{
		&ListItem{Content: []Node{
			&Paragraph{},
			&BulletList{Content: []Node{
				&ListItem{Content: []Node{
					&Paragraph{},
					&OrderedList{Content: []Node{innerItem}},
				}},
			}},
		}},
	}})

	violations := ListItemViolations(doc)
	if len(violations) != 1 {
		t.Fatalf("violations = %+v, want exactly one", violations)
	}
	if violations[0].Kind != "blockquote" {
		t.Errorf("Kind = %q, want blockquote", violations[0].Kind)
	}
	if violations[0].Item != innerItem {
		t.Errorf("Item = %p, want the innermost listItem %p", violations[0].Item, innerItem)
	}
}

func TestListItemViolationsInsideATableCell(t *testing.T) {
	item := &ListItem{Content: []Node{&Paragraph{}, &Blockquote{}}}
	doc := wireDoc(&Table{Content: []Node{
		&TableRow{Content: []Node{
			&TableCell{Content: []Node{
				&BulletList{Content: []Node{item}},
			}},
		}},
	}})

	violations := ListItemViolations(doc)
	if len(violations) != 1 || violations[0].Kind != "blockquote" || violations[0].Item != item {
		t.Fatalf("violations = %+v, want one blockquote violation on %p", violations, item)
	}
}

func TestListItemViolationsCleanDocumentIsNil(t *testing.T) {
	doc := wireDoc(&BulletList{Content: []Node{
		&ListItem{Content: []Node{
			&Paragraph{},
			&CodeBlock{},
			&MediaSingle{},
			&BulletList{Content: []Node{&ListItem{Content: []Node{&Paragraph{}}}}},
			&TaskList{},
		}},
	}})

	if violations := ListItemViolations(doc); violations != nil {
		t.Errorf("violations = %+v, want nil", violations)
	}
}

func TestListItemViolationsLeavesTheInputAlone(t *testing.T) {
	doc := wireDoc(&BulletList{Content: []Node{
		&ListItem{Content: []Node{&Paragraph{}, &Blockquote{}}},
	}})

	before, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal before: %v", err)
	}
	_ = ListItemViolations(doc)
	after, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal after: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("ListItemViolations mutated the document:\nbefore: %s\nafter:  %s", before, after)
	}
}
