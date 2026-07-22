package adfast

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/pmarschik/adfast/adf"
)

func datasourceCardJSON(t *testing.T, withViews, withURL bool) *adf.BlockCard {
	t.Helper()
	raw := `{
		"type": "doc",
		"version": 1,
		"content": [{
			"type": "blockCard",
			"attrs": {
				"datasource": {
					"id": "d8b75300-dfda-4519-b6cd-e49abbd50401",
					"parameters": {
						"cloudId": "cloud-1",
						"jql": "project = INFRA AND status = Open"
					}
				}
			}
		}]
	}`
	var docNode adf.Doc
	if err := json.Unmarshal([]byte(raw), &docNode); err != nil {
		t.Fatal(err)
	}
	card, ok := docNode.Content[0].(*adf.BlockCard)
	if !ok {
		t.Fatal("blockCard shape")
	}
	if withViews {
		card.Datasource["views"] = []any{map[string]any{
			"type": "table",
			"properties": map[string]any{"columns": []any{
				map[string]any{"key": "summary"},
				map[string]any{"key": "status"},
			}},
		}}
	}
	if withURL {
		card.URL = "https://ixolit.atlassian.net/issues/?jql=x"
	}
	return card
}

func TestDatasourceBlockCard_RoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name      string
		want      string
		withViews bool
		withURL   bool
	}{
		{
			name: "minimal", want: "::jql[project = INFRA AND status = Open]{cloudId=\"cloud-1\" datasource=\"d8b75300-dfda-4519-b6cd-e49abbd50401\"}\n",
		},
		{
			name: "views and url", withViews: true, withURL: true,
			want: "::jql[project = INFRA AND status = Open]{cloudId=\"cloud-1\" columns=\"summary,status\" datasource=\"d8b75300-dfda-4519-b6cd-e49abbd50401\" url=\"https://ixolit.atlassian.net/issues/?jql=x\"}\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			card := datasourceCardJSON(t, tc.withViews, tc.withURL)
			doc := adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{card}}
			md := adfToMD(doc)
			if md != tc.want {
				t.Fatalf("render:\n got: %q\nwant: %q", md, tc.want)
			}
			back := mdToADF(md)
			if !reflect.DeepEqual(back.Content, doc.Content) {
				t.Errorf("round trip diverged:\n got: %#v\nwant: %#v", back.Content, doc.Content)
			}
		})
	}
}

func TestDatasourceBlockCard_RicherShapesFallBack(t *testing.T) {
	card := datasourceCardJSON(t, false, true)
	card.Datasource["views"] = []any{map[string]any{"type": "board"}}
	md := adfToMD(adf.Doc{Type: "doc", Version: 1, Content: []adf.Node{card}})
	if md != "::linkCard[https://ixolit.atlassian.net/issues/?jql=x]\n" {
		t.Errorf("expected linkCard fallback, got %q", md)
	}
}
