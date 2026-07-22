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
