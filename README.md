# Card Wars

A web port of the Adventure Time Card Wars card game: a Go rules engine and
WebSocket game server, and a browser client. Play the bot, or a friend with a room code.

```
server/   Go rules engine, card data, game server and terminal client (see server/README.md)
web/      the browser client: plain ES modules, no build step (see web/README.md)
```

The two halves talk over the WebSocket protocol in `server/docs/protocol.md`; a change to it
changes both, which is why they live in one repository.

## Run it

```bash
docker compose up -d --build   # then open http://localhost:8088
```

That starts the game server and the web client behind one port. To develop instead, run the
server (`go run ./cmd/cardwarsd -origins 'localhost:*'` in `server/`) and the client
(`npm start` in `web/`), then open http://localhost:5173/. Each directory's README has the details.

## Tests

```bash
cd server && go test ./...
cd web && npm test
```

## Working on it

Changes go through a branch and a pull request, with
[Conventional Commits](https://www.conventionalcommits.org/) messages and
[Conventional Branches](https://conventional-branch.github.io/) names (`feat/...`, `fix/...`).
Issues for either half go in this repository.

## Known limitations

- **Card art preloading names hidden cards.** To have art ready before a card is played or drawn,
  the server sends each player a `preload` list of card ids: the opponent's hand plus your next
  3 draws, mixed, de-duplicated and sorted so it does not say which is which. A modified client,
  or anyone reading the network traffic, can still learn that those cards are in the opponent's
  hand or on top of the deck. Everything else hidden (hands, deck order) is never sent. It is a
  deliberate trade for faster loading; see `server/docs/protocol.md` ("Hidden information") and
  remove the field if it matters for your games.
