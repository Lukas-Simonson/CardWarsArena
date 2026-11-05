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

The server can keep accounts and decks in a SQLite file (`-db path`, e.g. `-db cardwars.db`). Nothing
uses it yet; it is the foundation for saved decks. With Docker Compose the file lives in the
`cardwars-data` volume, so it survives `docker compose pull && docker compose up -d`.

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

## Signing in

By default the server is anonymous. To let people sign in with Discord and/or GitHub (the first step
toward saved decks), register an app with the provider, with the callback
`<public url>/api/auth/callback/discord` (or `/github`), then copy `.env.example` to `.env`, fill it in and
`docker compose up -d`. Only the provider's id for the person and a display name are stored: no email,
no password. Signed-in players play under their account name and can build and keep decks: the home screen's "Build a new deck" opens a builder that searches every card the
game can play, starts from a starter deck if you like, and says what a deck still needs. Saved decks that are
complete appear on the deck shelf. Guests can still play. The details are in
[`server/docs/protocol.md`](server/docs/protocol.md).

## Configuration

Both images are configured through environment variables, so they can run on any host that runs containers
(compose, Railway, Fly, a VPS). The server's settings are also command-line flags; the flag wins when both are set.

**Server** (`cardwarstcg-server`, listens on 8080):

| Variable | Flag | What it does |
|---|---|---|
| `CARDWARS_DB` | `-db` | The SQLite file for accounts and decks. Without it nothing is kept. In the image, put it on a volume, e.g. `/var/lib/cardwars/cardwars.db`. |
| `CARDWARS_AUTH` | `-auth` | `none` (the default) or `oauth` for sign-in with Discord and GitHub. `oauth` needs `CARDWARS_DB` and `CARDWARS_PUBLIC_URL`. |
| `CARDWARS_PUBLIC_URL` | `-public-url` | Where browsers reach the site, e.g. `https://cards.example.com`: the address only, with no path. Providers send people back to it, and it decides whether cookies are `Secure`. |
| `CARDWARS_DISCORD_CLIENT_ID`, `CARDWARS_DISCORD_CLIENT_SECRET` | | The app's credentials at Discord. A provider without both is not offered. |
| `CARDWARS_GITHUB_CLIENT_ID`, `CARDWARS_GITHUB_CLIENT_SECRET` | | The same for GitHub. |

The other flags (`-addr`, `-data`, `-origins`, `-bot-delay`, `-dev`) are described in [`server/docs/protocol.md`](server/docs/protocol.md).

**Web** (`cardwarstcg-web`, Caddy, listens on 80):

| Variable | What it does |
|---|---|
| `CARDWARS_UPSTREAM` | Where the game server is, as `host:port`. Defaults to `server:8080`, the name compose gives it. |

Things to know when running the images yourself:

- The browser must reach the web image and the server through **one address**: the web image serves the page and proxies `/ws` and `/api/*` to the server, and sign-in relies on the cookie being first-party. Expose the web image publicly and keep the server private.
- The server runs as a non-root user (uid 65532), so the directory behind `CARDWARS_DB` must be writable by it. A named volume is; a folder on the host needs `chown 65532:65532`. The card data lives in the image at `/data`: do not mount over it.
- Games live in the server's memory, so restarting it ends every game in progress. Accounts and decks are in the database and survive.
- A database file can be used by one server at a time: run a single instance.
- Images are published for releases only, tagged `1.2.3`, `1.2`, and `latest`: pin a version to control when you update.

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
