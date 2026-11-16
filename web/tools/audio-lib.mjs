// Checks on the sound files: every one is credited with a source and a licence, both formats exist for each, the cues
// use only files that are there and every file is used, and the total stays small (the web image bundles them).
import { readFile, readdir, stat } from "node:fs/promises";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { CUES, allFiles } from "../js/sounds.js";

export const AUDIO = fileURLToPath(new URL("../audio", import.meta.url));
export const BUDGET = 2 * 1024 * 1024; // bytes, both formats together

/** The credited packs, read from the table in CREDITS.md: [{ prefix, pack, source, licence, text }]. */
export async function credited() {
  const md = await readFile(join(AUDIO, "CREDITS.md"), "utf8");
  return md.split("\n").filter((l) => /^\|/.test(l) && !/^\|[-| ]+\|$/.test(l) && !/^\| Prefix/.test(l)).map((l) => {
    const [prefix, pack, author, source, licence, text] = l.split("|").slice(1, -1).map((c) => c.trim());
    return { prefix, pack, author, source, licence, text };
  });
}

/** What is wrong with the audio folder: a list of sentences, empty when all is well. */
export async function problems() {
  const out = [];
  const packs = await credited();
  const names = await readdir(AUDIO);
  const sounds = names.filter((n) => /\.(ogg|m4a|mp3|wav|opus|webm)$/i.test(n));

  for (const p of packs) {
    if (!/^https:\/\//.test(p.source ?? "")) out.push(`${p.prefix}: no source address in CREDITS.md`);
    if (!p.licence) out.push(`${p.prefix}: no licence in CREDITS.md`);
    if (!/^(CC0|CC-BY|CC BY)/i.test(p.licence ?? "")) out.push(`${p.prefix}: licence "${p.licence}" is not CC0 or CC-BY (no non-commercial or unknown licences)`);
    try { await stat(join(AUDIO, p.text)); } catch { out.push(`${p.prefix}: the licence text ${p.text} is missing`); }
  }
  const prefixes = new Set(packs.map((p) => p.prefix));
  let total = 0;
  for (const n of sounds) {
    total += (await stat(join(AUDIO, n))).size;
    if (!/\.(ogg|m4a)$/.test(n)) out.push(`${n}: only .ogg and .m4a are used`);
    const prefix = n.split("-")[0];
    if (!prefixes.has(prefix)) out.push(`${n}: not from a credited pack (add it to CREDITS.md)`);
    const base = n.replace(/\.(ogg|m4a)$/, "");
    for (const ext of ["ogg", "m4a"]) if (!names.includes(`${base}.${ext}`)) out.push(`${base}: missing its .${ext}`);
  }
  if (total > BUDGET) out.push(`the sounds are ${(total / 1024).toFixed(0)} KB, over the ${BUDGET / 1024} KB budget`);

  const used = new Set(allFiles());
  for (const f of used) if (!names.includes(`${f}.ogg`)) out.push(`a cue uses ${f}, which is not in audio/`);
  for (const n of sounds) if (!used.has(n.replace(/\.(ogg|m4a)$/, ""))) out.push(`${n} is used by no cue (delete it)`);
  for (const [cue, c] of Object.entries(CUES)) if (!c.files.length) out.push(`cue ${cue} has no files`);
  return [...new Set(out)];
}
