package jira

import (
	"slices"
	"testing"

	"github.com/pmarschik/adfast"
)

// TestCodeLanguages_MatchesAtlaskitList proves the move of the language
// list into the root module did not alter Jira's set: same length, same
// order, same values.
func TestCodeLanguages_MatchesAtlaskitList(t *testing.T) {
	if !slices.Equal(CodeLanguages, adfast.AtlaskitCodeLanguages) {
		t.Fatalf("jira.CodeLanguages diverges from adfast.AtlaskitCodeLanguages:\ngot:  %v\nwant: %v",
			CodeLanguages, adfast.AtlaskitCodeLanguages)
	}
}
