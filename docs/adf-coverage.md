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

## A linked image: `link` on a media node

A link wrapping an image — `[![the logo](logo.png)](https://home/)`, a
logo that links home or a badge that links to a build — **is
representable in ADF**, and the destination belongs on the media node,
not on a wrapper. Two sources say so:

- **The pinned schema.** `MediaDefinition` and `MediaInlineDefinition`
  both declare
  `marks?: Array<LinkDefinition | BorderMarkDefinition | AnnotationMarkDefinition>`
  ([media.ts#L28](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/media.ts#L28),
  [media-inline.ts#L18](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/media-inline.ts#L18)).
  `MediaSingleBaseDefinition` also takes `marks?: Array<LinkDefinition>`
  ([media-single.ts#L20](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/media-single.ts#L20)),
  so both placements are schema-legal.
- **The Jira reference.** The
  [`nodes/media`](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/media/)
  page (HTTP 200) states it outright: _"The following marks can be
  applied: `border`, `link`"_. That is positive documented evidence for
  Jira, which the `mediaSingle` placement does not have — so **adfast
  writes the mark on the `media` (or `mediaInline`) node**. The
  neighboring `border` mark in the same union was render-confirmed on
  `media` in the 2026-07-22 live probe, which is corroborating live
  evidence that a media node carries its marks through.

What adfast does with each form of a linked image on the **encode**
(md → ADF) leg. Every reference spelling behaves as its inline
equivalent (`[![alt][logo]][home]` and `[![logo]][home]` included): the
references resolve first, and both definitions count as used.

| Markdown                                      | ADF                                                      | Lost                     |
| --------------------------------------------- | -------------------------------------------------------- | ------------------------ |
| lone in a paragraph, external image           | `mediaSingle` → `media`(external) + `link` mark          | nothing                  |
| lone in a paragraph, image in the asset store | `mediaSingle` → `media`(file) + `link` mark              | nothing                  |
| mid-sentence, image in the asset store        | `mediaInline` + `link` mark                              | nothing                  |
| mid-sentence, external image                  | one `text` node, `link` mark = **the outer** destination | the image URL            |
| the image cannot be placed (no asset store)   | one `text` node, `link` mark = **the outer** destination | the picture              |
| the image cannot be placed, and has no label  | nothing — the label converted away                       | the picture and the href |

The last three rows are the lossy ones, and none is silent:

- **mid-sentence external.** ADF has no inline external image at all
  (`mediaInline` addresses an uploaded attachment by id and has no
  external variant), so the image degrades to a link — and with two
  candidate hrefs and one link mark, the **enclosing** destination wins,
  because that is the one the reader means to click. The image URL is
  what leaves the document, and an `inline-image-degraded` diagnostic
  names both halves. The block form of the same picture loses nothing,
  so moving it onto its own line is the fix an author can apply.
- **unplaceable image.** ADF addresses an attachment by media id, so an
  image the store cannot map has no node for the PICTURE — in a
  paragraph as much as mid-sentence, because there is no block media to
  promote to either (`WithPreserveLocalImages` is the opt-in that keeps
  the path as external media instead). The degradation is the one the
  mid-sentence external form already performs: the **label** stays, as a
  link, and the **enclosing** destination wins the href when the image
  sits in one. An `unresolved-asset` diagnostic names both halves — the
  picture that will not be on the page, and the label that will. It is
  the only loss here that is not permanent: upload the asset and the
  next encode finds its id.

  The directive spellings of the same reference — `::media[alt]{path=…}`
  and `:::media`, what a pulled document writes for a downloaded
  attachment, and the inline `:media[alt]{path=…}` chip — take a
  different loss and report the same code. The directive is explicit and
  carries the alt text and the caption, so the media node ships; but
  `path` is a lookup key, not an ADF field, so with nothing behind it the
  node is written with an EMPTY id and the path is gone. A path spelled
  beside an explicit `id` is not reported: the id still addresses the
  attachment.

  All three spellings resolve the path the same way, through one asset
  store lookup. The inline chip used to resolve nothing at all — it
  dropped `path` on encode, so it shipped without an id even when the
  store held the file, and reported nothing because it never looked.

  Emitting nothing instead is what this row used to say, and the cost
  was out of all proportion to the picture. An image is very often the
  ONLY child of its block, so the drop emptied the block: a paragraph, a
  table cell, a list item came back blank, and a document that was one
  image came back EMPTY — the diagnostic the only surviving trace of the
  whole document. The label costs nothing to keep and stops the loss at
  the picture.

  The image title (`![alt](path "caption")`) travels too, on the title
  attribute of the link mark the label keeps — it follows whichever href
  won, so an enclosing link contributes its own title rather than the
  image's. The block form would have spelled the title as a `caption`
  child of `mediaSingle`, which the degradation has no node for; the
  mark's own attribute is where it lands instead.
- **unplaceable image with no label.** An image with neither alt text
  nor a destination to name it after (`![]()`) has no label to keep, so
  it still converts away — and says nothing, because there is no asset
  behind it to resolve later. Inside a link it is also the one markdown
  form that empties a whole label, leaving an ADF link mark with no node
  to ride on; a `link-destination-dropped` diagnostic names that href.

### Reading one back

The **decode** (ADF → md) leg reads the mark from the same place encode
writes it — the `media` or `mediaInline` node — and it has two ways to
say it, because not every media node has a markdown image form:

| ADF                                                 | Markdown                             | Lost    |
| --------------------------------------------------- | ------------------------------------ | ------- |
| `mediaSingle` → `media` + `link`, image-expressible | `[![alt](url)](href)`                | nothing |
| `media` + `link`, image form blocked                | `::media[alt]{href="…" …}`           | nothing |
| `mediaGroup` member `media` + `link`                | `::media{… group="true" href="…"}`   | nothing |
| `mediaSingle` → `media` + `link` + `caption`        | `[![alt](url "caption")](href)`      | nothing |
| `mediaInline` + `link`, asset store knows the path  | `see [![alt](path)](href) here`      | nothing |
| `mediaInline` + `link`, no asset store              | `see :media[alt]{#id href="…"} here` | nothing |

The `[![alt](url)](href)` wrapper is the exact markdown the encode leg
turns back into `media` plus a `link` mark, so the two legs describe one
document identically and a pull → push cycle is a no-op. The directive
forms have no room for a markdown link around a block leaf, so they
carry the destination as an `href` attribute — the same way the
neighboring `border` mark rides as `borderColor`/`borderSize`. The
attribute is spelled `href` because `url` already means the picture's
own source. `::media`/`:media` read it back on encode, so both spellings
reach the same ADF.

**Placement, when a remote disagrees with itself.** Both placements are
schema-legal, so decode reads the `media` node first and falls back to
the `mediaSingle` wrapper: a document written by Atlassian's own editor
may spell it on the wrapper, and dropping that would lose the same href
this projection exists to keep. When BOTH carry a mark and they
disagree, **the media node wins** — a picture with two different
destinations has no markdown form (a link cannot nest), and the media
node's is the placement adfast itself writes.

**The link mark's title travels too.** A media `link` mark's
`LinkAttributes` is the same type as a text link's, so the advisory title
is representable there as well, and **it survives both legs**:

| Markdown                                  | ADF `link` mark attrs on the `media` node |
| ----------------------------------------- | ----------------------------------------- |
| `[![alt](url)](href "Home")`              | `{"href": "href", "title": "Home"}`       |
| `::media[alt]{href="…" hrefTitle="Home"}` | the same mark — both spellings agree      |
| `[![alt](url)](href)`                     | `{"href": "href"}` — no title attribute   |

The directive forms carry it as `hrefTitle`, a compound name for the same
reason `borderColor`/`borderSize` are compound: it belongs to the `href`,
and a bare `title` beside `url` would read as the picture's own caption.
**No `href`, no title:** the encode builds no `link` mark from a title
alone, so both legs drop an `hrefTitle` with no destination beside it.

The IMAGE's own title is a different fact and lands elsewhere — the block
form spells it as a `mediaSingle` caption child, a sibling node rather
than an attribute on the mark — so the two ride together as
`[![alt](url "caption")](href "Home")`.

Reading a title back needs BOTH decodes to write it (dialect's decode
hooks and `convert/normalize.go`'s hand-written mirror of them), which is
why the encode side carried the href alone at first: an attribute one leg
writes and neither reads is a loss dressed as a gain. What still does not
come back is the rest of the mark — the id, collection and occurrenceKey a
`link` mark may carry have no place in a markdown link, and the text-link
projection drops them the same way. See the text-link title below for the
evidence trail on the attribute itself.

### The `annotation` mark: an inline comment anchor on a picture

The third member of the media mark union is `annotation`, the anchor that
ties a picture to a Confluence inline comment thread. It has **no
markdown form at all** — `:annotation[…]` wraps inline CONTENT, and a
block media leaf is not content it can wrap — so, like `border`, it rides
as directive attributes:

| ADF `annotation` mark attrs on the media node | Markdown                                                         |
| --------------------------------------------- | ---------------------------------------------------------------- |
| `{"id": "ann-1", "annotationType": "…"}`      | `::media[alt]{annotationId="ann-1" annotationType="…" …}`        |
| the same, on a `mediaInline`                  | `see :media[alt]{#id annotationId="ann-1" annotationType="…"} …` |
| the same, on a `mediaGroup` member            | `::media{… group="true" annotationId="ann-1" …}`                 |
| `{"annotationType": "…"}` — no id             | nothing; the anchor is dropped                                   |

The names are compound for the same reason `hrefTitle` is: a bare `#id`
on a `::media` node is already the media's own. `annotationType` is
always written, defaulting to `inlineComment` — the same default the
inline `:annotation` projection applies.

**The anchor blocks the image form**, exactly as a border does: a
`![alt](url)` has nowhere to put an attribute, so an annotated picture
falls back to the directive even when the asset store could resolve it to
a path. This is where `annotation` differs from `link`, the one union
member that does NOT block — markdown has `[![alt](url)](href "title")`,
so a destination and its title fit the wrapper while an anchor does not.

**Why it is carried rather than reported.** A diagnostic would name the
loss without preventing it: pushing a body whose picture lost its anchor
ORPHANS the comment thread on the remote. An anchor with no `id`,
however, names no thread, so there is nothing to re-attach and nothing
worth degrading a picture for — it is dropped and does not block, which
mirrors an id-less `:annotation` dissolving to its children.

## A link title: `title` on a text link mark

A markdown link may spell advisory text after its destination —
`[label](href "title")`, what an HTML `<a title="…">` shows on hover.
**It survives both legs**, on the `title` attribute of the `link` mark:

| Markdown                    | ADF `link` mark attrs                        |
| --------------------------- | -------------------------------------------- |
| `[spec](./spec.md "Title")` | `{"href": "./spec.md", "title": "Title"}`    |
| `[spec](./spec.md)`         | `{"href": "./spec.md"}` — no title attribute |

Absent rather than empty is the contract. Markdown has no syntax for an
empty-but-present title, so a titleless link writes no attribute at all;
otherwise every link adfast has ever produced would grow a `"title": ""`
that no author asked for and that a remote read would hand straight back
as content. ADF that arrives with `"title": ""` still re-encodes as it
came, because the field is a pointer.

All three reference spellings reach the same place — a definition's title
(`[spec]: ./spec.md "Title"`) rides onto the inline form the trip
produces — and so does a title on an inline-code label,
``[`spec`](./spec.md "Title")``. A link wrapped around an image carries
one as well — `[![alt](url)](href "Title")`, on the `link` mark of the
`media` node; see the media section above for that spelling and its
`hrefTitle` directive form.

**The attribute is spelled `title`**, on three independent sources:

- the pinned schema's `LinkAttributes`, `title?: string`
  ([link.ts#L32](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/marks/link.ts#L32)),
  whose serializer emits it from
  `OPTIONAL_ATTRS = ['title', 'id', 'collection', 'occurrenceKey', '__confluenceMetadata']`;
- the published JSON schema of the same package
  (`@atlaskit/adf-schema@57.3.0`,
  `dist/json-schema/v1/full.json`): `link_mark.attrs` has properties
  `href`, `title`, `id`, `collection`, `occurrenceKey`, all `string`,
  `required: ["href"]`, `additionalProperties: false`;
- the [Jira ADF reference for `marks/link`](https://developer.atlassian.com/cloud/jira/platform/apis/document/marks/link/)
  (200), which documents `attrs.title` as "the equivalent of the `title`
  value for an HTML `<a>` element".

**Whether a product keeps the title across a save: could NOT be
established.** Jira documents the attribute outright, which is the
strongest per-product documented evidence available for it. Confluence
neither confirms nor denies: `confluence-schema.ts` lists mark _names_
only, with no attribute constraints, and there is no enumerated
Confluence ADF reference. And upstream contradicts itself for BOTH
products — the modern ProseMirror `MarkSpec` factory in
[next-schema/generated/markTypes.ts](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/next-schema/generated/markTypes.ts)
declares the link mark's attrs as `{ href, __confluenceMetadata }`, so
`title` is not in that whitelist and `Mark.fromJSON` would strip it
wherever a product rehydrates ADF through the editor schema. No fixture
here carries a link-mark title, and only a live probe could settle it,
which the test suite must not do. **What this means for a caller:** if a
product does strip the title on save, a local title becomes a difference
no push can settle, and a diff-driven workflow should expect that rather
than treat it as a bug in this projection.

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
