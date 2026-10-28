// Spaces out updates so a quick opponent stays followable. Server updates
// arrive as fast as the bot moves; the pacer delivers the first at once and
// holds each later one until the previous has had time to play out.
export class Pacer {
  /**
   * @param {object} o
   * @param {(msg:object)=>void} o.deliver  apply a message to the client
   * @param {(msg:object)=>number} o.holdFor  after delivering msg: ms before the next may be shown
   */
  constructor({ deliver, holdFor, now = () => Date.now(), timers = globalThis }) {
    Object.assign(this, { deliver, holdFor, now, timers });
    this.queue = [];
    this.readyAt = 0;
    this.timer = null;
  }

  push(msg) {
    this.queue.push(msg);
    this.#pump();
  }

  /** Drop everything waiting (the room ended, or we left it). */
  clear() {
    this.queue = [];
    this.timers.clearTimeout(this.timer);
    this.timer = null;
  }

  get waiting() {
    return this.queue.length;
  }

  #pump() {
    if (this.timer != null || this.queue.length === 0) return;
    const wait = this.readyAt - this.now();
    if (wait > 0) {
      this.timer = this.timers.setTimeout(() => { this.timer = null; this.#pump(); }, wait);
      return;
    }
    const msg = this.queue.shift();
    this.deliver(msg);
    this.readyAt = this.now() + this.holdFor(msg);
    this.#pump();
  }
}
