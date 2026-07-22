# ADF coverage — fully cited matrix

This is the evidence-backed companion to the **ADF coverage** section of
[`README.md`](../README.md). Every ADF node and mark adfast handles is
listed here with (1) a link to its upstream schema definition pinned to a
commit SHA, and (2) the evidence behind its per-product availability
marker. The machine-readable form is [`adf-availability.json`](adf-availability.json).

## Provenance

- **Schema mirror:** [`pioug/atlassian-frontend-mirror`](https://github.com/pioug/atlassian-frontend-mirror)
  — a public daily mirror of Atlassian's `atlassian-frontend` monorepo
  (the home of the upstream `@atlaskit/adf-schema` package).
- **Pinned commit:** `f5ca0f120c6ea5d79873805d081a72c82917e1f8` (2026-07-21).
  Every schema link below is pinned to this SHA.
- **Schema base path:** [`editor/adf-schema/src/schema`](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema)
- **Doc snapshot date:** 2026-07-22.

## Legend

Per-product markers describe whether the kind can occur in that product's
documents. As of 2026-07-22 they reflect **live render / round-trip
evidence** (see [Empirical validation](#empirical-validation-2026-07-22)),
which supersedes the documentation-by-omission the schema/reference
columns still cite:

- **✓ — available.** The product renders it first-class or
  degraded-but-present (Jira), or preserves it on save (Confluence).
- **∘ — in the shared schema, genuinely untestable here.** Present in the
  shared default ADF schema but not empirically determinable (e.g.
  attachment-gated file media).
- **— — not available.** Dropped by the render, rejected by the product's
  ADF endpoint, or stripped/downgraded on save.

The **Support** column is adfast's own handling of the kind and is
**independent of product availability**:

- **converted** — has a markdown mapping and round-trips through it.
- **preserved** — survives ADF decode → encode losslessly (typed or as
  `RawNode`/`RawMark`) but is dropped/reduced by the markdown projection
  (emits a `raw-node` diagnostic). _(No tabled kind is preserved-only; the
  category covers unknown/undocumented ADF, which is why it does not appear
  as a value below.)_
- **dropped** — retired: adfast never produces the kind, and a legacy
  instance decodes to plain text (emits a `fontsize-dropped` diagnostic) —
  text kept, styling lost. `fontSize` is the only such kind (see [Retired marks](#retired-marks)).

## Evidence model

Product markers are **not** self-evident from the schema — the shared ADF
schema is a superset of what any one product accepts. Evidence is drawn, in
priority order, from:

1. **Per-product schemas in the mirror.** The mirror ships
   [`jira-schema.ts`](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/jira-schema.ts)
   and
   [`confluence-schema.ts`](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/confluence-schema.ts).
   **Both are `@deprecated [ED-15676]`** — "We have stopped supporting
   product specific schemas. Use `@atlaskit/adf-schema/schema-default`
   instead." They are stale, non-exhaustive snapshots:
   - `jira-schema.ts` is a minimal, config-gated _editor_ schema whose base
     set is only `doc, paragraph, text, hardBreak, heading, rule` plus a
     handful of feature-flag-gated additions. It materially under-lists what
     Jira Cloud actually renders (no `panel`, `status`, `date`, `inlineCard`,
     `expand`, …), so it is **not** used to set Jira markers below.
   - `confluence-schema.ts` is a fixed allowlist and is the best
     machine-readable per-product source that exists for Confluence, so its
     `nodes`/`marks` arrays are cited as the primary Confluence evidence —
     but **absence from it is evidence-by-omission only**, not proof of
     non-support (it predates newer features like sync blocks and status
     lozenges).
   - The modern `next-schema/` node definitions carry no per-product
     metadata (only a `stage0` staging flag), so they cannot serve as a
     current per-product allowlist.
2. **Atlassian developer docs — Jira.** The
   [Jira Cloud ADF reference](https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/)
   enumerates the nodes/marks Jira documents. A dedicated node/mark page
   (HTTP 200) is positive "documented available" evidence for **Jira**; a
   404 is evidence-by-omission. The reference itself warns it is
   non-exhaustive: _"Marks and nodes included in the JSON schema may not be
   valid in this implementation. Refer to this documentation for details of
   supported marks and nodes."_ **There is no equivalent enumerated
   Confluence ADF reference** — `developer.atlassian.com/cloud/confluence/apis/document/*`
   returns 404 — which is why Confluence evidence falls back to
   `confluence-schema.ts`.
3. **Shared-schema existence** (for `∘` and for the schema-definition
   links) is the kind's definition file under `schema/{nodes,marks}/`.

The version-pinned JSON schema
([`unpkg.com/@atlaskit/adf-schema`](https://unpkg.com/browse/@atlaskit/adf-schema/dist/json-schema/v1/),
canonically [`go.atlassian.com/adf-json-schema`](https://go.atlassian.com/adf-json-schema))
is the fallback artifact for the "exists in the shared schema" claim.

> **Line anchors.** Schema links point at the `@name`/`export` line of each
> spec, verified at the pinned SHA. Where several kinds share one file
> (`tableNodes.ts`, `multi-bodied-extension.ts`, `task-item.ts`) each anchor
> targets that kind's own declaration.

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

> `blockCard + datasource` in the README is not a distinct ADF `type` — it
> is a `blockCard` carrying a `datasource` attribute (see the `anyOf` in
> [block-card.ts](https://github.com/pioug/atlassian-frontend-mirror/blob/f5ca0f120c6ea5d79873805d081a72c82917e1f8/editor/adf-schema/src/schema/nodes/block-card.ts)),
> so it shares `blockCard`'s evidence.

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

The `Jira`/`Confluence` columns above are **empirically render/round-trip
confirmed (2026-07-22)** — see the next section. They previously carried
`∘`/`—` markers wherever the upstream _documentation_ did not positively
back adfast's `✓` (Jira's ADF reference is self-admittedly non-exhaustive;
the Confluence `confluence-schema.ts` allowlist is deprecated and stale).
The live probe resolved every one of those gaps: Jira in fact renders the
overwhelming majority of the kinds its docs omit (task/decision lists,
smart-link cards, `status`, page layouts, the extension family,
`syncBlock`, and the `alignment`/`indentation`/`breakout`/`annotation`/
`fragment`/`dataConsumer` marks), and Confluence preserves almost
everything the deprecated snapshot omits. The markers now reflect that
evidence, not documentation-by-omission.

## Empirical validation (2026-07-22)

**Full coverage:** every one of the 45 nodes and 17 marks was written to a
live **Jira** issue description (`ARCH-506`) and a live **Confluence** page
(`1729232906`, ENGINEERING space) on `ixolit.atlassian.net` via the
Atlassian API (`contentFormat=adf`), each preceded by an `L-<kind>` label
paragraph. Determination used two oracles: (1) the **product-rendered DOM**
inspected read-only in a logged-in browser, and (2) for Confluence, the
**stored ADF read back** to see which kinds survived save. Attachment-gated
file media (`mediaGroup`, `mediaInline`) is not injection-testable —
synthetic ids raise `ATTACHMENT_VALIDATION_ERROR` (a data error, not a
schema/render signal) — so those rely on documentation.

### Jira — live render is the product-support oracle

The Jira issue view renders description ADF with the shared
`@atlaskit/renderer`. Classification: renders first-class **or**
degraded-but-present = available; **dropped** (no DOM) / **unsupported-
content block** = not available; a REST `INVALID_INPUT` rejection = the
kind is not in Jira's ADF schema = not available. **No unsupported-content
blocks appeared for any kind.**

**Rendered first-class:** paragraph, text, heading, blockquote, rule,
codeBlock, bullet/ordered lists + listItem, `taskList`/`taskItem`,
`blockTaskItem` (task item with block body), `decisionList`/`decisionItem`,
table family, panel, expand, nestedExpand, `mediaSingle`/`media`/`caption`
(external media), `inlineCard`, `blockCard`, `embedCard`, mention, emoji,
`status`, date, hardBreak, `layoutSection`/`layoutColumn`, and every text
mark (strong, em, strike, code, underline, link, subsup, textColor,
backgroundColor) plus `border`, `alignment` (`data-align`), `indentation`
(`data-level`), `breakout` (`data-mode`), `annotation` (`data-mark-type`).

**Rendered degraded-but-present (available):** `extension` /
`bodiedExtension` (in an `ak-renderer-extension` container; body content
shows), `inlineExtension` (inline fallback), `syncBlock` (renders the
sync-block widget, error state only because the synthetic `resourceId`
doesn't resolve), `bodiedSyncBlock` (body renders), and the `fragment` /
`dataConsumer` marks (rendered as `data-mark-type` wrappers around their
extension).

**Not available in Jira (4 kinds):**

- **`placeholder`** — DROPPED: renders as an empty `<span></span>`; its
  text is not shown.
- **`fontSize`** — REST `INVALID_INPUT`: the mark is not in Jira's ADF
  schema (a doc carrying it is rejected outright). **RETIRED by adfast**
  (see below) — never produced, so it cannot reach a Jira push.
- **`multiBodiedExtension`** / **`extensionFrame`** — REST `INVALID_INPUT`
  (rejected together; not in Jira's ADF schema).

`jira.UnsupportedKinds` = `placeholder`, `multiBodiedExtension`,
`extensionFrame` (three kinds). `fontSize` is deliberately **excluded**:
adfast retires it — the mark is never produced (the `:fontSize` directive
drops to plain text with a `fontsize-dropped` diagnostic), so an
`unsupported-in-product` check for it would be moot.

**Inconclusive:** `mediaInline` (attachment-gated; `ATTACHMENT_VALIDATION_
ERROR` with a synthetic id — the schema accepts the node, so left `∘`).

### Confluence — round-trip survival is the oracle

Confluence silently strips or downgrades unsupported kinds on save;
round-trip survival (read-back via `contentFormat=adf`) confirms support,
and the browser render confirmed no unsupported-content blocks.

**Survived (available):** every node and mark **except** the two below —
including the previously-inconclusive `dataConsumer` and `fragment` marks
(both survived), `multiBodiedExtension`+`extensionFrame`, `syncBlock`,
`bodiedSyncBlock`, `placeholder`, and all the block marks. Known macros are
resolved on save (`extension{toc}` stays an extension; `bodiedExtension{info}`
resolves to a native panel; `inlineExtension{status}` resolves to a native
status node) — the node kinds are still accepted and rendered.

**Not preserved in Confluence (2 kinds):**

- **`fontSize`** — the mark is STRIPPED on save (text kept, `fontSize`
  removed). **RETIRED by adfast** (see below) — never produced.
- **`blockTaskItem`** — DOWNGRADED to a plain `taskItem` (its block body is
  flattened to inline); the distinct kind is not preserved.

`confluence.UnsupportedKinds` = `blockTaskItem` (one kind). `fontSize` is
deliberately **excluded**: adfast retires it — the mark is never produced
(the `:fontSize` directive drops to plain text with a `fontsize-dropped`
diagnostic), so an `unsupported-in-product` check for it would be moot.

> Scratch artifacts: Jira issue `ARCH-506` and Confluence page
> `1729232906` (ENGINEERING) — safe to delete.

### Retired marks

- **`fontSize`** — the **only** kind adfast classifies as **dropped** (not
  `converted`). The empirical probe confirmed neither product supports the
  mark: Jira's REST endpoint rejects it (`INVALID_INPUT`) and Confluence
  strips it on save. Because non-support is unanimous, adfast retires it
  rather than round-tripping it: the `:fontSize[text]{size}` directive still
  **parses** (so old documents read cleanly), but on ADF encode it unwraps
  to its plain-text content — no `fontSize` mark is ever produced — and a
  legacy `fontSize` ADF mark decodes to bare text. Both directions emit a
  `fontsize-dropped` diagnostic. The text is always preserved; only the
  size annotation is lost. It is therefore absent from both product
  `UnsupportedKinds` sets (an `unsupported-in-product` check would be moot).
