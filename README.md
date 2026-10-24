# Card Wars web client

A deliberately plain browser client for the Card Wars server (`../server`):
no animations, no build step, no dependencies. It exists to try the game and
the protocol out before the Godot client.

## Run it

```
# 1. the game server (in ../server); -origins lets this page talk to it
go run ./cmd/cardwarsd -origins 'localhost:*'

# 2. this client
npm start            # or: node serve.mjs 5173
```

Open http://localhost:5173/. The page asks for the server address (default
`http://localhost:8080`; also settable with `?server=http://host:port`).

* **Play against the bot** or **create a room** and give the code to a friend
  (open a second browser tab to play yourself).
* Every legal action is a button. Click a card in your hand, or a Creature or
  Building on your side of the board, to narrow the list to what that card can
  do. Green outlines mark cards that can act right now.
* Click any card to read it. The discard piles are clickable.
* Reloading the page puts you back in your seat.

## Layout

```
index.html       lobby markup and the #game mount point
css/style.css    light and dark themes
js/net.js        reconnecting WebSocket, server address helpers
js/state.js      pure reducer: server messages -> client state
js/describe.js   pure: actions, events and targets -> text; click filtering
js/render.js     draws the game screen (DOM)
js/main.js       wiring: lobby, connection, rejoin, click handlers
test/            node --test unit tests, fixtures, and a smoke test
serve.mjs        tiny static server for development
```

Everything with logic (`net`, `state`, `describe`) is DOM-free and unit tested;
`render` and `main` are thin.

## Tests

```
npm test                              # unit tests, no server needed
node test/smoke.mjs [server] [games]  # whole games against a running server
```

The smoke test is the important one: it plays many games over a real WebSocket
and fails if the client meets an action it cannot describe or a card it
cannot name.

## Protocol notes for client authors

See `../server/docs/protocol.md`. Two rules this client follows that any client
must:

* send `seq` with every action, and expect `stale_state` when the game moved;
* resend after `rate_limited`: that message was dropped and no update follows.
