# adfast dialect — complete syntax reference

The base dialect is **CommonMark + GFM**: pipe tables (padded to column
width, plus cell merging — see [Tables](#tables)), task lists (`- [ ]` /
`- [x]`), strikethrough (`~~text~~`), and autolink literals. On top of
that: YAML `---` frontmatter (split off before parsing, re-emitted
verbatim by the formatter), decision lists, heading anchors
(`## Title {#id}`), and the directive dialect below.

## Directive levels

- `:name[label]{attrs}` — **text directive**, inline within a paragraph.
- `::name[label]{attrs}` — **leaf directive**, a standalone line.
- `:::name[label]{attrs}` … `:::` — **container directive** around block
  content.

Nested containers grow the outer fence (`::::`, `:::::`, …), exactly
like remark-directive: the outermost container carries the longest
fence, and every closing fence matches its opener's length.

## Attribute syntax

Attributes live in `{…}` after the label:

- `key="value"` — quoted named attribute.
- `#some-id` — shorthand for the id attribute (used for media uuids,
  mention account ids, annotation ids, emoji ids).
- A bare word (e.g. `collection`, `group`) — an attribute with an empty
  value; some directives treat specific bare attributes as flags.
- **Bare-value form**: for single-valued directives, exactly one
  attribute with an empty value is the directive's value — `{2}` for
  `:::indent`, `{wide}` for `:::breakout` (and `{small}` for the retired
  `:fontSize`). A named `level=` / `mode=` / `size=` attribute wins when
  both are present; the canonical rendering is always the bare form.
- **Canonical JSON attr encoding**: the arbitrary-JSON `parameters`
  payload on extensions carries `json.Marshal` output (sorted keys, no
  insignificant whitespace). Since that JSON contains `"`, the attribute
  is single-quoted so it stays readable and lossless:
  `parameters='{"station":"rooftop"}'`. If the JSON value itself contains
  a `'`, single-quoting is not lossless, so the attribute falls back to
  double quotes with every `"` written as `&quot;` (remark decodes
  character references in attribute values, so this stays
  remark-compatible).
- **Comma-separated sources**: `sources` on `:::dataConsumer` is a plain
  comma-separated list of opaque source ids (not JSON) —
  `sources="id1,id2"`.

Rendered attributes are sorted; the id shorthand comes first.

## Container directives (block elements)

| Markdown                               | ADF                          | Notes                                                                                                                                                                  |
| -------------------------------------- | ---------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `:::info` … `:::`                      | panel                        | Also `note`, `warning`, `success`, `error`; unknown panel types degrade to `info`                                                                                      |
| `:::expand[Title]` … `:::`             | expand                       | Title is optional; nests inside panels as nestedExpand                                                                                                                 |
| `:::media[alt]{…}` … `:::`             | mediaSingle + caption        | The `::media` attrs on the fence line, the caption as the body; a plain-text caption on image-expressible media uses the image title instead: `![alt](path "caption")` |
| `:::extension{…}` … `:::`              | bodiedExtension              | Same attrs as `::extension`; when every child is a `:::frame` container (extensionFrame) it encodes as multiBodiedExtension (a frameless one carries the bare `multi`) |
| `:::frame` … `:::`                     | extensionFrame               | Only inside a `:::extension` container                                                                                                                                 |
| `:::syncBlock{resourceId localId}` …   | bodiedSyncBlock              | The source body of a synced block                                                                                                                                      |
| `:::section` + `:::column{width="…"}`  | layoutSection / layoutColumn | Page layouts; `columnRuleStyle`/`localId` on the section, `width`/`valign`/`localId` on each column; columns nest inside the section, so the section fence grows       |
| `:::center` / `:::end` … `:::`         | alignment mark               | Block mark on each wrapped paragraph/heading                                                                                                                           |
| `:::indent{2}` … `:::`                 | indentation mark             | The bare value is the level (1–6)                                                                                                                                      |
| `:::breakout{wide}` … `:::`            | breakout mark                | Modes `wide`/`full-width`; optional `width="1200"`                                                                                                                     |
| `:::dataConsumer{sources="id1,id2"}` … | dataConsumer mark            | `sources` is a comma-separated list of opaque source ids                                                                                                               |
| `:::fragment{localId="…" name?}` …     | fragment mark                | Stable references to tables/extensions                                                                                                                                 |

The mark-wrapper containers (`:::center`/`:::end`, `:::indent`,
`:::breakout`, `:::dataConsumer`, `:::fragment`) put the ADF **block
mark** on every block they wrap; wrappers compose by nesting, and the
ADF mark array maps inside-out onto the nesting (first mark innermost),
so round trips preserve mark order.

## Leaf directives (standalone lines)

| Markdown                                                                                                 | ADF                        | Notes                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| -------------------------------------------------------------------------------------------------------- | -------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `::linkCard[ABC-123]`                                                                                    | blockCard                  | Bare keys expand via the configured `SmartLinks` resolver; full URLs also work: `::linkCard[https://…]`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| `::linkEmbed[https://…]{layout="center" width="80"}`                                                     | embedCard                  | `layout`/`width` mirror the embed attributes                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| `::media[shot.png]{#<media-uuid> collection height="551" layout="align-start" type="file" width="2308"}` | mediaSingle / media        | Attachments; the label is the alt text. Every attribute is optional. `type` is `file`\|`external` (default `file`); `#<id>` is the media id; `url` carries the link for `type="external"` media; `collection`/`occurrenceKey` are opaque strings kept when present; `width`/`height` are the intrinsic pixel dimensions (numbers), `layoutWidth` the display width (number) and `widthType` its unit; `layout` is the ADF mediaSingle layout; `group="true"` items reassemble a mediaGroup; `path` points at the downloaded local file; `borderColor`/`borderSize` (number) carry the ADF border mark |
| `::colwidths[79,320,200]`                                                                                | table column widths        | Placed directly before a table; widths re-apply to every row on encode. A `::colwidths` with no following table is dropped with a `colwidths-orphan` diagnostic. Counts visual columns — a colspan cell carries one width per covered column                                                                                                                                                                                                                                                                                                                                                          |
| `::decisions`                                                                                            | decisionList marker        | Marks the immediately following plain bullet list as a decision list — see [Decision lists](#decision-lists)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| `::jql[project = X AND status = Open]{cloudId="…" datasource="…" columns="summary,status"}`              | blockCard (JQL datasource) | Live JQL tables (Jira); `columns` lists the table-view keys, `url` is kept when present                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| `::extension{key="…" type="…" parameters='…' layout? localId? text?}`                                    | extension                  | Bodiless macros; `key`/`type` are the ADF extensionKey/extensionType; `parameters` carries arbitrary JSON in the canonical attr encoding                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| `::syncBlock{localId="…" resourceId="…"}`                                                                | syncBlock                  | A reference to a synced block                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |

## Text directives (inline elements)

| Markdown                                                | ADF                  | Notes                                                                                                                                                                                   |
| ------------------------------------------------------- | -------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `:mention[Jane Doe]{#712020:aa…}`                       | mention              | `#…` is the account id; `accessLevel` kept when present; a legacy leading `@` in the label is accepted (stripped)                                                                       |
| `:status[In Progress]{color="blue"}`                    | status               | Lozenge; `style` kept when present                                                                                                                                                      |
| `:date[2026-07-15]{timestamp="1784073600000"}`          | date                 | `timestamp` (ms since epoch, required unless the `YYYY-MM-DD` label supplies it) is authoritative; the label is the UTC day derived from it; `localId` kept when present                |
| `:placeholder[Type something…]`                         | placeholder          | Template placeholder text; `localId` kept when present                                                                                                                                  |
| `:emoji{#custom-id shortName=":team_logo:"}`            | emoji                | Fallback for custom/site emojis only — standard emojis are plain unicode text (🐝). `shortName` (`:name:`) is required; `#<id>` optional; `text` is the optional rendered fallback text |
| `:extension{key="…" type="…" …}`                        | inlineExtension      | Inline macros; same attrs as `::extension` minus `layout`                                                                                                                               |
| `:annotation[text]{#id annotationType="inlineComment"}` | annotation mark      | Confluence inline-comment anchor — pushing a body without it orphans the comment thread, so the mark must survive                                                                       |
| `:color[text]{color="#ff5630"}`                         | textColor mark       |                                                                                                                                                                                         |
| `:bg[text]{color="#fffae6"}`                            | backgroundColor mark |                                                                                                                                                                                         |
| `:u[text]`                                              | underline mark       |                                                                                                                                                                                         |
| `:sub[text]` / `:sup[text]`                             | subsup mark          |                                                                                                                                                                                         |
| `:fontSize[text]{small}`                                | _retired_            | Parses (bare value or `size="…"`) but no product supports the mark — it is dropped to plain text with a `fontsize-dropped` diagnostic. Do not author it; text is kept, size lost        |
| `:media{#<media-uuid> collection}`                      | mediaInline          | Inline attachment chip. `type` defaults to `file` and is left out when canonical; a bare `collection` is an empty collection, and its absence means none                                |

Mark directives nest with regular emphasis:
`:color[**bold red**]{color="#ff5630"}`. Inline mark directives wrap per
text run in fixed nesting order (outside → inside): `:annotation`,
`:color`, `:bg`, `:u`, `:sub`/`:sup`. (`:fontSize` is retired — it parses
but drops to plain text.) Directive labels cannot nest brackets, so
overlapping annotation marks on one text run degrade to the outermost
anchor.

## Mentions

Current syntax: `:mention[Jane Doe]{#712020:aa11}` — the label is the
display name **without** a leading `@`. A legacy `@` in the label
(`:mention[@Jane Doe]{…}`) is accepted on parse and stripped; the
canonical rendering never carries it.

## Decision lists

A `::decisions` leaf directive marks the **immediately following plain
bullet list** as an ADF decisionList (exactly like `::colwidths` marks
the following table):

```markdown
::decisions

- we requeen Hive B this season, Hive A next year
- no honey harvest before the summer solstice
```

Items encode with state DECIDED. A `::decisions` with no plain bullet
list following is dropped with a `decisions-orphan` diagnostic. Task
lists (`- [ ]`) do not qualify — the list must be a plain bullet list.

## Tables

GFM pipe tables, padded to column width. Cell merging
(remark-extended-table syntax):

- A cell containing only `>` merges into the cell to its **right** — a
  colspan-N cell is written as N−1 `>` markers followed by the content
  cell.
- A cell containing only `^` extends the cell **above** (rowspan).
- Literal `>` / `^` cell content is escaped as `\>` / `\^` so it does
  not read as a marker.
- A marker whose merge cannot apply (a `>` with nothing to its right, a
  `^` with no spanning cell above) is kept as literal text with a
  `span-marker-invalid` diagnostic.

`::colwidths[…]` on the line directly before the table carries the ADF
column widths (one entry per **visual** column).

A header row is synthesized when the ADF table has none.

## Heading anchors

A heading can carry an explicit anchor id as a trailing `{#id}` — the
pandoc / remark-heading-id spelling:

```markdown
## Release process {#release}

Link to it with [the process](#release).
```

The parse is deliberately narrow, so that every accepted form renders
back byte-identically:

- The id must match `[0-9A-Za-z][0-9A-Za-z._-]*`: it opens with an ASCII
  alphanumeric and continues in alphanumerics, `-`, `_`, `.`. A `:` would
  open a text directive and a `*`, `` ` `` or `[` an inline span, so an id
  containing one is not plain text at all and cannot be written back.
- It must be separated from the heading text by a space or tab, or be the
  heading's whole content (`## {#solo}` — an anchor-only heading, as in
  pandoc).
- It must end the heading line.

Anything else stays **literal text**: `{#}`, `{#a b}`, `{.class}`,
`{#a b=c}`, `{bare}`, `## Title{#x}` (no space), and an escaped brace
`## Title \{#lit}`. Escaping is how a literal `{#…}` is written on
purpose; the renderer adds the backslash itself when heading text would
otherwise end in the anchor shape.

**ADF has no platform-neutral anchor.** The id rides through the
conversion as a synthetic attribute that never reaches the wire
(`adf.Heading.Anchor`; `adf.IsWireSafe` reports one left unresolved), and
the host product's addon decides what it becomes:

| Host                         | Encode                                                         | Decode                          |
| ---------------------------- | -------------------------------------------------------------- | ------------------------------- |
| `confluence.MarkdownOptions` | lowered to the anchor macro inside the heading                 | lifted back to `{#id}`          |
| `jira.MarkdownOptions`       | dropped, with a `heading-anchor-dropped` diagnostic per anchor | —                               |
| neither                      | attribute stays (not wire-safe)                                | attribute becomes `{#id}` again |

Confluence anchor names that the `{#id}` surface cannot spell (a space,
say), and headings carrying more than one anchor, stay as
`:anchor[name]` macro directives instead — see the macro sugar below.

## Escaping

- Rendering escapes markdown syntax characters exactly like
  remark-stringify (emphasis markers, brackets, backslashes, character
  references).
- **Colon escaping**: a `:` directly before an ASCII letter (and not
  preceded by another `:`) would re-parse as a text directive, so it is
  written `\:` (`a\:b`); a leading `::` at a block break is escaped the
  same way. Colons before non-letters need no escape (`5:30`).
- **Span-marker escaping**: table cells whose entire content is `>` or
  `^` are written `\>` / `\^` (see Tables).

## Frontmatter

A leading YAML `---` block is split off before parsing and re-emitted
verbatim by the style-preserving formatter; it never reaches ADF.
(Splitting is pluggable via `WithFrontmatterProvider` for other header
conventions.) A malformed opener — a `---` fence that never closes into
a valid block — keeps the bytes as body and raises a
`malformed-frontmatter` diagnostic instead of dropping them. The
`frontmatter` addon module parses/edits the block directly, including a
`PatchPreserving` edit that changes one key while leaving the author's
key order, comments, and quoting untouched.

## Related conventions (no directive needed)

- **Attachments as images** — with a media-asset store wired in, file
  media whose local copy carries every ADF property renders as a plain
  `![alt](assets/shot.png)` and maps back to its media id on encode.
  Anything richer (PDFs, resized media, non-default layouts) keeps the
  `::media` directive.
- **Issue/page links** — a link whose text equals the resolver-derived
  key (e.g. `[ABC-123](https://…/browse/ABC-123)`, or
  `[DOCS/123456](https://…/wiki/spaces/DOCS/pages/123456/…)` with the
  confluence resolver) becomes an inlineCard.
- **Image titles as captions** — `![alt](path "caption")` maps the title
  to a mediaSingle caption child; richer captions (formatting, hard
  breaks) use the `:::media` container form.

## Confluence macro sugar (host-installed)

Hosts that sync Confluence pages install `confluence.Macros()`, which
adds named directives for the core macros. They are ordinary directives —
macro parameters are the attributes, and the macro's unnamed parameter is
the `[label]`:

| Markdown                      | Macro             | Notes                            |
| ----------------------------- | ----------------- | -------------------------------- |
| `::toc{maxLevel="3"}`         | `toc`             | Table of contents                |
| `::children{sort="title"}`    | `children`        | Child pages                      |
| `::pagetree{root="Notes"}`    | `pagetree`        | Page tree                        |
| `:::excerpt{name="…"}` + body | `excerpt`         | Excerpt definition (bodied form) |
| `::excerptInclude[Page]`      | `excerpt-include` | The label is the target page     |
| `::includePage[Page]`         | `include`         | The label is the target page     |
| `:anchor[name]`               | `anchor`          | A link target outside a heading  |

Prefer the `{#id}` heading suffix over `:anchor[name]` on a heading — the
suffix is what a heading anchor lowers to and lifts back from. `:anchor`
carries the anchors that sit elsewhere, where there is no heading for a
suffix to attach to.

Each name also works inline (`:pagetree{…}`) and as a container
(`:::toc`); the ADF node type decides the form on the way back. Do not
write `macroId`, `schemaVersion`, or `title` — Confluence derives them,
so a plain table of contents is just `::toc`. Macros without this sugar
(or with parameters the sugar cannot carry) use the generic
`::extension{key type parameters}` form instead.

## Degradation of unknown directives

Unknown directive names keep the generic directive kinds and degrade
exactly like remark: containers dissolve into their content, unknown
leaves drop, unknown text directives flatten to plain text. Unknown ADF
content survives ADF-level round trips losslessly (RawNode/RawMark) and
is reduced only by the markdown projection, with a `raw-node`
diagnostic.
