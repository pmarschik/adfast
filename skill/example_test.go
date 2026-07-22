package skill_test

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/pmarschik/adfast/skill"
)

// Files exposes the skill content with SKILL.md at the root; the
// frontmatter carries the canonical skill name.
func ExampleFiles() {
	raw, err := fs.ReadFile(skill.Files(), "SKILL.md")
	if err != nil {
		panic(err)
	}
	name, _, _ := strings.Cut(strings.TrimPrefix(string(raw), "---\n"), "\n")
	fmt.Println(name)
	// Output:
	// name: adfast-markdown
}
