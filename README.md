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

## CI and images

GitHub Actions (`.github/workflows/`) runs on every pull request and on `main`: `gofmt`, `go vet` and
`go test` for the server, `npm test` for the web client, a smoke test that plays whole games through the
client against a real server, and a Docker build of both images.

Images are published only for releases. Tagging a version pushes both images to GitHub's container
registry for `linux/amd64` and `linux/arm64`, as `ghcr.io/lukas-simonson/cardwarstcg-server` and `-web`,
tagged `1.2.3`, `1.2` and `1` (the last not for 0.x), and `latest` for a stable release. It also creates
a GitHub Release with generated notes. Merging to `main` publishes nothing.

To cut a release, tag the commit on `main`:

```bash
git tag v0.1.0 && git push origin v0.1.0
```

A tag with a suffix, like `v0.2.0-rc.1`, is a pre-release: it gets its own version tag and no `latest`.
To run the images instead of building them, pin a version in `docker-compose.yml` (or keep `latest`):

```bash
docker compose pull && docker compose up -d
```

The first push creates private packages. Make them public under the repository's Packages settings, or
`docker login ghcr.io` with a personal access token that has `read:packages` on the machine that pulls.

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
