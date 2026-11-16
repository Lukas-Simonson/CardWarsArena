// The game log as plain text, to copy or save. Pure: no DOM, no network.
//
//   Card Wars Arena game log
//   Room: B69K · Turn 7
//   Players: Lukas (Hero: Finn), Bot (Hero: Jake)
//
//   Player draws a card
//   ── Bot's turn ──
//   Bot plays Cool Dog
//
// The panel shows the newest lines first with markers around them; this is the whole log, oldest first, markers
// taken off, and a turn's line set apart by a blank line so it reads as a heading.
import { APP_NAME } from "./brand.js";
import { t } from "./i18n.js";

const MARK = /^(──|\*\*\*)\s*|\s*(──|\*\*\*)$/g;

/** A log line without the markers the panel turns into styling. */
const plain = (line) => String(line).replace(MARK, "").trim();

/**
 * @param {{log:string[], view:object|null, names:string[], session:{room:string}|null}} state
 * @param {object} cards  the catalog by id
 * @param {number} [limit]  the most lines the client keeps; a log that long is said to have lost its start
 */
export function gameLogText(state, cards, limit = Infinity) {
  const v = state.view;
  const head = [t("gamelog.title", { app: APP_NAME })];
  const where = [state.session?.room && t("gamelog.room", { room: state.session.room }), v?.turn != null && t("game.turn", { turn: v.turn })].filter(Boolean);
  if (where.length) head.push(where.join(t("detail.sep")));
  if (v) {
    const players = [0, 1].map((seat) => {
      const name = state.names?.[seat] || t(seat === v.you ? "player.you" : "player.opponent");
      const hero = v.players?.[seat]?.hero;
      const heroName = hero && (cards?.[hero.card]?.name ?? hero.card);
      return heroName ? t("gamelog.player_hero", { name, hero: heroName }) : name;
    });
    head.push(t("gamelog.players", { players }));
  }

  const body = [];
  if (state.log.length >= limit) body.push(t("gamelog.truncated"));
  for (const raw of state.log) {
    const line = plain(raw), mark = String(raw).trim().slice(0, 2);
    if (!line) continue;
    const set = mark === "──" || mark === "**"; // a turn or the end of the game stands apart
    if (set && body.length) body.push("");
    body.push(mark === "──" ? `── ${line} ──` : line);
  }
  if (!state.log.length) body.push(t("gamelog.empty"));
  return [...head, "", ...body].join("\n") + "\n";
}

/** A file name for the log: "card-wars-arena-b69k-log.txt". */
export const logFileName = (room) => `card-wars-arena-${String(room ?? "").toLowerCase().replace(/[^a-z0-9]+/g, "") || "game"}-log.txt`;
