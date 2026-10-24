// The client's model of the server conversation. Pure: reduce(state, msg)
// returns the next state and never touches the DOM or the network.
import { describeEvent } from "./describe.js";

export const LOG_LIMIT = 300;

export function initialState() {
  return {
    status: "closed",      // connection: closed | connecting | open | reconnecting
    session: null,         // { room, token, seat, vsBot } once seated
    waiting: false,        // room created, no opponent yet
    names: [],             // seat display names
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
    (p.hand ?? []).forEach(add);
    p.discard.forEach(add);
    for (const l of p.lanes) {
      add(l.creature);
      add(l.building);
    }
  }
  add(view.pending);
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
      const ctx = { view: msg.view, cards, names, known };
      const lines = (msg.events ?? []).map((e) => describeEvent(ctx, e));
      const log = lines.length ? [...state.log, ...lines].slice(-LOG_LIMIT) : state.log;
      return { ...state, view: msg.view, seq: msg.seq, names, known, log, waiting: false, error: "" };
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
