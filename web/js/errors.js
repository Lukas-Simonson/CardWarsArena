// What the player reads when the server says no. The server sends a stable code (and an English message with the
// detail); the client turns the code into a message in the active language. A code it has no message for shows the
// server's own text, so nothing is ever blank.
import { t, has } from "./i18n.js";

/** @param {{code:string, message?:string}|null} err */
export function errorText(err) {
  if (!err) return "";
  const key = `error.${err.code}`;
  if (has(key)) return t(key, { message: err.message ?? "" });
  return err.message ? `${err.code}: ${err.message}` : String(err.code);
}

/** Why a room ended, from room_closed's message (English prose from the server for now). */
export function closedText(reason) {
  return t("error.room_closed", { reason: reason ?? "" });
}
