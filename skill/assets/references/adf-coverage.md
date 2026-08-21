# ADF coverage — node/mark → markdown mapping

Every node and mark type in Atlassian's ADF schema, whether it can occur
in that product's documents, and how adfast treats it. Support levels:

- **converted** — has a markdown mapping and round-trips through it;
- **preserved** — survives ADF decode → encode losslessly (typed or as
  `RawNode`/`RawMark`) but is dropped or reduced by the markdown
  projection, with a `raw-node` diagnostic.
- **dropped** — retired: adfast never produces the kind, and a legacy
  instance decodes to plain text with a `fontsize-dropped` diagnostic
  (text kept, styling lost). `fontSize` is the only such kind.

A `∘` means the kind is present in the shared ADF schema but Atlassian
documents no product-specific availability.

## Nodes

| ADF node                                      | Jira | Confluence | adfast support | Markdown mapping / notes                                                                                                                                                                   |
| --------------------------------------------- | ---- | ---------- | -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| doc                                           | ✓    | ✓          | converted      | document root                                                                                                                                                                              |
| paragraph                                     | ✓    | ✓          | converted      | paragraph                                                                                                                                                                                  |
| text                                          | ✓    | ✓          | converted      | plain text carrying the marks below                                                                                                                                                        |
| heading                                       | ✓    | ✓          | converted      | `#`–`######`; a trailing `{#id}` is the explicit anchor id                                                                                                                                 |
| blockquote                                    | ✓    | ✓          | converted      | `>`                                                                                                                                                                                        |
| rule                                          | ✓    | ✓          | converted      | `---`                                                                                                                                                                                      |
| codeBlock                                     | ✓    | ✓          | converted      | fenced code block; fence grows past embedded backtick runs; language survives                                                                                                              |
| bulletList / orderedList / listItem           | ✓    | ✓          | converted      | `-` / `1.` lists; marker alternation between adjacent lists; `order` start preserved                                                                                                       |
| taskList / taskItem                           | ✓    | ✓          | converted      | `- [ ]` / `- [x]`; `localId` regenerates as empty on encode                                                                                                                                |
| blockTaskItem                                 | ∘    | ∘          | converted      | `- [ ]` + indented blocks; a single-paragraph item re-encodes as the inline taskItem                                                                                                       |
| decisionList / decisionItem                   | ∘    | ✓          | converted      | `::decisions` + following plain bullet list; encodes with state DECIDED; Jira renders decisions but has no editor UI for them                                                              |
| table / tableRow / tableHeader / tableCell    | ✓    | ✓          | converted      | GFM pipe table; colspan/rowspan via `>`/`^` markers; colwidth attrs via `::colwidths`                                                                                                      |
| panel                                         | ✓    | ✓          | converted      | `:::info` …; unknown panelType degrades to `info`                                                                                                                                          |
| expand / nestedExpand                         | ✓    | ✓          | converted      | `:::expand[Title]` …; encode always emits `expand` (Jira nests it as nestedExpand itself)                                                                                                  |
| mediaSingle / mediaGroup / media              | ✓    | ✓          | converted      | `![alt](path)` or `::media`; plain image only when fully expressible; groups fan out to `group="true"` items                                                                               |
| mediaInline                                   | ✓    | ✓          | converted      | `:media{…}` inline attachment chip, or an inline `![alt](path)` the asset store maps to a media id; an external inline image URL has no ADF form and degrades to a link                    |
| caption                                       | ∘    | ✓          | converted      | image title (`![alt](path "caption")`) when plain text on image-expressible media, else the `:::media` body                                                                                |
| inlineCard                                    | ✓    | ✓          | converted      | `[KEY](url)` link; encodes back to inlineCard when the label equals the resolver-derived key                                                                                               |
| blockCard                                     | ✓    | ✓          | converted      | `::linkCard[…]`; URL-less cards are dropped                                                                                                                                                |
| blockCard + datasource                        | ✓    | ∘          | converted      | `::jql[…]{…}` — only the documented jira/jql shape; richer shapes fall back to `::linkCard`                                                                                                |
| embedCard                                     | ✓    | ✓          | converted      | `::linkEmbed[…]{…}`                                                                                                                                                                        |
| mention                                       | ✓    | ✓          | converted      | `:mention[Name]{#id}`                                                                                                                                                                      |
| emoji                                         | ✓    | ✓          | converted      | with a `text` attr: that text (deliberately lossy — shortName/id degrade to plain text across markdown); without: unicode via the emoji-toolkit shortname table, else `:emoji{shortName…}` |
| status                                        | ✓    | ✓          | converted      | `:status[Text]{color}`                                                                                                                                                                     |
| date                                          | ✓    | ✓          | converted      | `:date[2026-07-15]{timestamp="…"}`; the timestamp attribute is authoritative                                                                                                               |
| hardBreak                                     | ✓    | ✓          | converted      | backslash / trailing-space break                                                                                                                                                           |
| placeholder                                   | —    | ✓          | converted      | `:placeholder[Type something…]`                                                                                                                                                            |
| layoutSection / layoutColumn                  | —    | ✓          | converted      | `:::section` containing `:::column{width="…"}` containers                                                                                                                                  |
| extension / bodiedExtension / inlineExtension | —    | ✓          | converted      | `::extension{…}` / `:::extension{…}` + body / `:extension{…}`; `confluence.Macros()` sugars the core macros (`::toc`, `:::excerpt`, `::includePage[SPACE/123]`, …)                         |
| multiBodiedExtension / extensionFrame         | ∘    | ✓          | converted      | `:::extension{…}` whose children are all `:::frame` containers; stage-0 schema                                                                                                             |
| syncBlock / bodiedSyncBlock                   | —    | ✓          | converted      | `::syncBlock{…}` (reference) / `:::syncBlock{…}` + body (source)                                                                                                                           |

## Marks

| ADF mark        | Jira | Confluence | adfast support | Markdown mapping / notes                                                                                                                                                                                              |
| --------------- | ---- | ---------- | -------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| strong          | ✓    | ✓          | converted      | `**bold**`                                                                                                                                                                                                            |
| em              | ✓    | ✓          | converted      | `_italic_`                                                                                                                                                                                                            |
| strike          | ✓    | ✓          | converted      | `~~strike~~`                                                                                                                                                                                                          |
| code            | ✓    | ✓          | converted      | `` `code` ``; exclusive like ADF (strong/em/strike stripped)                                                                                                                                                          |
| underline       | ✓    | ✓          | converted      | `:u[text]`                                                                                                                                                                                                            |
| link            | ✓    | ✓          | converted      | `[label](url)` incl. titles                                                                                                                                                                                           |
| subsup          | ✓    | ✓          | converted      | `:sub[text]` / `:sup[text]`                                                                                                                                                                                           |
| textColor       | ✓    | ✓          | converted      | `:color[text]{color="#ff5630"}`                                                                                                                                                                                       |
| backgroundColor | ✓    | ✓          | converted      | `:bg[text]{color="#fffae6"}`                                                                                                                                                                                          |
| border          | ✓    | ✓          | converted      | `borderColor`/`borderSize` attributes on the media directive forms                                                                                                                                                    |
| alignment       | —    | ✓          | converted      | `:::center` / `:::end` wrapper around the block                                                                                                                                                                       |
| indentation     | —    | ✓          | converted      | `:::indent{level}` wrapper around the block                                                                                                                                                                           |
| breakout        | —    | ✓          | converted      | `:::breakout{mode}` wrapper around the block                                                                                                                                                                          |
| annotation      | —    | ✓          | converted      | `:annotation[text]{#id annotationType}` — keeps Confluence inline-comment threads anchored across markdown edits; overlapping anchors on one text run degrade to the outermost                                        |
| dataConsumer    | —    | ✓          | converted      | `:::dataConsumer{sources="id1,id2"}` wrapper around the block (`sources` is a comma-separated id list)                                                                                                                |
| fragment        | —    | ✓          | converted      | `:::fragment{localId name?}` wrapper around the block                                                                                                                                                                 |
| fontSize        | —    | —          | dropped        | **Retired** — no product supports the mark. `:fontSize[text]{size}` parses but unwraps to plain text (encode) and a legacy mark decodes to bare text, each with a `fontsize-dropped` diagnostic. Text kept, size lost |

In short: unknown or undocumented ADF content survives ADF-level round
trips losslessly and can be reported through diagnostics; only the
markdown projection reduces it. Every kind in the tables has a markdown
mapping, so documents also survive markdown-only persistence (render →
store the file → re-parse → push).
