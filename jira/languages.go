package jira

import (
	"slices"

	"github.com/pmarschik/adfast"
)

// CodeLanguages lists every code-block language identifier Jira Cloud's
// editor accepts, for use with adfast.WithCodeLanguages (see
// MarkdownOptions, which wires it automatically). It is exactly
// adfast.AtlaskitCodeLanguages; see there for the source and the
// extraction date. Cloned so a caller that mutates this slice cannot
// reach the shared one.
var CodeLanguages = slices.Clone(adfast.AtlaskitCodeLanguages)
