package confluence

import (
	"slices"

	"github.com/pmarschik/adfast"
)

// CodeLanguages lists every code-block language identifier Confluence
// Cloud's ADF code block accepts, for use with adfast.WithCodeLanguages
// (see MarkdownOptions, which wires it automatically).
//
// Confluence Cloud's editor renders ADF codeBlock nodes through the code
// snippet element, whose language picker is the same @atlaskit list
// Jira's editor uses (adfast.AtlaskitCodeLanguages). MEASURED read-only
// against ixolit.atlassian.net page 1190100993 on 2026-08-25: the ADF
// read returned codeBlock language "go" (2 nodes) and "json" (11
// nodes), neither of which the legacy code block macro's set contains.
// This set therefore derives from the atlaskit list, not from the macro
// documentation.
//
// legacyMacroOnlyLanguages carries the two spellings the LEGACY code
// block macro accepts and the atlaskit picker does not, so that a
// document authored against the macro keeps encoding without a
// diagnostic. The remaining 31 macro values are all present in the
// atlaskit list already. Macro source: the Confluence Cloud code block
// macro documentation
// (https://support.atlassian.com/confluence-cloud/docs/insert-the-code-block-macro/),
// retrieved 2026-07-21, whose set is ActionScript, AppleScript, Bash,
// C#, C++, CSS, ColdFusion, Delphi, Diff, Erlang, Groovy, HTML and XML,
// Java, Java FX, JavaScript, Plain Text, PowerShell, Python, Ruby, SQL,
// Sass, Scala, and Visual Basic — carried here as its storage-format
// parameter value plus the lowercased display name where the two
// differ, because adfast.WithCodeLanguages matches fence info strings
// case-insensitively without alias normalization.
var legacyMacroOnlyLanguages = []string{
	"html/xml", // the macro's combined value; atlaskit has html and xml separately
	"vb",       // the macro's Visual Basic value; atlaskit has vbnet/vb.net/visualbasic
}

// CodeLanguages is the atlaskit list plus legacyMacroOnlyLanguages: a
// superset, chosen so a document authored against the legacy macro
// keeps encoding without a diagnostic while the ADF path's much larger
// set also passes. See legacyMacroOnlyLanguages for why the macro set
// is not exported: nothing in this repo renders storage format, so a
// second exported slice would be dead public API.
var CodeLanguages = append(
	slices.Clone(adfast.AtlaskitCodeLanguages),
	legacyMacroOnlyLanguages...,
)
