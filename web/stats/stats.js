// The /stats page: the shared totals from GET /api/stats as tiles and bars. It is not linked from the game, and it is
// for the person who runs the server, so it is in English only and has no dependencies.
import { h } from "../js/dom.js";
import { summarize, topCards, pct } from "./model.mjs";

const REFRESH_MS = 30_000;
const TOP = 15;
const root = document.getElementById("root"), meta = document.getElementById("meta");
let names = {}, showAll = false, last = null;

const n = (v) => (v ?? 0).toLocaleString();
const words = (key) => key.replace(/_/g, " ");

/** A row of label, bar (width is part of `max`) and number. */
function bar(label, value, max, { cls = "", note = "" } = {}) {
  return h("div", { class: "row" },
    h("span", { class: "label", title: label }, label),
    h("div", { class: "track", role: "img", "aria-label": `${label}: ${n(value)}` }, h("div", { class: `fill ${cls}`, style: `width:${max > 0 ? (value / max) * 100 : 0}%` })),
    h("span", { class: "num" }, note || n(value)));
}
const stat = (value, label) => h("div", { class: "well stat" }, h("b", {}, value), h("span", {}, label));
const empty = (text) => h("p", { class: "empty" }, text);

/** Two counts (bot and person) as one stacked bar. */
function stacked(title, split) {
  const total = split.bot + split.person;
  return h("div", {},
    h("div", { class: "row" }, h("span", { class: "label" }, title),
      h("div", { class: "track", role: "img", "aria-label": `${title}: ${n(split.bot)} against the bot, ${n(split.person)} against a person` },
        h("div", { class: "fill", style: `width:${total ? (split.bot / total) * 100 : 0}%` }), h("div", { class: "fill b", style: `width:${total ? (split.person / total) * 100 : 0}%` })),
      h("span", { class: "num" }, n(split.total))));
}

function draw(report) {
  const s = summarize(report);
  const section = (title, ...kids) => h("section", { class: "parch" }, h("h2", {}, title), ...kids);
  const maxOf = (rows) => Math.max(0, ...rows.map((r) => r[1] ?? r.n ?? 0));

  const cards = topCards(report.cards.played, names, showAll ? Infinity : TOP);
  const cardMax = cards[0]?.n ?? 0;
  const typeMax = maxOf(s.byType), actMax = maxOf(s.actions), botMax = maxOf(s.byBot);

  root.replaceChildren(
    h("div", { class: "tiles" },
      stat(n(s.started), "games started"), stat(n(s.completed), "games completed"), stat(n(s.abandoned), "abandoned"),
      stat(s.completion == null ? "–" : `${s.completion}%`, "reach a winner"), stat(n(s.cardsPlayed), "cards played"),
      stat(n(s.damage), "damage dealt"), stat(n(s.destroyed), "Creatures destroyed")),
    h("div", { class: "grid" },
      section("Games",
        stacked("Started", s.kinds.started), stacked("Completed", s.kinds.completed),
        h("div", { class: "split" }, h("span", {}, h("i"), "against the bot"), h("span", {}, h("i", { class: "b" }), "against a person"))),
      section("Against the bot",
        s.vsBot.games === 0 ? empty("No finished games against the bot yet.") : [
          bar("Player wins", s.vsBot.player, s.vsBot.games, { cls: "good", note: `${n(s.vsBot.player)} (${s.vsBot.playerRate}%)` }),
          bar("Bot wins", s.vsBot.bot, s.vsBot.games, { cls: "b", note: `${n(s.vsBot.bot)} (${100 - s.vsBot.playerRate}%)` }),
          ...s.byBot.map(([bot, v]) => bar(`  won by ${bot}`, v, botMax))])),
    h("div", { class: "grid" },
      section("Cards played by type", s.byType.some((r) => r[1]) ? s.byType.map(([k, v]) => bar(k, v, typeMax, { note: `${n(v)} (${pct(v, s.cardsPlayed) ?? 0}%)` })) : empty("Nothing played yet.")),
      section("Actions", s.actions.map(([k, v]) => bar(words(k), v, actMax)))),
    section("Decks (by the starter deck's Hero)",
      s.decks.length === 0 ? empty("No decks played yet.") : s.decks.map((d) =>
        bar(d.hero, d.won, d.started, { cls: "good", note: `${n(d.won)} / ${n(d.started)}${d.rate == null ? "" : ` (${d.rate}%)`}` }))),
    section(showAll ? "All cards played" : `Most played cards (top ${TOP})`,
      cards.length === 0 ? empty("No cards played yet.") : cards.map((c) => bar(c.name, c.n, cardMax)),
      Object.keys(report.cards.played).length > TOP && h("button", { type: "button", class: "btn-steel small", onclick: () => { showAll = !showAll; draw(last); } }, showAll ? `Show the top ${TOP}` : `Show all ${Object.keys(report.cards.played).length}`)));
}

async function load() {
  try {
    const res = await fetch("../api/stats", { cache: "no-store" });
    if (res.status === 404) throw new Error("Statistics are turned off on this server.");
    if (!res.ok) throw new Error(`The server answered ${res.status}.`);
    last = await res.json();
    if (last.v !== 2) throw new Error(`This page reads version 2 of the statistics; the server sends version ${last.v}.`);
    draw(last);
    meta.textContent = `Updated ${new Date().toLocaleTimeString()}. ${last.kept ? "Totals are kept across restarts." : "Totals start again at zero on a restart (no database)."} Not linked from the game.`;
  } catch (err) {
    if (!last) root.replaceChildren(h("section", { class: "parch error" }, h("h2", {}, "No statistics"), h("p", {}, err.message)));
    meta.textContent = last ? `Could not refresh (${err.message}); showing the last numbers.` : "";
  }
}

// Card names come from the catalogue; without it the page shows ids.
fetch("../api/cards").then((r) => (r.ok ? r.json() : [])).then((list) => { names = Object.fromEntries(list.map((c) => [c.id, c.name])); if (last) draw(last); }).catch(() => {});
await load();
setInterval(load, REFRESH_MS);
