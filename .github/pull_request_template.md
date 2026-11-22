## What changed and why

<!-- What this does, and the issue it closes (`Closes #123`) or refs. -->

## How it was checked

<!-- Tests run, and what you tried by hand. Say what you did not check. -->

## If this adds or changes how a card asks the player something

<!-- Delete this section if it does not. The rules are in web/docs/interactions.md. -->

- [ ] **Which pattern is this?** Name the rule or audit row it follows (for example "W1, a board click; M4 mode choice"). If it cannot follow one, say why, and propose a change to the guide.
- [ ] Asked in the right place: a board object is a board click, anything else a pop-up, a mixture asks the board part first (W1-W3).
- [ ] The whole anatomy: the source card, one sentence naming the target, options, a skip only if the rules allow it, results where cheap, a confirmation only where it protects (A1-A6).
- [ ] The words and button roles are the guide's (V, B); every string is in `web/locales/en.json`.
- [ ] A dependent step is asked only after its step, and steps that cannot apply are skipped with a reason (M2, M3).
- [ ] Works with the keyboard and a screen reader, and at 375 px wide (X1-X4).
- [ ] A new mid-effect question followed "Adding a question" in `server/docs/effects.md` and needed no client code.
- [ ] `web/docs/interaction-audit.md` is updated.
