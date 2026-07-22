package jira

// CodeLanguages lists every code-block language identifier Jira Cloud's
// editor accepts, for use with adfast.WithCodeLanguages (see
// MarkdownOptions, which wires it automatically).
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
// the picker-only "none". Extracted from @atlaskit/code v18.2.1 on
// 2026-07-21.
//
// Grouped one language per line, canonical (first) alias leading.
var CodeLanguages = []string{
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
