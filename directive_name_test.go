package adfast_test

import (
	"encoding/json"
	"testing"

	"github.com/pmarschik/adfast"
	"github.com/pmarschik/adfast/adf"
)

// The markdown renderer panics on a directive name it cannot spell (see
// markdown.mustSpellDirectiveName): the name is the one part of a
// directive that has no degradation, so writing an unspellable one
// destroys the node. That guard is only safe because no DOCUMENT can
// reach it — a panic on real input would be worse than the corruption it
// replaces.
//
// This test is what makes that a measured claim rather than a comment.
// Every value the ADF decode leg can put in a name position comes from a
// closed set: an unknown panelType degrades to "info", the block-mark
// wrappers and the dialect kinds write literal names, and an unknown
// extension key becomes an ATTRIBUTE of a generically named directive
// rather than a name. So each document below carries an unspellable
// string in a place that feeds a name, and the render must survive it.
//
// A conversion that panics here means a decode hook started deriving a
// directive name from document content, and the guard has to move or the
// hook has to whitelist.
func TestToMarkdown_NoDocumentReachesTheDirectiveNameGuard(t *testing.T) {
	docs := []string{
		// panelType is the one dialect name taken from an attribute.
		`{"type":"doc","version":1,"content":[{"type":"panel","attrs":{"panelType":"a b"},"content":[{"type":"paragraph","content":[{"type":"text","text":"x"}]}]}]}`,
		`{"type":"doc","version":1,"content":[{"type":"panel","attrs":{"panelType":""},"content":[{"type":"paragraph","content":[{"type":"text","text":"x"}]}]}]}`,
		`{"type":"doc","version":1,"content":[{"type":"panel","attrs":{"panelType":"a\nb"},"content":[{"type":"paragraph","content":[{"type":"text","text":"x"}]}]}]}`,
		// The block marks whose value names the wrapping container.
		`{"type":"doc","version":1,"content":[{"type":"paragraph","marks":[{"type":"alignment","attrs":{"align":"a b"}}],"content":[{"type":"text","text":"x"}]}]}`,
		`{"type":"doc","version":1,"content":[{"type":"paragraph","marks":[{"type":"alignment","attrs":{"align":""}}],"content":[{"type":"text","text":"x"}]}]}`,
		`{"type":"doc","version":1,"content":[{"type":"paragraph","marks":[{"type":"indentation","attrs":{"level":"a b"}}],"content":[{"type":"text","text":"x"}]}]}`,
		`{"type":"doc","version":1,"content":[{"type":"paragraph","marks":[{"type":"breakout","attrs":{"mode":"a b"}}],"content":[{"type":"text","text":"x"}]}]}`,
		// Extension keys and types: attributes of ::extension, not names.
		`{"type":"doc","version":1,"content":[{"type":"extension","attrs":{"extensionType":"a b","extensionKey":"c d","parameters":{}}}]}`,
		`{"type":"doc","version":1,"content":[{"type":"bodiedExtension","attrs":{"extensionType":"a b","extensionKey":"c d","parameters":{}},"content":[{"type":"paragraph","content":[{"type":"text","text":"x"}]}]}]}`,
		`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"inlineExtension","attrs":{"extensionType":"a b","extensionKey":"c d","parameters":{}}}]}]}`,
		// An unknown node kind and an unknown mark reach the markdown
		// projection without lending their type to a directive name.
		`{"type":"doc","version":1,"content":[{"type":"a b","content":[{"type":"paragraph","content":[{"type":"text","text":"x"}]}]}]}`,
		`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"a b"}]}]}]}`,
		// Attribute-carried values of the directive-backed kinds.
		`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"status","attrs":{"text":"s","color":"a b"}}]}]}`,
		`{"type":"doc","version":1,"content":[{"type":"expand","attrs":{"title":"a b"},"content":[{"type":"paragraph","content":[{"type":"text","text":"x"}]}]}]}`,
		`{"type":"doc","version":1,"content":[{"type":"embedCard","attrs":{"url":"a b","layout":"c d"}}]}`,
		`{"type":"doc","version":1,"content":[{"type":"mediaSingle","attrs":{"layout":"a b"},"content":[{"type":"media","attrs":{"type":"file","id":"1","collection":"c"}}]}]}`,
		`{"type":"doc","version":1,"content":[{"type":"layoutSection","content":[{"type":"layoutColumn","attrs":{"width":"a b"},"content":[{"type":"paragraph","content":[{"type":"text","text":"x"}]}]}]}]}`,
		`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"textColor","attrs":{"color":"a b"}}]}]}]}`,
		`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"fragment","attrs":{"localId":"a b","name":"c d"}}]}]}]}`,
	}

	for _, src := range docs {
		var raw map[string]any
		if err := json.Unmarshal([]byte(src), &raw); err != nil {
			t.Fatalf("bad test document %s: %v", src, err)
		}
		doc, ok := adf.DecodeDoc(raw)
		if !ok {
			t.Fatalf("adf.DecodeDoc declined %s", src)
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("a document reached the directive-name guard: %v\ndocument: %s", r, src)
				}
			}()
			if out := adfast.ToMarkdown(adfast.FromADF(doc)); out == "" {
				t.Errorf("rendered nothing for %s", src)
			}
		}()
	}
}
