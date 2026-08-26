package adfast

// AtlaskitCodeLanguages lists every code-block language identifier the
// @atlaskit editor accepts, for use with WithCodeLanguages.
//
// Source: the ADF codeBlock schema does not restrict attrs.language
// (unknown values render as plain, monospaced text —
// https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/codeBlock/);
// what constrains real-world values is the editor's language picker,
// @atlaskit/editor-plugin-code-block (src/pm-plugins/language-list.ts),
// which imports SUPPORTED_LANGUAGES from @atlaskit/code
// (design-system/code/src/constants.tsx in the Atlassian Frontend
// mirror, https://github.com/pioug/atlassian-frontend-mirror). The
// canonical value the editor stores in ADF is each entry's FIRST alias
// (per its ED-2813 comment) and incoming values match case-insensitively
// against ANY alias, so this set carries every alias, lowercased, plus
// the picker-only "none". Extracted from @atlaskit/code v18.2.1
// (published to npm 2026-07-17) on 2026-07-21.
//
// PROVENANCE: pinned to commit
// 7d83995738eb9c5e5da2c84107f2d66956d64ccc, the last commit to touch
// design-system/code/src/constants.tsx before v18.2.1's npm publish —
// confirmed unchanged through the mirror's HEAD as of 2026-08-26
// (ae723bb2fdcae8c006c91df9ced328738e94a8c4). That commit's alias set
// transcribes to exactly the 169 entries below, verified
// programmatically. This closes the gap flagged during planning (the
// list previously cited only a package version, unlike
// docs/adf-coverage.md's commit-pinned adf-schema oracle).
//
// The set is shared: BOTH Jira Cloud's editor and Confluence Cloud's ADF
// code-snippet element use this picker, so jira.CodeLanguages and
// confluence.CodeLanguages both derive from it. Measured against
// ixolit.atlassian.net page 1190100993 on 2026-08-25 (read-only): the ADF
// read returned codeBlock language "go" (2 nodes) and "json" (11 nodes),
// neither of which the legacy Confluence code block macro accepts.
//
// Grouped one language per line, canonical (first) alias leading.
var AtlaskitCodeLanguages = []string{
	"none",
	"abap",
	"actionscript", "actionscript3", "as",
	"ada", "ada95", "ada2005",
	"applescript",
	"arduino",
	"autoit",
	"c",
	"c++", "cpp", "clike",
	"clojure",
	"coffeescript", "coffee-script", "coffee",
	"coldfusion",
	"csharp", "c#",
	"css",
	"cuda", "cu",
	"d",
	"dart",
	"diff",
	"docker", "dockerfile",
	"elixir", "ex", "exs",
	"erlang", "erl",
	"fortran",
	"foxpro", "purebasic",
	"gherkin", "cucumber",
	"go",
	"graphql",
	"groovy",
	"handlebars", "mustache",
	"haskell", "hs",
	"haxe", "hx", "hxsl",
	"hcl", "terraform",
	"html",
	"java",
	"javafx", "jfx",
	"javascript", "js",
	"json",
	"jsx",
	"julia", "jl",
	"kotlin",
	"livescript", "live-script",
	"lua",
	"markdown",
	"mathematica", "mma", "nb",
	"matlab",
	"nginx",
	"objective-c", "objectivec", "obj-c", "objc",
	"objective-j", "objectivej", "obj-j", "objj",
	"ocaml",
	"octave",
	"pas", "pascal", "objectpascal", "delphi",
	"perl", "pl",
	"php", "php3", "php4", "php5",
	"powershell", "posh", "ps1", "psm1",
	"prolog",
	"protobuf", "proto",
	"puppet",
	"python", "py",
	"qbs", "qml",
	"r",
	"racket", "rkt",
	"restructuredtext", "rst", "rest",
	"ruby", "rb", "duby",
	"rust",
	"sass",
	"scala",
	"scheme", "scm",
	"shell", "bash", "sh", "ksh", "zsh",
	"smalltalk", "squeak", "st",
	"splunk-spl",
	"sql", "postgresql", "postgres", "plpgsql", "psql", "postgresql-console", "postgres-console", "tsql", "t-sql", "mysql", "sqlite",
	"standardml", "sml",
	"swift",
	"tcl",
	"tex", "latex",
	"text", "plaintext",
	"toml",
	"tsx",
	"typescript", "ts",
	"vala", "vapi",
	"vbnet", "vb.net", "vfp", "clipper", "xbase",
	"verilog", "v",
	"vhdl",
	"visualbasic",
	"xml",
	"xquery", "xqy", "xq", "xql", "xqm",
	"yaml", "yml",
}
