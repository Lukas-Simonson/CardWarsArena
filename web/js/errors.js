// What the player reads when the server says no. The server sends no prose to translate: it sends what went wrong as
// a code (the kind of failure), a reason (exactly what, such as the rule of the game that was broken), and the values
// the message needs (cards by id). The client words it in the player's language:
//
//   reason.<reason>   the most exact message, from `reason` (a rule, a deck problem, why a room closed)
//   error.<code>      the kind of failure, from `code`
//
// The English `message` the server adds is only for a code or reason this client has no message for, so nothing is
// ever blank. Params named `card` are card ids: the message gets the card's name.
import { t, has } from "./i18n.js";

/** The params of a problem with cards turned into their names. */
function named(params, cards) {
  const out = { ...params };
  for (const [k, v] of Object.entries(out)) {
    if ((k === "card" || k.endsWith("_card")) && typeof v === "string") out[k] = cards?.[v]?.name ?? v;
  }
  return out;
}

/** A problem { code, params, text } (a deck problem, say) as a sentence. */
export function problemText(p, cards) {
  const key = `reason.${p.code}`;
  return has(key) ? t(key, named(p.params, cards)) : p.text ?? p.code;
}

/** What stops a deck being played, as sentences. */
export const problemTexts = (problems, cards) => (problems ?? []).map((p) => (typeof p === "string" ? p : problemText(p, cards)));

/**
 * @param {{code:string, reason?:string, params?:object, message?:string}|null} err  an `error` message, or an API error
 * @param {object} [cards]  the catalog by id, to name the cards the error mentions
 */
export function errorText(err, cards) {
  if (!err) return "";
  const params = named(err.params, cards);
  if (err.reason && has(`reason.${err.reason}`)) return t(`reason.${err.reason}`, { ...params, message: err.message ?? "" });
  if (err.code === "deck_not_playable" && Array.isArray(err.params?.problems)) {
    return t("error.deck_not_playable_list", { ...params, problems: problemTexts(err.params.problems, cards).join(t("targets.sep")) });
  }
  if (has(`error.${err.code}`)) return t(`error.${err.code}`, { ...params, message: err.message ?? "" });
  return err.message ? `${err.code}: ${err.message}` : String(err.code ?? "");
}

/** An error the deck API answered: { error: "<English>", code, params }, or nothing when it said nothing useful. */
export const apiError = (data) => (data?.code ? { code: data.code, params: data.params, message: data.error } : null);

/** Why a room ended: room_closed's reason code, with its English message as the fallback. */
export function closedText(reason, message) {
  return has(`closed.${reason}`) ? t("closed.text", { why: t(`closed.${reason}`) }) : t("error.room_closed", { reason: message ?? reason ?? "" });
}
