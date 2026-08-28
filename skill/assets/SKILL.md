---
name: adfast-markdown
description: Writing or editing markdown destined for Jira/Confluence via adfast (the markdown ⇄ ADF converter) — issue descriptions, comments, or pages that round-trip through ADF.
---

# adfast markdown dialect

adfast converts markdown to and from ADF (the Atlassian document format
behind Jira Cloud and Confluence Cloud). The dialect is **CommonMark +
GFM** (pipe tables, task lists, strikethrough, autolinks, footnotes, YAML
frontmatter) plus **remark-directive-style directives** for ADF features
without native syntax. Everything below round-trips losslessly, except
footnotes (see What to avoid).

## The directive level rule

- `:name[label]{attrs}` — inline (text directive), e.g. a status lozenge
- `::name[label]{attrs}` — a standalone line (leaf directive), e.g. a card
- `:::name` … `:::` — a container around block content, e.g. a panel

Nested containers grow the OUTER fence: a panel holding an expand is
`::::warning` … `:::expand[…]` … `:::` … `::::`.

## Most-used constructs

| Construct  | Example                                                           |
| ---------- | ----------------------------------------------------------------- |
| Panel      | `:::info` body `:::` — also `note`, `warning`, `success`, `error` |
| Expand     | `:::expand[Optional Title]` body `:::`                            |
| Status     | `:status[In Progress]{color="blue"}`                              |
| Mention    | `:mention[Jane Doe]{#712020:aa11}` — label WITHOUT a leading `@`  |
| Date       | `:date[2026-04-12]{timestamp="1775952000000"}`                    |
| Attachment | `::media[shot.png]{#<media-uuid> collection type="file"}`         |
| Captioned  | `:::media[alt]{…attrs…}` + caption text as the body + `:::`       |
| Smart card | `::linkCard[ABC-123]` or `::linkCard[https://…]`                  |
| Col widths | `::colwidths[120,80,220]` on the line directly before a table     |
| Decisions  | `::decisions` line, blank line, then a plain `-` bullet list      |

Table cell merging (GFM tables): a cell containing only `>` merges into
the cell to its RIGHT (colspan); a cell containing only `^` extends the
cell ABOVE (rowspan). Literal `>` / `^` cell content is escaped as
`\>` / `\^`.

## What to avoid

- **Unknown directive names** degrade like remark on the way to ADF:
  containers dissolve into their content, unknown leaves drop, unknown
  text directives flatten to plain text — each with a diagnostic when a
  sink is wired. The md → md formatter keeps them all.
- **Raw HTML**: ADF has no mapping — block HTML is dropped, inline tags
  become literal text. Use directives instead.
- **Local image paths** (`![alt](assets/x.png)`) drop from the ADF
  payload unless an asset store is wired in
  (see references/pitfalls.md).
- **Footnotes** (`[^1]` with `[^1]: note`) parse and survive md → md
  untouched, but ADF has no footnote: the ADF route flattens each
  reference to a superscript number and collects the definitions in an
  ordered list behind a rule at the end of the document
  (`footnote-flattened`). Use one only when that reading is acceptable —
  it is the one construct that does not come back.
- **Brackets inside directive labels** cannot nest; keep labels flat.
- A `:` directly before a letter reads as a directive start; adfast
  escapes it as `\:` when rendering — do the same when writing by hand
  (`a\:b`, but `5:30` needs no escape).

## References

- `references/syntax.md` — the complete dialect: every directive with
  level, attributes, bare-value forms, and escaping rules.
- `references/adf-coverage.md` — ADF node/mark → markdown mapping with
  Jira/Confluence availability.
- `references/example.md` — a complete worked document exercising the
  dialect (kept format-stable by a test; safe to copy from).
- `references/pitfalls.md` — asset-store requirements, attachment
  scoping, diagnostics to watch, depth limits, wire safety, supported
  code languages.
