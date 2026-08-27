// Node smoke test for the js/wasm build.
//
// This is the ONLY thing that proves main.go works. Everything in api.go
// is table-tested on the host, but main.go is behind `//go:build js &&
// wasm`, so `go test ./...` never compiles it — the js.FuncOf glue, the
// argument coercion, and the globalThis.adfast registration are invisible
// to the Go test suite by construction.
//
//   GOOS=js GOARCH=wasm go build -o adfast.wasm .
//   node smoke.mjs [path/to/adfast.wasm]
//
// wasm_exec.js is vendored from the Go toolchain that must also build the
// binary; see README.md for the pinned version.

import { readFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const wasmPath = process.argv[2] ?? join(here, "adfast.wasm");

let failures = 0;

function check(label, ok, detail) {
  if (ok) {
    console.log(`  ok   ${label}`);
  } else {
    failures += 1;
    console.error(`  FAIL ${label}${detail === undefined ? "" : `: ${detail}`}`);
  }
}

// value unwraps the {ok, value} result shape every export returns, and
// fails the run rather than throwing so one broken export does not hide
// the others.
function value(label, res) {
  if (res === undefined || res === null || typeof res !== "object") {
    check(label, false, `result is not an object: ${JSON.stringify(res)}`);
    return undefined;
  }
  if (res.ok !== true) {
    check(label, false, `ok=false, error=${res.error}`);
    return undefined;
  }
  return res.value;
}

await import(join(here, "wasm_exec.js"));

const go = new globalThis.Go();
const bootStart = performance.now();
const { instance } = await WebAssembly.instantiate(
  await readFile(wasmPath),
  go.importObject,
);
const instantiated = performance.now();
go.run(instance); // never resolves: main() parks on `select {}`
const booted = performance.now();

check("globalThis.adfast is installed", typeof globalThis.adfast === "object");
for (const name of ["scanSpans", "codeSpans", "catalog", "toADF", "toMarkdown", "diagnostics"]) {
  check(`adfast.${name} is a function`, typeof globalThis.adfast?.[name] === "function");
}
if (failures > 0) {
  console.error("the module did not register its exports; aborting");
  process.exit(1);
}

const { scanSpans, codeSpans, catalog, toADF, toMarkdown, diagnostics } = globalThis.adfast;

// --- scanSpans ------------------------------------------------------------
// The accented letter and the emoji are the point: the offsets must index
// the string the way JavaScript does, or `md.slice` returns garbage.
{
  const md = "Héllo :status[Ready]{color=\"green\"} 🐝 :date[2026-04-12] end\n";
  const spans = JSON.parse(value("scanSpans returns JSON", scanSpans(md)) ?? "[]");
  check("scanSpans found both directives", spans.length === 2, JSON.stringify(spans));
  check(
    "scanSpans offsets are UTF-16 code units",
    md.slice(spans[0]?.start, spans[0]?.end) === ":status[Ready]{color=\"green\"}",
    md.slice(spans[0]?.start, spans[0]?.end),
  );
  check(
    "scanSpans offsets survive a surrogate pair",
    md.slice(spans[1]?.start, spans[1]?.end) === ":date[2026-04-12]",
    md.slice(spans[1]?.start, spans[1]?.end),
  );
  check("scanSpans reports the level", spans[0]?.level === 1, spans[0]?.level);
  check("scanSpans reports parsed attrs", spans[0]?.attrs?.color === "green", JSON.stringify(spans[0]?.attrs));
}
{
  const spans = JSON.parse(value("scanSpans on a container", scanSpans(":::info\nbody\n:::\n")) ?? "[]");
  check("container spans are level 3", spans[0]?.level === 3, JSON.stringify(spans));
  check("container span reaches the closing fence", spans[0]?.end === 16, spans[0]?.end);
}
{
  // An unclosed container is a real input, not a theoretical one: it is
  // what the buffer looks like mid-keystroke.
  const spans = JSON.parse(value("scanSpans on an unclosed container", scanSpans(":::info\nstill typing\n")) ?? "[]");
  check("unclosed container runs to the end", spans[0]?.end === 21, JSON.stringify(spans));
}

// --- codeSpans -----------------------------------------------------------
// The same UTF-16 contract as scanSpans, on the code-block view, plus the
// two shapes a line scanner gets wrong.
{
  const md = "Héllo 🐝\n\n```go\nx := 1\n```\n\nafter\n";
  const spans = JSON.parse(value("codeSpans returns JSON", codeSpans(md)) ?? "[]");
  check("codeSpans found the fenced block", spans.length === 1, JSON.stringify(spans));
  check(
    "codeSpans offsets are UTF-16 code units",
    md.slice(spans[0]?.start, spans[0]?.end) === "```go\nx := 1\n```\n",
    md.slice(spans[0]?.start, spans[0]?.end),
  );
}
{
  const md = "```js\ncode\n``` js\nmore\n";
  const spans = JSON.parse(value("codeSpans on a closer with an info string", codeSpans(md)) ?? "[]");
  check("a closer carrying an info string does not close", md.slice(spans[0]?.start, spans[0]?.end) === md, JSON.stringify(spans));
}
{
  const spans = JSON.parse(value("codeSpans on prose", codeSpans("just `inline` prose\n")) ?? "null");
  check("codeSpans returns an array when there is no code", Array.isArray(spans) && spans.length === 0, JSON.stringify(spans));
}

// --- catalog --------------------------------------------------------------
// The join a plugin actually performs: take the names scanSpans reports and
// look each one up by (name, level).
{
  const entries = JSON.parse(value("catalog returns JSON", catalog()) ?? "[]");
  check("catalog is a non-empty array", Array.isArray(entries) && entries.length > 0, entries.length);
  const byKey = new Map(entries.map((e) => [`${e.name}/${e.level}`, e]));
  check("catalog binds a panel name to its kind", byKey.get("info/3")?.kind === "panel", JSON.stringify(byKey.get("info/3")));
  check(
    "catalog keeps a per-level kind",
    byKey.get("media/1")?.kind === "mediaInline" && byKey.get("media/2")?.kind === "media",
    JSON.stringify([byKey.get("media/1"), byKey.get("media/2")]),
  );
  check("catalog reports decodedByCore", byKey.get("colwidths/2")?.decodedByCore === true, JSON.stringify(byKey.get("colwidths/2")));
  const md = ":::info\nSee :status[Ready]{color=\"green\"}.\n:::\n";
  const spans = JSON.parse(value("scanSpans for the catalog join", scanSpans(md)) ?? "[]");
  const unknown = spans.filter((s) => !byKey.has(`${s.name}/${s.level}`));
  check("every scanned span resolves in the catalog", unknown.length === 0, JSON.stringify(unknown));
}

// --- toADF ----------------------------------------------------------------
{
  const md = ":::info\nFeed :status[Done]{color=\"green\"} on :date[2026-04-12].\n:::\n";
  const doc = JSON.parse(value("toADF with no opts", toADF(md)) ?? "{}");
  check("toADF produced a doc", doc.type === "doc", JSON.stringify(doc).slice(0, 120));
  const panel = doc.content?.[0];
  check("toADF produced a panel", panel?.type === "panel" && panel?.attrs?.panelType === "info", JSON.stringify(panel));
  const inlines = panel?.content?.[0]?.content ?? [];
  check("toADF produced a status", inlines.some((n) => n.type === "status" && n.attrs?.color === "green"));
  check("toADF produced a date", inlines.some((n) => n.type === "date"));
}
{
  // opts crosses the boundary as a live JS object, not a string.
  const doc = JSON.parse(value("toADF with a jira opts object", toADF("See BEE-42.\n", {
    product: "jira",
    baseUrl: "https://hive.example.org",
  })) ?? "{}");
  const inlines = doc.content?.[0]?.content ?? [];
  check("the jira bundle expanded a bare issue key", inlines.some((n) => n.type === "inlineCard"), JSON.stringify(inlines));
}
{
  const res = toADF("hello\n", { product: "bitbucket" });
  check("an unknown product is a value, not a throw", res?.ok === false, JSON.stringify(res));
  check("the error explains itself", /unknown product/.test(res?.error ?? ""), res?.error);
}

// --- toMarkdown -----------------------------------------------------------
{
  const doc = {
    type: "doc",
    version: 1,
    content: [{
      type: "panel",
      attrs: { panelType: "note" },
      content: [{ type: "paragraph", content: [{ type: "text", text: "Mind the bees 🐝" }] }],
    }],
  };
  check(
    "toMarkdown accepts a live ADF object",
    value("toMarkdown(object)", toMarkdown(doc)) === ":::note\nMind the bees 🐝\n:::\n",
    JSON.stringify(value("toMarkdown(object)", toMarkdown(doc))),
  );
  check(
    "toMarkdown accepts an ADF JSON string",
    value("toMarkdown(string)", toMarkdown(JSON.stringify(doc))) === ":::note\nMind the bees 🐝\n:::\n",
  );
}
{
  const res = toMarkdown("{not json");
  check("malformed ADF is a value, not a throw", res?.ok === false, JSON.stringify(res));
}

// --- diagnostics ----------------------------------------------------------
{
  const clean = JSON.parse(value("diagnostics on a clean document", diagnostics("just prose\n")) ?? "null");
  check("clean diagnostics is an empty array", Array.isArray(clean) && clean.length === 0, JSON.stringify(clean));
  const noisy = JSON.parse(
    value("diagnostics with a product", diagnostics("```klingon\nqapla'\n```\n", { product: "confluence" })) ?? "null",
  );
  check("diagnostics reports a code", noisy?.[0]?.code === "unsupported-code-language", JSON.stringify(noisy));
  check("diagnostics reports a message", typeof noisy?.[0]?.message === "string" && noisy[0].message.length > 0);
  check("diagnostics invents no offsets", noisy?.[0]?.start === undefined, JSON.stringify(noisy?.[0]));
}

// --- timings --------------------------------------------------------------
{
  const unit = ":::info[Hive check]\n\nFeed :status[Done]{color=\"green\"} on :date[2026-04-12], per :mention[@Ada]{#712020:ada} 🐝.\n\n| a | b |\n| - | - |\n| 1 | 2 |\n\n:::\n";
  const timings = [];
  for (const [label, md] of [["small", unit], ["large", unit.repeat(8)]]) {
    for (let i = 0; i < 100; i += 1) toADF(md); // warm up
    const start = performance.now();
    const runs = 300;
    for (let i = 0; i < runs; i += 1) toADF(md);
    timings.push(`toADF ${label} (${md.length} chars) ${((performance.now() - start) / runs).toFixed(3)} ms/doc`);
  }
  console.log(
    `\ninstantiate ${(instantiated - bootStart).toFixed(1)} ms, ` +
      `go.run ${(booted - instantiated).toFixed(1)} ms\n${timings.join("\n")}`,
  );
}

console.log(failures === 0 ? "\nwasm smoke test passed" : `\nwasm smoke test FAILED (${failures})`);
process.exit(failures === 0 ? 0 : 1);
