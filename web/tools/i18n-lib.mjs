// Checks on the message catalogues and on the code that uses them. Used by `npm run i18n` and by the tests, so a
// missing, unused or mismatched key fails CI. No dependencies.
import { readFile, readdir } from "node:fs/promises";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { parse } from "../js/i18n.js";

export const WEB = fileURLToPath(new URL("..", import.meta.url));
export const slug = (label) => String(label).toLowerCase().replace(/[^a-z0-9]+/g, "_").replace(/^_|_$/g, "");

/** The names of the arguments a message uses, with the kind of each ("plural", "select", "list", "number" or ""). */
export function placeholders(message) {
  const out = new Map();
  const walk = (nodes) => {
    for (const n of nodes) {
      if (typeof n === "string" || n.hash) continue;
      out.set(n.arg, n.type ?? "");
      for (const branch of Object.values(n.cases ?? {})) walk(branch);
    }
  };
  walk(parse(message));
  return out;
}

export async function readCatalogs(dir = join(WEB, "locales")) {
  const index = JSON.parse(await readFile(join(dir, "index.json"), "utf8"));
  const catalogs = {};
  for (const { tag } of index) catalogs[tag] = JSON.parse(await readFile(join(dir, `${tag}.json`), "utf8"));
  return { index, catalogs };
}

/** What a locale lacks or gets wrong compared with English: { missing, extra, mismatched, broken }. */
export function compareCatalog(en, other) {
  const missing = Object.keys(en).filter((k) => !(k in other));
  const extra = Object.keys(other).filter((k) => !(k in en));
  const mismatched = [], broken = [];
  for (const k of Object.keys(en)) {
    if (!(k in other)) continue;
    let a, b;
    try { a = placeholders(en[k]); } catch (e) { broken.push(`en ${k}: ${e.message}`); continue; }
    try { b = placeholders(other[k]); } catch (e) { broken.push(`${k}: ${e.message}`); continue; }
    const names = (m) => [...m.keys()].sort().join(",");
    if (names(a) !== names(b)) mismatched.push(`${k}: English uses {${names(a)}}, this uses {${names(b)}}`);
  }
  return { missing, extra, mismatched, broken };
}

async function files(dir, ext) {
  const out = [];
  for (const e of await readdir(dir, { withFileTypes: true })) {
    if (e.isDirectory()) out.push(...(await files(join(dir, e.name), ext)));
    else if (ext.some((x) => e.name.endsWith(x))) out.push(join(dir, e.name));
  }
  return out;
}

const KEY = /^[a-z][a-z0-9_]*(\.[a-z0-9_]+)+$/;
const stripComments = (src) => src.replace(/\/\*[\s\S]*?\*\//g, (c) => c.replace(/[^\n]/g, "")).replace(/(^|[^:"'`\\])\/\/.*$/gm, "$1");

/** The source files the client is made of. */
export async function sourceFiles() {
  return [...(await files(join(WEB, "js"), [".js"])), join(WEB, "index.html")];
}

/** What the code refers to: keys named in t("…") / has("…") calls and data-i18n attributes, and dynamic key prefixes. */
export async function usage() {
  const keys = new Set(), prefixes = new Set(), literals = new Set();
  for (const file of await sourceFiles()) {
    const raw = await readFile(file, "utf8");
    if (file.endsWith(".html")) {
      for (const m of raw.matchAll(/data-i18n="([^"]+)"/g)) keys.add(m[1]);
      for (const m of raw.matchAll(/data-i18n-attr="([^"]+)"/g)) for (const pair of m[1].split(";")) keys.add(pair.split(":")[1]?.trim());
      continue;
    }
    if (file.endsWith("i18n.js")) continue; // the machinery's own examples
    const src = stripComments(raw);
    for (const m of src.matchAll(/(?<![.\w])(?:t|has)\(\s*"([^"]+)"/g)) keys.add(m[1]);
    for (const m of src.matchAll(/(?<![.\w])(?:t|has)\(\s*`([a-z0-9_.]*)\$\{/g)) prefixes.add(m[1]);
    for (const m of src.matchAll(/"([a-z][a-z0-9_]*(?:\.[a-z0-9_]+)+)"/g)) if (KEY.test(m[1])) literals.add(m[1]);
    for (const m of src.matchAll(/`([a-z][a-z0-9_.]*[._])\$\{/g)) prefixes.add(m[1]);
  }
  return { keys: [...keys].filter(Boolean), prefixes: [...prefixes], literals: [...literals] };
}

/** Keys the code asks for that English lacks, and English keys nothing refers to. */
export async function keyUse(en) {
  const { keys, prefixes, literals } = await usage();
  const used = new Set([...keys, ...literals, ...[...keys, ...literals].map((k) => `${k}_inline`)]); // "x" also has an "x_inline" form (the prompts)
  const undefinedKeys = keys.filter((k) => !(k in en)).sort();
  const unused = Object.keys(en).filter((k) => !used.has(k) && !prefixes.some((p) => k.startsWith(p)) && !k.startsWith("label.")).sort();
  return { undefinedKeys, unused };
}

/** The target labels written in the card data (data/effects), which the client may translate as label.<slug>. */
export async function effectLabels(dir = join(WEB, "..", "server", "data", "effects")) {
  const labels = new Set();
  const walk = (o) => {
    if (Array.isArray(o)) o.forEach(walk);
    else if (o && typeof o === "object") for (const [k, v] of Object.entries(o)) ((k === "label" || k === "skip_label") && typeof v === "string" ? labels.add(v) : walk(v));
  };
  for (const f of await files(dir, [".json"])) walk(JSON.parse(await readFile(f, "utf8")));
  return [...labels].sort();
}

/** Labels in the card data without a label.<slug> message, and label messages no card uses. */
export async function labelGaps(en) {
  const labels = await effectLabels();
  const want = new Map(labels.map((l) => [`label.${slug(l)}`, l]));
  const missing = [...want].filter(([k]) => !(k in en)).map(([k, l]) => ({ key: k, label: l }));
  const stale = Object.keys(en).filter((k) => k.startsWith("label.") && !want.has(k));
  return { missing, stale };
}

// Words that look like English prose but are not: they are not shown to the player.
const SKIP = /^(btn|well|steel|parch|felt|tile|strip|modal|chip|card|hbtn|fx|pick|go|sel|off|foe|mine|theirs|ink|sr|h|row|btns|acts|orb|lbl|tag|nm|cap|ok|bad|good|warn|info|small|big|muted|empty|dest|lane|hand|pending|hero|creature|building|landscape|discard|thaw|floop|attack|activate|choose|ready|flooped|fought|open|closed|connecting|reconnecting)\b/;

/**
 * String literals in a piece of code that look like text for the player: [{ line, text }]. A ratchet, not a proof.
 * A line may say `// i18n-ignore` when a word there is not for the player (a file format's keyword).
 */
export function findHardcoded(code) {
  const out = [];
  const raw = code.split("\n");
  stripComments(code).split("\n").forEach((line, i) => {
    if (raw[i].includes("i18n-ignore") || /\.key\s*===/.test(line)) return;
    for (const m of line.matchAll(/"((?:[^"\\]|\\.)*)"|`((?:[^`\\]|\\.)*)`/g)) {
      const text = (m[1] ?? m[2]).replace(/\$\{[^}]*\}/g, "").trim();
      if (!/[A-Za-z]{3,}/.test(text) || KEY.test(text) || SKIP.test(text) || /^[a-z0-9_:#.\- /[\]=>"'(),%*+~|]+$/.test(text) || /^(https?:|data:|\/|<|\.|#)/.test(text)) continue;
      if (/^[A-Z][a-z]+(?: [A-Za-z']+)+/.test(text) || /^[A-Z][a-z]{3,}$/.test(text) || /[a-z]{2,} [a-z]{2,} [a-z]{2,}/.test(text)) out.push({ line: i + 1, text });
    }
  });
  return out;
}

/** findHardcoded over every client module: [{ file, line, text }]. */
export async function hardcoded() {
  const out = [];
  for (const file of await files(join(WEB, "js"), [".js"])) {
    if (file.endsWith("i18n.js") || file.endsWith("brand.js")) continue;
    for (const f of findHardcoded(await readFile(file, "utf8"))) out.push({ file: file.slice(WEB.length), ...f });
  }
  return out;
}
