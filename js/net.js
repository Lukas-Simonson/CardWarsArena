// A WebSocket that reconnects. It only carries JSON messages; what to say
// after reconnecting (rejoin) is the caller's job, triggered by onStatus("open").

export class Connection {
  /**
   * @param {object} o
   * @param {string} o.url ws:// or wss:// address of the server's /ws
   * @param {(msg:object)=>void} o.onMessage
   * @param {(status:string)=>void} o.onStatus  connecting | open | reconnecting | closed
   */
  constructor({ url, onMessage, onStatus, WebSocketImpl = globalThis.WebSocket, timers = globalThis, backoff = [500, 1000, 2000, 4000, 8000] }) {
    Object.assign(this, { url, onMessage, onStatus, WebSocketImpl, timers, backoff });
    this.ws = null;
    this.wantOpen = false;
    this.attempt = 0;
    this.timer = null;
  }

  open() {
    this.wantOpen = true;
    this.#connect("connecting");
  }

  /** Send a message; false if the socket is not open. */
  send(msg) {
    if (!this.ws || this.ws.readyState !== 1) return false;
    this.ws.send(JSON.stringify(msg));
    return true;
  }

  /** Close for good (no reconnecting). */
  close() {
    this.wantOpen = false;
    this.timers.clearTimeout(this.timer);
    this.timer = null;
    const ws = this.ws;
    this.ws = null;
    ws?.close();
    this.onStatus("closed");
  }

  #connect(status) {
    this.onStatus(status);
    const ws = new this.WebSocketImpl(this.url);
    this.ws = ws;
    ws.onopen = () => {
      if (this.ws !== ws) return;
      this.attempt = 0;
      this.onStatus("open");
    };
    ws.onmessage = (e) => {
      if (this.ws !== ws) return;
      let msg;
      try {
        msg = JSON.parse(e.data);
      } catch {
        return; // not ours to handle
      }
      this.onMessage(msg);
    };
    ws.onclose = () => {
      if (this.ws !== ws) return; // a socket we already replaced or closed
      this.ws = null;
      if (!this.wantOpen) return;
      const delay = this.backoff[Math.min(this.attempt, this.backoff.length - 1)];
      this.attempt++;
      this.onStatus("reconnecting");
      this.timer = this.timers.setTimeout(() => this.#connect("reconnecting"), delay);
    };
    ws.onerror = () => {}; // a close event always follows
  }
}

/** http(s)://host:port -> ws(s)://host:port/ws */
export function wsUrl(serverBase) {
  const u = new URL(serverBase);
  u.protocol = u.protocol === "https:" ? "wss:" : "ws:";
  u.pathname = "/ws";
  u.search = "";
  return u.toString();
}

/** Normalise what a person typed into a server address. */
export function normaliseServer(input) {
  let s = (input ?? "").trim();
  if (!s) return "";
  if (!/^[a-z]+:\/\//i.test(s)) s = "http://" + s;
  const u = new URL(s);
  if (u.protocol === "ws:") u.protocol = "http:";
  if (u.protocol === "wss:") u.protocol = "https:";
  return u.origin;
}
