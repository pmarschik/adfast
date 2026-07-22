package confluence

// CodeLanguages lists every code-block language identifier Confluence
// Cloud's code block macro accepts, for use with
// adfast.WithCodeLanguages (see MarkdownOptions, which wires it
// automatically).
//
// Source: the Confluence Cloud code block macro documentation
// (https://support.atlassian.com/confluence-cloud/docs/insert-the-code-block-macro/)
// enumerates the supported languages: ActionScript, AppleScript, Bash,
// C#, C++, CSS, ColdFusion, Delphi, Diff, Erlang, Groovy, HTML and
// XML, Java, Java FX, JavaScript, Plain Text, PowerShell, Python,
// Ruby, SQL, Sass, Scala, and Visual Basic. This is a much smaller set
// than Jira's editor list (jira.CodeLanguages) — notably no Go, JSON,
// Kotlin, Rust, TypeScript, or YAML — and also smaller than the
// Confluence DATA CENTER macro's 80-language list
// (https://confluence.atlassian.com/doc/code-block-macro-139390.html).
// The set carries each language's storage-format parameter value (the
// value of the macro's "language" parameter, as documented across the
// Data Center macro doc lineage) plus the lowercased display name where
// the two differ, since adfast.WithCodeLanguages matches fence info
// strings case-insensitively without alias normalization. Retrieved
// 2026-07-21.
//
// Note the distinction: Confluence Cloud's NEW editor renders ADF
// codeBlock nodes through the code snippet element, whose language
// picker is the same @atlaskit list Jira's editor uses; this set is
// the legacy code block macro's, which is what the Cloud macro
// documentation specifies.
//
// Grouped one language per line, storage parameter value leading.
var CodeLanguages = []string{
	"actionscript3", "actionscript",
	"applescript",
	"bash",
	"c#", "csharp",
	"coldfusion",
	"cpp", "c++",
	"css",
	"delphi",
	"diff",
	"erl", "erlang",
	"groovy",
	"html/xml", "html", "xml",
	"java",
	"jfx", "javafx",
	"js", "javascript",
	"powershell",
	"py", "python",
	"ruby",
	"sass",
	"scala",
	"sql",
	"text",
	"vb", "visualbasic",
}
