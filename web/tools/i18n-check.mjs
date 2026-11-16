// node tools/i18n-check.mjs [--add-labels] [--strings]
//   (no flag)  every locale has English's keys and placeholders; the code uses only keys English has, and English has
//              no key nothing uses; every target label in the card data has a label.<slug> message
//   --add-labels  add a label.<slug> message to en.json for each label in the card data that has none
//   --strings  also list string literals in the code that look like hard-coded text (for finding what is left)
import { readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { readCatalogs, compareCatalog, keyUse, labelGaps, hardcoded, WEB } from "./i18n-lib.mjs";

const { catalogs } = await readCatalogs();
const en = catalogs.en;

if (process.argv.includes("--add-labels")) { // add a label.<slug> message, in English, for each label in the card data
  const { missing } = await labelGaps(en);
  const file = join(WEB, "locales", "en.json");
  const next = { ...en, ...Object.fromEntries(missing.map((m) => [m.key, m.label])) };
  await writeFile(file, JSON.stringify(Object.fromEntries(Object.entries(next).sort(([a], [b]) => (a < b ? -1 : 1))), null, 1) + "\n");
  console.log(`added ${missing.length} label message(s)`);
  process.exit(0);
}
let bad = 0;
const report = (title, list) => { if (list.length) { bad += list.length; console.error(`\n${title} (${list.length}):`); for (const x of list) console.error("  " + x); } };

for (const [tag, cat] of Object.entries(catalogs)) {
  if (tag === "en") continue;
  const c = compareCatalog(en, cat);
  report(`${tag}: keys missing`, c.missing); report(`${tag}: keys English does not have`, c.extra);
  report(`${tag}: placeholders differ`, c.mismatched); report(`${tag}: broken messages`, c.broken);
}
const use = await keyUse(en);
report("Keys used in the code but missing from en.json", use.undefinedKeys);
report("Keys in en.json that nothing uses", use.unused);
const gaps = await labelGaps(en);
report("Card-data labels without a message (add label.<slug> to en.json)", gaps.missing.map((m) => `${m.key}  ${JSON.stringify(m.label)}`));
report("label.* messages no card uses", gaps.stale);
if (process.argv.includes("--strings")) {
  const found = await hardcoded();
  console.error(`\nText that may be hard-coded (${found.length}):`);
  for (const f of found) console.error(`  ${f.file}:${f.line}  ${JSON.stringify(f.text)}`);
}
if (bad) { console.error(`\n${bad} problem(s).`); process.exit(1); }
console.log(`i18n ok: ${Object.keys(catalogs).length} locale(s), ${Object.keys(en).length} messages.`);
