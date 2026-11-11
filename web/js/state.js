// The client's model of the server conversation. Pure: reduce(state, msg)
// returns the next state and never touches the DOM or the network.
import { describeEvent } from "./describe.js";

export const LOG_LIMIT = 2000; // lines kept for the panel and for copying the game out

export function initialState() {
  return {
    status: "closed",      // connection: closed | connecting | open | reconnecting
    session: null,         // { room, token, seat, vsBot } once seated
    waiting: false,        // room created, no opponent yet
    names: [],             // seat display names
    avatars: [],           // seat picture addresses, "" for a guest or the bot
    opponent: { connected: null, name: "" },
    view: null,            // latest view from the server
    seq: 0,                // seq of that view
    known: {},             // uid -> card id, for naming cards in events
    log: [],               // human-readable event lines
    error: "",             // last error shown to the player
    closedReason: "",      // why the last room ended
  };
}

/** Remember the card id of every instance visible in a view. */
export function learnCards(known, view) {
  const out = { ...known };
  const add = (c) => c && (out[c.uid] = c.card);
  for (const p of view.players) {
    add(p.hero);
    (p.hand ?? []).forEach(add);
    p.discard.forEach(add);
    for (const l of p.lanes) {
      add(l.creature);
      add(l.building);
      (l.extra ?? []).forEach(add);
    }
  }
  add(view.pending);
  (view.choice?.revealed ?? []).forEach(add);
  return out;
}

export function setStatus(state, status) {
  return { ...state, status };
}

/** Clear the current room (used when leaving). */
export function leave(state) {
  return { ...state, session: null, view: null, seq: 0, waiting: false, log: [], error: "", opponent: { connected: null, name: "" } };
}

/**
 * Apply one server message. `cards` is the catalog (id -> card) used to name
 * cards in the log.
 */
export function reduce(state, msg, cards = {}) {
  switch (msg.type) {
    case "joined": {
      const sameRoom = state.session?.room === msg.room;
      return {
        ...(sameRoom ? state : leave(state)),
        session: { room: msg.room, token: msg.token, seat: msg.seat, vsBot: !!msg.vs_bot },
        names: msg.names ?? state.names,
        avatars: msg.avatars ?? state.avatars,
        error: "",
        closedReason: "",
      };
    }
    case "waiting":
      return { ...state, waiting: true };
    case "opponent":
      return { ...state, opponent: { connected: msg.connected, name: msg.name ?? "" } };
    case "update": {
      if (msg.seq < state.seq) return state; // an older update arriving late
      const names = msg.names?.length ? msg.names : state.names;
      const known = learnCards(state.known, msg.view);
      for (const e of msg.events ?? []) if (e.card && e.card_id) known[e.card] = e.card_id;
      const ctx = { view: msg.view, cards, names, known };
      // The view is the position after all the events, so its turn number is the last turn_start's; count back from it.
      let turn = msg.view?.turn;
      const events = [...(msg.events ?? [])].reverse().map((e) => (e.kind === "turn_start" && turn ? { ...e, turn: turn-- } : e)).reverse();
      const lines = events.map((e) => describeEvent(ctx, e));
      const log = lines.length ? [...state.log, ...lines].slice(-LOG_LIMIT) : state.log;
      const avatars = msg.avatars?.length ? msg.avatars : state.avatars;
      return { ...state, view: msg.view, seq: msg.seq, names, avatars, known, log, waiting: false, error: "" };
    }
    case "room_closed":
      return { ...leave(state), closedReason: msg.message ?? "room closed" };
    case "error":
      return { ...state, error: `${msg.code}: ${msg.message}` };
    default: // hello, pong, anything newer than this client
      return state;
  }
}

/** Seats the server is waiting on. */
export const awaited = (state) => state.view?.awaiting ?? [];

/** Is it this seat's decision? */
export const isMyDecision = (state) => !!state.view && state.view.awaiting.includes(state.view.you) && state.view.legal.length > 0;
