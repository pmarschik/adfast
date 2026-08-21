# ADF coverage — fully cited matrix

This document is the evidence-backed companion to the **ADF coverage**
section of [`README.md`](../README.md). It lists every ADF node and mark
that adfast handles. Each row carries two things: (1) a link to the
upstream schema definition, pinned to a commit SHA, and (2) the evidence
behind the per-product availability marker.
[`adf-availability.json`](adf-availability.json) is the machine-readable
form.

## Provenance

- **Schema mirror:** [`pioug/atlassian-frontend-mirror`](https://github.com/pioug/atlassian-frontend-mirror)
  — a public daily mirror of the `atlassian-frontend` monorepo of
  Atlassian, which is the home of the upstream `@atlaskit/adf-schema`
  package.
- **Pinned commit:** `f5ca0f120c6ea5d79873805d081a72c82917e1f8` (2026-07-21).
  Every schema link below is pinned to this SHA.
- **Schema base path:** [`editor/adf-schema/src/schema`](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema)
- **Doc snapshot date:** 2026-07-22.

## Legend

A per-product marker states whether the kind can occur in the documents
of that product. Since 2026-07-22 the markers hold **live render and
round-trip evidence** (see
[Empirical validation](#empirical-validation-2026-07-22)). That evidence
supersedes the documentation-by-omission that the schema and reference
columns still cite:

- **✓ — available.** The product renders it first-class or
  degraded-but-present (Jira), or keeps it on save (Confluence).
- **∘ — in the shared schema, genuinely untestable here.** The kind is
  present in the shared default ADF schema, but no test here can
  determine its availability. File media behind an attachment gate is
  the example.
- **— — not available.** The render drops it, the ADF endpoint of the
  product rejects it, or the save strips or downgrades it.

The **Support** column is the handling of the kind by adfast itself and
is **independent of product availability**:

- **converted** — the kind has a markdown mapping and round-trips
  through it.
- **preserved** — the kind survives ADF decode → encode losslessly, as a
  typed node or as `RawNode`/`RawMark`, but the markdown projection
  drops or reduces it and emits a `raw-node` diagnostic. _(No tabled
  kind is preserved-only. The category covers unknown and undocumented
  ADF, which is why no row below carries this value.)_
- **dropped** — retired. adfast never produces the kind, and a legacy
  instance decodes to plain text and emits a `fontsize-dropped`
  diagnostic: the text is kept and the styling is lost. `fontSize` is
  the only such kind (see [Retired marks](#retired-marks)).

## Evidence model

The schema does **not** make a product marker self-evident. The shared
ADF schema is a superset of what any one product accepts. The evidence
comes, in priority order, from these three sources:

1. **Per-product schemas in the mirror.** The mirror ships
   [`jira-schema.ts`](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/jira-schema.ts)
   and
   [`confluence-schema.ts`](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts).
   **Both are `@deprecated [ED-15676]`** — "We have stopped supporting
   product specific schemas. Use `@atlaskit/adf-schema/schema-default`
   instead." They are stale, non-exhaustive snapshots:
   - `jira-schema.ts` is a minimal _editor_ schema behind configuration
     gates. Its base set is only `doc, paragraph, text, hardBreak,
     heading, rule`, plus a few additions behind feature flags. It lists
     materially less than Jira Cloud renders in practice: no `panel`, no
     `status`, no `date`, no `inlineCard`, no `expand`, and more. It is
     therefore **not** used to set the Jira markers below.
   - `confluence-schema.ts` is a fixed allowlist and the best
     machine-readable per-product source that exists for Confluence. Its
     `nodes` and `marks` arrays are cited as the primary Confluence
     evidence. But **absence from it is evidence-by-omission only**, not
     proof of non-support, because it predates newer features such as
     sync blocks and status lozenges.
   - The modern `next-schema/` node definitions carry no per-product
     metadata, only a `stage0` staging flag, so they cannot serve as a
     current per-product allowlist.
2. **Atlassian developer docs — Jira.** The
   [Jira Cloud ADF reference](https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/)
   enumerates the nodes and marks of a Jira document. A dedicated node or
   mark page (HTTP 200) is positive "documented available" evidence for
   **Jira**. A 404 is evidence-by-omission. The reference warns that it
   is non-exhaustive: _"Marks and nodes included in the JSON schema may
   not be valid in this implementation. Refer to this documentation for
   details of supported marks and nodes."_ **There is no equivalent
   enumerated Confluence ADF reference.**
   `developer.atlassian.com/cloud/confluence/apis/document/*` returns
   404, which is why the Confluence evidence falls back to
   `confluence-schema.ts`.
3. **Shared-schema existence** is the definition file of the kind under
   `schema/{nodes,marks}/`. It backs the `∘` marker and the
   schema-definition links.

The version-pinned JSON schema
([`unpkg.com/@atlaskit/adf-schema`](https://unpkg.com/browse/@atlaskit/adf-schema/dist/json-schema/v1/),
canonically [`go.atlassian.com/adf-json-schema`](https://go.atlassian.com/adf-json-schema))
is the fallback artifact for the "exists in the shared schema" claim.

> **Line anchors.** Each schema link points at the `@name` or `export`
> line of the spec, verified at the pinned SHA. Where several kinds share
> one file (`tableNodes.ts`, `multi-bodied-extension.ts`,
> `task-item.ts`), each anchor targets the declaration of its own kind.

## Nodes

| ADF node (`type`)    | Jira | Confluence | Support   | Schema definition (pinned)                                                                                                                                                                         | Jira evidence                                                                                                                 | Confluence evidence                                                                                                                                                                      |
| -------------------- | ---- | ---------- | --------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| doc                  | ✓    | ✓          | converted | [doc.ts#L15](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/doc.ts#L15)                                       | [nodes/doc](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/doc/) (200)                               | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| paragraph            | ✓    | ✓          | converted | [paragraph.ts#L15](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/paragraph.ts#L15)                           | [nodes/paragraph](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/paragraph/) (200)                   | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| text                 | ✓    | ✓          | converted | [text.ts#L5](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/text.ts#L5)                                       | [nodes/text](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/text/) (200)                             | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| heading              | ✓    | ✓          | converted | [heading.ts#L8](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/heading.ts#L8)                                 | [nodes/heading](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/heading/) (200)                       | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| blockquote           | ✓    | ✓          | converted | [blockquote.ts#L20](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/blockquote.ts#L20)                         | [nodes/blockquote](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/blockquote/) (200)                 | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| rule                 | ✓    | ✓          | converted | [rule.ts#L8](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/rule.ts#L8)                                       | [nodes/rule](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/rule/) (200)                             | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| codeBlock            | ✓    | ✓          | converted | [code-block.ts#L30](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/code-block.ts#L30)                         | [nodes/codeBlock](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/codeBlock/) (200)                   | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| bulletList           | ✓    | ✓          | converted | [bullet-list.ts#L7](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/bullet-list.ts#L7)                         | [nodes/bulletList](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/bulletList/) (200)                 | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| orderedList          | ✓    | ✓          | converted | [ordered-list.ts#L7](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/ordered-list.ts#L7)                       | [nodes/orderedList](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/orderedList/) (200)               | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| listItem             | ✓    | ✓          | converted | [list-item.ts#L6](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/list-item.ts#L6)                             | [nodes/listItem](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/listItem/) (200)                     | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| taskList             | ✓    | ✓          | converted | [task-list.ts#L14](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/task-list.ts#L14)                           | **no** `nodes/taskList` page (404) — omission. render-confirmed 2026-07-22                                                    | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| taskItem             | ✓    | ✓          | converted | [task-item.ts#L12](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/task-item.ts#L12)                           | **no** `nodes/taskItem` page (404) — omission. render-confirmed 2026-07-22                                                    | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| blockTaskItem        | ✓    | —          | converted | [task-item.ts#L28](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/task-item.ts#L28)                           | no page (404); shared-schema only                                                                                             | absent from confluence-schema.ts; shared-schema only                                                                                                                                     |
| decisionList         | ✓    | ✓          | converted | [decision-list.ts#L7](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/decision-list.ts#L7)                     | no page (404); shared-schema only                                                                                             | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| decisionItem         | ✓    | ✓          | converted | [decision-item.ts#L7](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/decision-item.ts#L7)                     | no page (404); shared-schema only                                                                                             | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| table                | ✓    | ✓          | converted | [tableNodes.ts#L369](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/tableNodes.ts#L369)                       | [nodes/table](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/table/) (200)                           | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| tableRow             | ✓    | ✓          | converted | [tableNodes.ts#L383](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/tableNodes.ts#L383)                       | [nodes/table_row](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/table_row/) (200)                   | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| tableCell            | ✓    | ✓          | converted | [tableNodes.ts#L419](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/tableNodes.ts#L419)                       | [nodes/table_cell](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/table_cell/) (200)                 | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| tableHeader          | ✓    | ✓          | converted | [tableNodes.ts#L428](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/tableNodes.ts#L428)                       | [nodes/table_header](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/table_header/) (200)             | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| panel                | ✓    | ✓          | converted | [panel.ts#L45](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/panel.ts#L45)                                   | [nodes/panel](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/panel/) (200)                           | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| expand               | ✓    | ✓          | converted | [expand.ts#L12](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/expand.ts#L12)                                 | [nodes/expand](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/expand/) (200)                         | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| nestedExpand         | ✓    | ✓          | converted | [nested-expand.ts#L23](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/nested-expand.ts#L23)                   | [nodes/nestedExpand](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/nestedExpand/) (200)             | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| mediaSingle          | ✓    | ✓          | converted | [media-single.ts#L20](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/media-single.ts#L20)                     | [nodes/mediaSingle](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/mediaSingle/) (200)               | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| mediaGroup           | ✓    | ✓          | converted | [media-group.ts#L6](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/media-group.ts#L6)                         | [nodes/mediaGroup](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/mediaGroup/) (200)                 | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| media                | ✓    | ✓          | converted | [media.ts#L28](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/media.ts#L28)                                   | [nodes/media](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/media/) (200)                           | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| mediaInline          | ∘    | ✓          | converted | [media-inline.ts#L18](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/media-inline.ts#L18)                     | **no** `nodes/mediaInline` page (404); `jira-schema.ts` (deprecated) lists it under `allowMedia`. render-confirmed 2026-07-22 | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| caption              | ✓    | ✓          | converted | [caption.ts#L14](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/caption.ts#L14)                               | no page (404); shared-schema only                                                                                             | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| inlineCard           | ✓    | ✓          | converted | [inline-card.ts#L8](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/inline-card.ts#L8)                         | [nodes/inlineCard](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/inlineCard/) (200)                 | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| blockCard            | ✓    | ✓          | converted | [block-card.ts#L47](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/block-card.ts#L47)                         | **no** `nodes/blockCard` page (404; only `inlineCard` is documented). render-confirmed 2026-07-22                             | **absent** from confluence-schema.ts. render-confirmed 2026-07-22                                                                                                                        |
| embedCard            | ✓    | ✓          | converted | [embed-card.ts#L14](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/embed-card.ts#L14)                         | **no** `nodes/embedCard` page (404). render-confirmed 2026-07-22                                                              | **absent** from confluence-schema.ts. render-confirmed 2026-07-22                                                                                                                        |
| mention              | ✓    | ✓          | converted | [mention.ts#L23](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/mention.ts#L23)                               | [nodes/mention](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/mention/) (200)                       | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| emoji                | ✓    | ✓          | converted | [emoji.ts#L8](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/emoji.ts#L8)                                     | [nodes/emoji](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/emoji/) (200)                           | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| status               | ✓    | ✓          | converted | [status.ts#L10](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/status.ts#L10)                                 | [nodes/status](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/status/) (200)                         | **absent** from confluence-schema.ts (deprecated snapshot predates it). render-confirmed 2026-07-22                                                                                      |
| date                 | ✓    | ✓          | converted | [date.ts#L7](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/date.ts#L7)                                       | [nodes/date](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/date/) (200)                             | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| hardBreak            | ✓    | ✓          | converted | [hard-break.ts#L5](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/hard-break.ts#L5)                           | [nodes/hardBreak](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/hardBreak/) (200)                   | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| placeholder          | —    | ✓          | converted | [placeholder.ts#L6](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/placeholder.ts#L6)                         | no `nodes/placeholder` page (404); Confluence node — omission                                                                 | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| layoutSection        | ✓    | ✓          | converted | [layout-section.ts#L12](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/layout-section.ts#L12)                 | no page (404); Confluence node — omission                                                                                     | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| layoutColumn         | ✓    | ✓          | converted | [layout-column.ts#L56](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/layout-column.ts#L56)                   | no page (404); Confluence node — omission                                                                                     | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| extension            | ✓    | ✓          | converted | [extension.ts#L10](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/extension.ts#L10)                           | no page (404); Confluence node — omission                                                                                     | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| bodiedExtension      | ✓    | ✓          | converted | [bodied-extension.ts#L11](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/bodied-extension.ts#L11)             | no page (404); Confluence node — omission                                                                                     | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| inlineExtension      | ✓    | ✓          | converted | [inline-extension.ts#L10](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/inline-extension.ts#L10)             | no page (404); Confluence node — omission                                                                                     | [confluence-schema.ts#L4-L49](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L4-L49) |
| multiBodiedExtension | —    | ✓          | converted | [multi-bodied-extension.ts#L96](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/multi-bodied-extension.ts#L96) | no page (404); stage-0 shared schema only                                                                                     | **absent** from confluence-schema.ts (stage-0). render-confirmed 2026-07-22                                                                                                              |
| extensionFrame       | —    | ✓          | converted | [multi-bodied-extension.ts#L32](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/multi-bodied-extension.ts#L32) | no page (404); stage-0 shared schema only                                                                                     | **absent** from confluence-schema.ts (stage-0). render-confirmed 2026-07-22                                                                                                              |
| syncBlock            | ✓    | ✓          | converted | [sync-block.ts#L21](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/sync-block.ts#L21)                         | no page (404) — omission                                                                                                      | **absent** from confluence-schema.ts (predates sync blocks). render-confirmed 2026-07-22                                                                                                 |
| bodiedSyncBlock      | ✓    | ✓          | converted | [bodied-sync-block.ts#L46](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/bodied-sync-block.ts#L46)           | no page (404) — omission                                                                                                      | **absent** from confluence-schema.ts (predates sync blocks). render-confirmed 2026-07-22                                                                                                 |

> `blockCard + datasource` in the README is not a distinct ADF `type`. It
> is a `blockCard` that carries a `datasource` attribute (see the `anyOf`
> in [block-card.ts](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/block-card.ts)),
> so it shares the evidence of `blockCard`.

## Marks

| ADF mark (`type`) | Jira | Confluence | Support   | Schema definition (pinned)                                                                                                                                                             | Jira evidence                                                                                                           | Confluence evidence                                                                                                                                                                        |
| ----------------- | ---- | ---------- | --------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| strong            | ✓    | ✓          | converted | [strong.ts#L5](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/marks/strong.ts#L5)                       | [marks/strong](https://developer.atlassian.com/cloud/jira/platform/apis/document/marks/strong/) (200)                   | [confluence-schema.ts#L50-L70](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L50-L70) |
| em                | ✓    | ✓          | converted | [em.ts#L5](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/marks/em.ts#L5)                               | [marks/em](https://developer.atlassian.com/cloud/jira/platform/apis/document/marks/em/) (200)                           | [confluence-schema.ts#L50-L70](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L50-L70) |
| strike            | ✓    | ✓          | converted | [strike.ts#L5](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/marks/strike.ts#L5)                       | [marks/strike](https://developer.atlassian.com/cloud/jira/platform/apis/document/marks/strike/) (200)                   | [confluence-schema.ts#L50-L70](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L50-L70) |
| code              | ✓    | ✓          | converted | [code.ts#L5](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/marks/code.ts#L5)                           | [marks/code](https://developer.atlassian.com/cloud/jira/platform/apis/document/marks/code/) (200)                       | [confluence-schema.ts#L50-L70](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L50-L70) |
| underline         | ✓    | ✓          | converted | [underline.ts#L5](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/marks/underline.ts#L5)                 | [marks/underline](https://developer.atlassian.com/cloud/jira/platform/apis/document/marks/underline/) (200)             | [confluence-schema.ts#L50-L70](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L50-L70) |
| link              | ✓    | ✓          | converted | [link.ts#L32](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/marks/link.ts#L32)                         | [marks/link](https://developer.atlassian.com/cloud/jira/platform/apis/document/marks/link/) (200)                       | [confluence-schema.ts#L50-L70](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L50-L70) |
| subsup            | ✓    | ✓          | converted | [subsup.ts#L9](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/marks/subsup.ts#L9)                       | [marks/subsup](https://developer.atlassian.com/cloud/jira/platform/apis/document/marks/subsup/) (200)                   | [confluence-schema.ts#L50-L70](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L50-L70) |
| textColor         | ✓    | ✓          | converted | [text-color.ts#L53](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/marks/text-color.ts#L53)             | [marks/textColor](https://developer.atlassian.com/cloud/jira/platform/apis/document/marks/textColor/) (200)             | [confluence-schema.ts#L50-L70](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L50-L70) |
| backgroundColor   | ✓    | ✓          | converted | [background-color.ts#L25](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/marks/background-color.ts#L25) | [marks/backgroundColor](https://developer.atlassian.com/cloud/jira/platform/apis/document/marks/backgroundColor/) (200) | [confluence-schema.ts#L50-L70](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L50-L70) |
| border            | ✓    | ✓          | converted | [border.ts#L22](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/marks/border.ts#L22)                     | **no** `marks/border` page (404). render-confirmed 2026-07-22                                                           | **absent** from confluence-schema.ts. render-confirmed 2026-07-22                                                                                                                          |
| alignment         | ✓    | ✓          | converted | [alignment.ts#L16](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/marks/alignment.ts#L16)               | no `marks/alignment` page (404) — omission                                                                              | **absent** from confluence-schema.ts (live Confluence mark; deprecated snapshot omits it). render-confirmed 2026-07-22                                                                     |
| indentation       | ✓    | ✓          | converted | [indentation.ts#L13](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/marks/indentation.ts#L13)           | no page (404) — omission                                                                                                | **absent** from confluence-schema.ts. render-confirmed 2026-07-22                                                                                                                          |
| breakout          | ✓    | ✓          | converted | [breakout.ts#L12](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/marks/breakout.ts#L12)                 | no page (404) — omission                                                                                                | **absent** from confluence-schema.ts. render-confirmed 2026-07-22                                                                                                                          |
| annotation        | ✓    | ✓          | converted | [annotation.ts#L5](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/marks/annotation.ts#L5)               | no `marks/annotation` page (404); Confluence mark — omission                                                            | [confluence-schema.ts#L50-L70](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts#L50-L70) |
| dataConsumer      | ✓    | ✓          | converted | [data-consumer.ts#L27](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/marks/data-consumer.ts#L27)       | no page (404) — omission                                                                                                | **absent** from confluence-schema.ts. render-confirmed 2026-07-22                                                                                                                          |
| fragment          | ✓    | ✓          | converted | [fragment.ts#L17](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/marks/fragment.ts#L17)                 | no page (404) — omission                                                                                                | **absent** from confluence-schema.ts. render-confirmed 2026-07-22                                                                                                                          |
| fontSize          | —    | —          | dropped   | [font-size.ts#L11](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/marks/font-size.ts#L11)               | no page (404); shared-schema only; REST rejects it                                                                      | absent from confluence-schema.ts; shared-schema only; stripped on save                                                                                                                     |

## Historical documentation gaps (now empirically resolved)

The `Jira` and `Confluence` columns above are **confirmed empirically by
render and round trip (2026-07-22)** — see the next section. They
previously carried a `∘` or `—` marker wherever the upstream
_documentation_ did not positively back the `✓` of adfast. The ADF
reference of Jira admits that it is non-exhaustive, and the Confluence
`confluence-schema.ts` allowlist is deprecated and stale. The live probe
resolved every one of those gaps. Jira renders the great majority of the
kinds its documentation omits: task and decision lists, smart-link
cards, `status`, page layouts, the extension family, `syncBlock`, and the
`alignment`, `indentation`, `breakout`, `annotation`, `fragment`, and
`dataConsumer` marks. Confluence keeps almost everything the deprecated
snapshot omits. The markers now hold that evidence instead of
documentation-by-omission.

## Empirical validation (2026-07-22)

**Full coverage:** all 45 nodes and all 17 marks were written through the
Atlassian API (`contentFormat=adf`) to a live **Jira** issue description
(`ARCH-506`) and a live **Confluence** page (`1729232906`, ENGINEERING
space) on `ixolit.atlassian.net`. An `L-<kind>` label paragraph precedes
each one. Two oracles gave the determination: (1) the **product-rendered
DOM**, inspected read-only in a logged-in browser, and (2) for
Confluence, the **stored ADF read back**, which shows which kinds
survived the save. File media behind an attachment gate (`mediaGroup`,
`mediaInline`) cannot be tested by injection, because a synthetic id
raises `ATTACHMENT_VALIDATION_ERROR`. That is a data error, not a schema
or render signal, so those two kinds rely on the documentation.

### Jira — live render is the product-support oracle

The Jira issue view renders the description ADF with the shared
`@atlaskit/renderer`. The classification runs as follows. First-class
**or** degraded-but-present means available. A drop (no DOM) or an
**unsupported-content block** means not available. A REST `INVALID_INPUT`
rejection means the kind is not in the ADF schema of Jira, and therefore
not available. **No unsupported-content block appeared for any kind.**

**Rendered first-class:** paragraph, text, heading, blockquote, rule,
codeBlock, bullet/ordered lists + listItem, `taskList`/`taskItem`,
`blockTaskItem` (task item with block body), `decisionList`/`decisionItem`,
table family, panel, expand, nestedExpand, `mediaSingle`/`media`/`caption`
(external media), `inlineCard`, `blockCard`, `embedCard`, mention, emoji,
`status`, date, hardBreak, `layoutSection`/`layoutColumn`, and every text
mark (strong, em, strike, code, underline, link, subsup, textColor,
backgroundColor) plus `border`, `alignment` (`data-align`), `indentation`
(`data-level`), `breakout` (`data-mode`), `annotation` (`data-mark-type`).

**Rendered degraded-but-present (available):** `extension` and
`bodiedExtension` (inside an `ak-renderer-extension` container, and the
body content shows), `inlineExtension` (an inline fallback), `syncBlock`
(the sync-block widget renders, in an error state only because the
synthetic `resourceId` does not resolve), `bodiedSyncBlock` (the body
renders), and the `fragment` and `dataConsumer` marks (rendered as
`data-mark-type` wrappers around their extension).

**Not available in Jira (4 kinds):**

- **`placeholder`** — DROPPED. It renders as an empty `<span></span>`,
  and its text is not shown.
- **`fontSize`** — REST `INVALID_INPUT`. The mark is not in the ADF
  schema of Jira, and the endpoint rejects a whole document that carries
  it. adfast **RETIRES** the mark (see below) and never produces it, so
  it cannot reach a Jira push.
- **`multiBodiedExtension`** and **`extensionFrame`** — REST
  `INVALID_INPUT`. They are rejected together, and they are not in the
  ADF schema of Jira.

`jira.UnsupportedKinds` = `placeholder`, `multiBodiedExtension`,
`extensionFrame` (three kinds). `fontSize` is **excluded** deliberately.
adfast retires the mark and never produces it, because the `:fontSize`
directive drops to plain text with a `fontsize-dropped` diagnostic. An
`unsupported-in-product` check for it would therefore be moot.

**Inconclusive:** `mediaInline`. It is attachment-gated, and a synthetic
id raises `ATTACHMENT_VALIDATION_ERROR`. The schema accepts the node, so
the marker stays `∘`.

### Confluence — round-trip survival is the oracle

Confluence strips or downgrades an unsupported kind on save, and it does
so silently. Survival of the round trip, read back through
`contentFormat=adf`, therefore confirms support. The browser render
showed no unsupported-content block.

**Survived (available):** every node and mark **except** the two below.
This includes the two marks that were inconclusive before,
`dataConsumer` and `fragment` (both survived), as well as
`multiBodiedExtension` with `extensionFrame`, `syncBlock`,
`bodiedSyncBlock`, `placeholder`, and every block mark. A known macro
resolves on save: `extension{toc}` stays an extension,
`bodiedExtension{info}` resolves to a native panel, and
`inlineExtension{status}` resolves to a native status node. The node
kinds are still accepted and rendered.

**Not kept in Confluence (2 kinds):**

- **`fontSize`** — the save STRIPS the mark. The text is kept and
  `fontSize` is removed. adfast **RETIRES** the mark (see below) and
  never produces it.
- **`blockTaskItem`** — DOWNGRADED to a plain `taskItem`, with its block
  body flattened to inline content. The distinct kind is not kept.

`confluence.UnsupportedKinds` = `blockTaskItem` (one kind). `fontSize` is
**excluded** deliberately. adfast retires the mark and never produces it,
because the `:fontSize` directive drops to plain text with a
`fontsize-dropped` diagnostic. An `unsupported-in-product` check for it
would therefore be moot.

> Scratch artifacts: Jira issue `ARCH-506` and Confluence page
> `1729232906` (ENGINEERING). Both are safe to erase.

### Retired marks

- **`fontSize`** — the **only** kind adfast classifies as **dropped**
  instead of `converted`. The empirical probe confirmed that neither
  product supports the mark: the REST endpoint of Jira rejects it with
  `INVALID_INPUT`, and Confluence strips it on save. The non-support is
  unanimous, so adfast retires the mark instead of a round trip through
  it. The `:fontSize[text]{size}` directive still **parses**, which keeps
  old documents readable, but on ADF encode it unwraps to its plain-text
  content. No `fontSize` mark is ever produced, and a legacy `fontSize`
  ADF mark decodes to bare text. Both directions emit a
  `fontsize-dropped` diagnostic. The text is always kept, and only the
  size annotation is lost. The mark is therefore absent from both
  product `UnsupportedKinds` sets, where an `unsupported-in-product`
  check would be moot.
