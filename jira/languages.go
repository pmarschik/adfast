package jira

import (
	"maps"
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

// CodeLanguageAliases maps each identifier in CodeLanguages to the
// spelling Jira Cloud's editor stores in ADF for it, for use with
// adfast.WithCanonicalCodeLanguages: with it, a ```bash fence pushes
// language "shell". It is exactly adfast.AtlaskitCodeLanguageAliases —
// Jira's picker IS the atlaskit one — cloned so a caller that mutates
// this map cannot reach the shared one.
//
// MarkdownOptions does NOT wire it: canonicalizing changes the pushed
// payload, so it stays an explicit choice a caller adds to the bundle.
// See adfast.WithCanonicalCodeLanguages for what that choice costs on a
// pull.
var CodeLanguageAliases = maps.Clone(adfast.AtlaskitCodeLanguageAliases)
