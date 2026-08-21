---
title: Rooftop apiary — season plan
labels: [bees, community-garden]
---

# Rooftop apiary — season plan

Kept by :mention[Maya Winters]{#712020:aa11} ·
:status[In Progress]{color="blue"} · first inspection
:date[2026-04-12]{timestamp="1775952000000"} 🐝

:::info
This page is synced from markdown — edits here survive the round trip, including
the :annotation[inline comments]{#c9e1 annotationType="inlineComment"} your
co-keepers left.
:::

## Season scope

This year covers **two hives** and _one nucleus colony_, ~~three hives~~, and
the `varroa` monitoring plan. Sugar syrup is mixed :sub[1] / :sup[1] in spring,
alerts show in :color[red]{color="#ff5630"} on :bg[highlight]{color="#fff0b3"},
and queen marks in :u[underline].

Rooftop rules:\
no open smoker near the door, and new keepers sign the [rota](#rota) as
:placeholder[your name here…].

- [ ] assemble the new brood boxes
- [x] order spring sugar syrup
- [ ] paint the new stands

  Use the leftover green from the shed door.

Decisions so far:

::decisions

- we requeen Hive B this season, Hive A next year
- no honey harvest before the summer solstice

1. Clean and scorch the empty boxes
2. Split the strongest colony
3. Merge the nucleus before winter

---

## Hive setup

:::expand[Why a vertical hive stand?]
The stand keeps landing boards clear of the gravel; see
[BEE-42](https://hive.example.org/browse/BEE-42) and the club wiki at
https://wiki.example.org/apiary.
:::

::::warning
Mind the parapet ledge when hauling supers.

:::expand[Storage map]
Frames live in the attic crates; smoker fuel stays in the metal locker.
:emoji{#1f9a9-custom shortName=":county_bee:"}
:::
::::

```python
if colony.strength() > SPLIT_THRESHOLD:
    apiary.split(colony)
```

## Inspection rota {#rota}

::colwidths[120,80,220]

| Keeper | Week | Notes                       |
| ------ | ---- | --------------------------- |
| Maya   | 15   | queen spotting              |
| Sam    | >    | mite count                  |
| ^      | 17   | shares the mite-count sheet |

## Task board

::linkCard[https://hive.example.org/browse/BEE-42]

::linkEmbed[https://wiki.example.org/apiary/map]{layout="center" width="80"}

::jql[project = APIARY AND status = Open]{cloudId="1234abcd-12ab-34cd-56ef-123456abcdef" datasource="d8b5e8c9-5f4a-4a10-9c2f-05b0e3a5b0e3"}

Hive scale: :extension{key="scale" type="com.example"}

::extension{key="weather-widget" parameters='{"station":"rooftop"}' type="com.example.apiary"}

:::extension{key="inspection-log" type="com.example.apiary"}
Entries in this body render inside the inspection-log macro.
:::

::::extension{key="season-tabs" type="com.example.apiary"}
:::frame
Spring: feed, inspect, split.
:::

:::frame
Summer: supers on, harvest after solstice.
:::
::::

## Shared checklists

:::syncBlock{localId="safety-1" resourceId="ari:cloud:example:page/123"}
Zip the suit before opening any hive.
:::

::syncBlock{localId="safety-1" resourceId="ari:cloud:example:page/123"}

## Layout

:::::section
::::column{width="50"}
:::center
**Before**: two weathered hives
:::
::::

::::column{width="50"}
:::center
**After**: painted stands, three colonies
:::
::::
:::::

:::indent{2}
The nucleus stands two paces in from the parapet edge.
:::

:::end
Wind readings align to the end of the content column.
:::

:::dataConsumer{sources="d8b52e33-6a5d-4c6e-8f6a-1b2c3d4e5f60"}
This summary re-reads the task-board datasource above.
:::

:::fragment{localId="rota-fragment" name="Inspection rota"}
Other macros reference this block by its fragment name.
:::

## Attachments

![Mite counts by week](https://static.example.org/mite-counts.png "Varroa counts, spring 2026")

::media[hive-inspection-sheet.pdf]{#b5773183-5f9a-481f-b1b8-8fe286bba8e9}

:::media[hive stand sketch]{#0f4b9a2c-3d5e-4f60-8a71-92b3c4d5e6f7 height="480" layout="center" width="640"}
Sketch of the **vertical** stand — drawn by Sam.
:::

Field kit: :media{#7c1e0d2a-4b3f-45e8-9a2b-6c5d4e3f2a1b collection}

:::breakout{wide}
> This wide quote breaks out of the content column.
:::
