# Contributing

Thanks for helping with Card Wars Arena. This page covers how the repository is worked on. For what the project is
and how to run it, see the [README](README.md).

Card Wars Arena is an unofficial fan project; please keep it that way (see the README's [fan project](README.md#a-fan-project)
note). Do not add artwork, card scans or other material you do not have the right to share.

## Reporting problems and ideas

Use the [issue templates](https://github.com/Lukas-Simonson/CardWarsArena/issues/new/choose):

- **Bug report**: something is broken or looks wrong.
- **Card or rules problem**: a card does something different from what it says, or a rule is played wrongly. Say what
  the card says, what the game did, and the board it happened on; that usually decides how it should work.
- **Feature request**: something you would like to see.

Search the existing issues first. For anything bigger than a small fix, open or comment on an issue before writing code,
so the approach can be agreed.

## Setting up

You need Go (the version in `server/go.mod`) and Node 22 or newer. Docker is optional.

```bash
cd server && go run ./cmd/cardwarsd -origins 'localhost:*'   # the game server on :8080
cd web && npm start                                          # the client on :5173
```

Open http://localhost:5173/. Add `-dev` to the server to let a client choose seeds while debugging. The terminal client
(`go run ./cmd/cardwars`, see `server/README.md`) is the quickest way to try a rule without the browser.

## Making a change

1. Work from an issue where there is one.
2. Create a branch from `main`, named in the [Conventional Branches](https://conventional-branch.github.io/) style:
   `feat/deck-export`, `fix/bmo-deck`, `docs/readme`, `chore/ci-cache`.
3. Commit with [Conventional Commits](https://www.conventionalcommits.org/) messages, e.g.
   `fix(data): take Cerebral Bloodstorm out of BMO's deck`. Common types are `feat`, `fix`, `docs`, `test`, `refactor`
   and `chore`; the scope is the area (`engine`, `web`, `server`, `data`, `auth`). Reference the issue (`Refs #31`) in the
   commit, and `Closes #31` in the pull request.
4. Keep a branch to one topic, and commit each coherent step as it happens rather than one large commit at the end.
5. Open a pull request against `main`. Say what changed and why, and what you did to check it. CI should pass before it is merged.

## Checks

CI runs these on every pull request, and you can run them locally:

```bash
cd server
gofmt -l .          # must print nothing
go vet ./...
go test ./...       # go test -short ./... plays fewer random games

cd ../web
npm test            # unit tests, no server needed
```

There is also a smoke test that plays whole games through the web client against a real server:

```bash
cd server && go run ./cmd/cardwarsd -bot-delay 1ms &      # start the server
cd web && node test/smoke.mjs http://localhost:8080 10    # 10 games
```

CI also builds both Docker images. A change that adds behaviour should add or update a test with it.

## Where things live

- **Rules engine:** `server/internal/engine`. It is pure and deterministic: the same seed and actions give the same game.
  Keep it free of I/O, and put hidden-information rules in `server/internal/view`, not the client.
- **Protocol:** `server/docs/protocol.md`. A change to messages changes the server and the web client together, so make
  them in one pull request and update the document.
- **Web client:** `web/js`. Plain ES modules with no build step and no dependencies. Logic that can be pure (reducers,
  planning, text) goes in its own module with a test in `web/test`. The look follows the project's design (wood, felt
  and steel, Luckiest Guy and Patrick Hand), so match what is there.

### Card data

Cards come from two places, and neither is edited the obvious way:

- `server/data/CardWarsCatalogue.json` is the source catalogue: names, costs, text and which deck each printing belongs
  to. **Do not edit `server/data/cards/*.json` by hand.** Change the catalogue and regenerate:

  ```bash
  cd server && go run ./cmd/cardimport
  ```

  A card's `quantity` is the sum of its printings. The importer's test pins the size of each deck, so a miscount fails
  the build; update it deliberately if a deck really changes.
- `server/data/effects/*.json` holds the hand-written rules for each card, in the effect language described in
  `server/docs/effects.md`. A card without an entry there is "unmodelled": the server only lets it be played with `-dev`.

The server reads card data once at startup, so restart it to see a change. The image bakes the data in, so players get
it with the next release.

## Releases

Images are published only for releases, not for every merge to `main`. A maintainer tags a commit on `main`:

```bash
git tag v1.2.3 && git push origin v1.2.3
```

The `publish` workflow then builds `ghcr.io/lukas-simonson/cardwarsarena-server` and `-web` for `linux/amd64` and
`linux/arm64`, tags them `1.2.3`, `1.2` and `1` (the last not for 0.x) and `latest` for a stable release, and creates a GitHub Release with
generated notes. A tag with a suffix, like `v0.5.0-rc.1`, is a pre-release: it gets its own version tag and no `latest`.

## AI assistance

Using an AI coding assistant is fine. Say so in the pull request description, and read and understand what it
wrote before asking for a review: you are responsible for the change, its tests and the rules it implements. The
README's [AI Disclosure](README.md#ai-disclosure) says how the project itself was built.

## License

By contributing you agree that your contribution is covered by the project's [license](LICENSE), which applies to the
code only and not to the Adventure Time or Card Wars material described in the README.
