package frontmatter_test

import (
	"fmt"

	"github.com/pmarschik/adfast/frontmatter"
)

// PatchPreserving edits only the named key, leaving the author's key
// order, comments, and quoting untouched.
func ExamplePatchPreserving() {
	block := "---\n# document metadata\ntitle: 'Rooftop apiary'\nstatus: Open\n---"

	out, err := frontmatter.PatchPreserving(block, map[string]any{"status": "Done"})
	if err != nil {
		panic(err)
	}
	fmt.Println(out)
	// Output:
	// ---
	// # document metadata
	// title: 'Rooftop apiary'
	// status: Done
	// ---
}

// ParseSequenceSourceText keeps the spelling of unquoted numbers in a
// list, so version-like items survive as names. A scalar directly under
// a key stays typed.
func ExampleParseSequenceSourceText() {
	block := "---\nfix_versions: [1.0, 2, \"3.0\", 1.50]\npoints: 5.0\n---"

	m, err := frontmatter.ParseSequenceSourceText(block)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%q %v\n", m["fix_versions"], m["points"])
	// Output:
	// ["1.0" "2" "3.0" "1.50"] 5
}
