package adfast

// atlaskitCodeLanguageGroups is the @atlaskit editor's code-block
// language picker, ONE GROUP PER LANGUAGE, each group carrying that
// language's aliases in upstream order with the canonical identifier
// FIRST. It is the single source both AtlaskitCodeLanguages (the flat
// accept set) and AtlaskitCodeLanguageAliases (alias → canonical) derive
// from, so the two can never disagree about which spellings exist.
//
// Source: the ADF codeBlock schema does not restrict attrs.language
// (unknown values render as plain, monospaced text —
// https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/codeBlock/);
// what constrains real-world values is the editor's language picker,
// @atlaskit/editor-plugin-code-block (src/pm-plugins/language-list.ts),
// which imports SUPPORTED_LANGUAGES from @atlaskit/code
// (design-system/code/src/constants.tsx in the Atlassian Frontend
// mirror, https://github.com/pioug/atlassian-frontend-mirror). Extracted
// from @atlaskit/code v18.2.1 (published to npm 2026-07-17) on
// 2026-07-21.
//
// PROVENANCE: pinned to commit
// 7d83995738eb9c5e5da2c84107f2d66956d64ccc, the last commit to touch
// design-system/code/src/constants.tsx before v18.2.1's npm publish —
// confirmed unchanged through the mirror's HEAD as of 2026-08-26
// (ae723bb2fdcae8c006c91df9ced328738e94a8c4). That commit's 85 entries
// transcribe to exactly the 85 language groups below (the 86th, "none",
// is the picker-only entry language-list.ts prepends), verified
// programmatically against the upstream file's alias arrays — membership
// AND per-group order — on 2026-08-27. This closes the gap flagged during
// planning (the list previously cited only a package version, unlike
// docs/adf-coverage.md's commit-pinned adf-schema oracle).
//
// The GROUPING and the leading position are load-bearing, not cosmetic:
// language-list.ts matches an incoming value against ANY alias
// (findMatchedLanguage: `supportedLanguage.alias.indexOf(lang.toLowerCase())`)
// and writes back `alias[0]` (getLanguageIdentifier, per its ED-2813
// comment), so the first entry of a group is the spelling the editor
// stores in ADF and the rest are spellings it accepts. Aliases are
// lowercased here: upstream's one mixed-case entry ("standardmL") is
// unreachable by that lowercase match and its group already lists
// "standardml" as well.
//
// The set is shared: BOTH Jira Cloud's editor and Confluence Cloud's ADF
// code-snippet element use this picker, so jira.CodeLanguages and
// confluence.CodeLanguages both derive from it. Measured against
// ixolit.atlassian.net page 1190100993 on 2026-08-25 (read-only): the ADF
// read returned codeBlock language "go" (2 nodes) and "json" (11 nodes),
// neither of which the legacy Confluence code block macro accepts.
//
// Groups are listed in canonical-identifier order.
var atlaskitCodeLanguageGroups = [][]string{
	{"none"},
	{"abap"},
	{"actionscript", "actionscript3", "as"},
	{"ada", "ada95", "ada2005"},
	{"applescript"},
	{"arduino"},
	{"autoit"},
	{"c"},
	{"c++", "cpp", "clike"},
	{"clojure"},
	{"coffeescript", "coffee-script", "coffee"},
	{"coldfusion"},
	{"csharp", "c#"},
	{"css"},
	{"cuda", "cu"},
	{"d"},
	{"dart"},
	{"diff"},
	{"docker", "dockerfile"},
	{"elixir", "ex", "exs"},
	{"erlang", "erl"},
	{"fortran"},
	{"foxpro", "purebasic"},
	{"gherkin", "cucumber"},
	{"go"},
	{"graphql"},
	{"groovy"},
	{"handlebars", "mustache"},
	{"haskell", "hs"},
	{"haxe", "hx", "hxsl"},
	{"hcl", "terraform"},
	{"html"},
	{"java"},
	{"javafx", "jfx"},
	{"javascript", "js"},
	{"json"},
	{"jsx"},
	{"julia", "jl"},
	{"kotlin"},
	{"livescript", "live-script"},
	{"lua"},
	{"markdown"},
	{"mathematica", "mma", "nb"},
	{"matlab"},
	{"nginx"},
	{"objective-c", "objectivec", "obj-c", "objc"},
	{"objective-j", "objectivej", "obj-j", "objj"},
	{"ocaml"},
	{"octave"},
	{"pas", "pascal", "objectpascal", "delphi"},
	{"perl", "pl"},
	{"php", "php3", "php4", "php5"},
	{"powershell", "posh", "ps1", "psm1"},
	{"prolog"},
	{"protobuf", "proto"},
	{"puppet"},
	{"python", "py"},
	{"qbs", "qml"},
	{"r"},
	{"racket", "rkt"},
	{"restructuredtext", "rst", "rest"},
	{"ruby", "rb", "duby"},
	{"rust"},
	{"sass"},
	{"scala"},
	{"scheme", "scm"},
	{"shell", "bash", "sh", "ksh", "zsh"},
	{"smalltalk", "squeak", "st"},
	{"splunk-spl"},
	{"sql", "postgresql", "postgres", "plpgsql", "psql", "postgresql-console", "postgres-console", "tsql", "t-sql", "mysql", "sqlite"},
	{"standardml", "sml"},
	{"swift"},
	{"tcl"},
	{"tex", "latex"},
	{"text", "plaintext"},
	{"toml"},
	{"tsx"},
	{"typescript", "ts"},
	{"vala", "vapi"},
	{"vbnet", "vb.net", "vfp", "clipper", "xbase"},
	{"verilog", "v"},
	{"vhdl"},
	{"visualbasic"},
	{"xml"},
	{"xquery", "xqy", "xq", "xql", "xqm"},
	{"yaml", "yml"},
}

// AtlaskitCodeLanguages lists every code-block language identifier the
// @atlaskit editor accepts, for use with WithCodeLanguages: every alias
// of every language, lowercased, plus the picker-only "none". It is
// atlaskitCodeLanguageGroups flattened in group order — see there for
// the source, the pinned commit, and why the grouping exists.
var AtlaskitCodeLanguages = flattenCodeLanguages(atlaskitCodeLanguageGroups)

// AtlaskitCodeLanguageAliases maps every identifier in
// AtlaskitCodeLanguages to the CANONICAL spelling the @atlaskit editor
// stores in ADF for it — the first alias of its group
// (getLanguageIdentifier in language-list.ts, per its ED-2813 comment).
// Pass it to WithCanonicalCodeLanguages to have a fence's language tag
// normalized on the way into ADF, so ```bash encodes as "shell", the
// spelling the editor's own picker round-trips.
//
// Every key is lowercase, and a canonical identifier maps to itself, so
// a lookup doubles as the membership test AtlaskitCodeLanguages answers
// and applying the map is idempotent. A value not in the map is not an
// accepted identifier; leave it alone rather than guessing.
//
// The map is a package-level value like AtlaskitCodeLanguages: treat it
// as read-only, and clone it (maps.Clone) before handing it anywhere
// that might write.
var AtlaskitCodeLanguageAliases = canonicalCodeLanguages(atlaskitCodeLanguageGroups)

// flattenCodeLanguages concatenates the groups in order.
func flattenCodeLanguages(groups [][]string) []string {
	n := 0
	for _, g := range groups {
		n += len(g)
	}
	out := make([]string, 0, n)
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// canonicalCodeLanguages maps every alias of every group to the group's
// first alias, the identifier the editor writes back.
func canonicalCodeLanguages(groups [][]string) map[string]string {
	out := make(map[string]string, len(groups)*2)
	for _, g := range groups {
		if len(g) == 0 {
			continue
		}
		for _, alias := range g {
			out[alias] = g[0]
		}
	}
	return out
}
