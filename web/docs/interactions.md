# How card interactions work

This is the rule book for every place the game asks the player something: choosing a target for a Spell, answering a
pop-up, confirming a play. It exists so that two similar questions look and behave the same, and so that a new card
does not quietly invent a new way of asking.

If you are adding or changing a card, read [the checklist](#checklist-for-a-new-or-changed-interaction) first. If you
are about to add a new pop-up, a new button colour or a new word for "skip", stop and read the rule it would break.

- **Status:** draft, written for [#107](https://github.com/Lukas-Simonson/CardWarsArena/issues/107) (phase 1 of the UI
  consistency work, [#88](https://github.com/Lukas-Simonson/CardWarsArena/issues/88)). It describes how the game *should*
  work. [The audit](interaction-audit.md) lists where it does not yet, and which phase fixes each case.
- Every rule has a short code (W1, A2, B3...) so the audit, pull requests and code comments can point at it.

## Contents

1. [Principles](#principles)
2. [Where a question is asked](#where-a-question-is-asked) (W)
3. [The anatomy of a question](#the-anatomy-of-a-question) (A)
4. [Vocabulary](#vocabulary) (V)
5. [Button roles](#button-roles) (B)
6. [Cards that ask several things](#cards-that-ask-several-things) (M)
7. [What the player must be able to see](#what-the-player-must-be-able-to-see) (I)
8. [Feedback and undo](#feedback-and-undo) (F)
9. [Accessibility and phones](#accessibility-and-phones) (X)
10. [Decision log](#decision-log)
11. [Checklist for a new or changed interaction](#checklist-for-a-new-or-changed-interaction)
12. [Where the code lives](#where-the-code-lives)

## Principles

1. **The same question looks the same.** If two cards ask "choose a Creature", they ask it the same way, whatever card
   asks. A difference between two questions must mean something to the player.
2. **The player can always see why.** The card that asked is shown, the sentence says what is being decided, and
   anything that matters to the decision is visible while deciding.
3. **Nothing is final without a way back, until it is.** Choosing is undoable (Esc, Cancel) until the card is played;
   after that, effects are shown, not hidden.
4. **Words come from the catalogue** (`web/locales/en.json`), never from the code. One word for each idea.
5. **One set of building blocks.** A pop-up is configuration of the shared components, not new markup.

## Where a question is asked

A card asks for things in two moments: **up front**, as it is played or used (its *targets*, chosen together with the
action), and **mid-effect**, when the engine stops to ask (a *choice*: `State.Choice` in the engine, `choice` in the
view). The rules below apply to both.

- **W1. A question that names a board object is answered by clicking the board.** The board objects are: a card in play
  (a Creature or Building), a Lane, a Landscape, a discard pile, a Hero, and a player. The legal ones are highlighted and
  the rest are dimmed. This includes mid-effect choices ("choose a Creature to burn", "choose a Lane to move to"); they
  are *not* pop-ups.
- **W2. Anything else is a pop-up.** That is: an amount, keep or discard, a yes or no, a choice among cards that are not
  on the board (your hand, a revealed set, your deck), and a choice of a type (a Landscape type for Silo of Freedom).
- **W3. A question that mixes both asks the board part first, then the pop-up.** One decision never uses two surfaces
  at once.
- **W4. Targets chosen up front are always board clicks, with one confirmation at the end** (see A6). The bar above the
  player's hand carries the guidance, the Skip, Cancel and Confirm buttons.
- **W5. A viewer is not a question.** The discard-pile viewer, a card's close-up and the log show information. They can be
  opened on their own (and closed with Close or Esc); when a question needs a pile to be seen, the pile viewer is part
  of the question (see I2) and closes when an answer is picked.

Why this split: choosing among things that are on the table is quicker and clearer when you point at them, and a pop-up
that hides the table while it asks you to choose from the table makes you remember what was under it. A decision that is
not about a thing on the table has nothing to point at, so a pop-up is the right size for it.

## The anatomy of a question

Every question, on the board or in a pop-up, has these parts, in this order and in the same place:

1. **A1. Source.** The card that asked, shown (its face, small) and named. Never only a name.
2. **A2. The sentence.** One sentence: what is being decided, naming the target and, where it fits, the amount.
   "Choose a Creature to heal 3 damage from." Not "Choose a target". The card's full rules text is not repeated; enough to
   decide, and short enough for a phone.
3. **A3. The options.** What can be chosen, each with a label that says what happens. A pop-up of cards shows the cards;
   a pop-up of numbers shows what each number does ("1 damage", not "1").
4. **A4. Skip or decline.** Only if the card's rules allow it. It is the last option, always a neutral button (B3), and
   always worded as in V1. It is absent when the rules require an answer.
5. **A5. Results.** Where the client already knows an option's result, show it ("Corn Dog 4 → 3", "heal 2", "cost 2").
   This is only where it is cheap to compute (damage, heal, move, cost). If it is not cheap, show the label only. This
   list says which kinds show results: *(to be settled in phase 2, with the components; see the decision log)*.
6. **A6. Confirmation.** Only when it protects something: a card with several steps (so the player sees what will happen
   before spending the action), or a card whose optional parts were all skipped (so it is not played by accident to do
   nothing). A question with a single click as its answer has no Confirm. The label is the verb and the card: "Play Corn
   Dog".

## Vocabulary

One word for each idea. The old words are retired; a pull request that brings one back needs a reason.

- **V1. Skip** leaves an optional part out. Its label names what is skipped, in the form **"Don't {verb} {thing}"**:
  "Don't move a Building", "Don't heal a Creature". It is set on the effect data (`skip_label`), so the card owns the
  wording. Retired: *None*, *Choose nothing*, *Leave it*, *Let it go*, *Stay*, *Take it instead*.
- **V2. Cancel** backs out of the *whole card* before anything is spent. It exists on the board prompt only, and is
  always the last button there. Esc does the same.
- **V3. Confirm** commits (A6). It says what it commits: the verb and the card name.
- **V4. Done** finishes choosing several things when more could be added.
- **V5. Close** dismisses a viewer that decides nothing (W5).
- **V6. A real alternative is an option, not a skip.** When declining has its own effect (discard a card *or* take 5
  damage), both are options of equal weight, and the one that costs the player something uses the danger role (B4).

## Button roles

Colour means one thing, everywhere. The class names are in `web/css/style.css`.

| Role | Class | Means | Examples |
|---|---|---|---|
| **B1. Choice** | `btn-purple` | One of several answers to a question | a Landscape type, an amount, "Put it under the Camel" |
| **B2. Commit** | `btn-green` | Go ahead, keep, start, advance | Confirm, Keep this hand, End turn, Practice, Save |
| **B3. Neutral** | `btn-steel` | Skip, Cancel, Close, back, view, a secondary action | Skip, Cancel, Close, Draw a card, Log |
| **B4. Danger** | `btn-red` | An answer that costs the player something or can't be taken back, and the attack | Take the damage, Destroy it, Fight! |
| **B5. Special** | `btn-ice` | Thaw a Landscape (its own control, on the Landscape) | Thaw |

Size and spacing:

- **B6. Touch size.** At least 44 px high, and at least 44 px wide for an icon or a number.
- **B7. Spacing.** Buttons in a row are separated by one spacing step (one token, set once in the shared component, not
  chosen per pop-up). Rows wrap; they do not touch.
- **B8. Order.** Options first, then the skip or decline, then (on the board prompt) Cancel and Confirm. Confirm is on
  the end, where the eye finishes.

## Cards that ask several things

- **M1. Name the steps** when there are several ("Step 2 of 3: choose the Lane to move it to"), in the order the rules
  need them.
- **M2. A step that cannot apply is skipped, and the player is told why** ("There is no Building to move"), not silently
  dropped and not asked.
- **M3. A step that depends on another is only asked after it.** Skipping the Building never asks for a destination Lane
  (Reclaim Landscape, #79). The effect data says so with `requires`.
- **M4. Pick the thing, then say what to do with it.** When one target can be used in several ways (heal or deal damage),
  the player picks the Creature and then the mode, as often as the card allows (`group` and `together` in the effect
  data).
- **M5. Before the card is played, show what will happen** when any part was skipped (A6), including "this will do nothing".

## What the player must be able to see

- **I1. Everything that matters to the choice is visible while choosing.** The target and its result; the other
  player's relevant cards; the numbers that decide it.
- **I2. Piles.** A choice from a discard pile shows *both* players' piles when the card may take from either, each under
  its own heading, scrollable, with an empty pile shown as empty (#85). A card that may take from only one shows only that
  pile. The server says which in `piles` on the legal action.
- **I3. Hidden information stays hidden.** The opponent's hand and deck are never shown; what a card reveals is shown to
  both players.
- **I4. Whose turn it is to answer** is always clear: the other player's question shows as "Waiting for Bob to choose",
  with the card that asked.

## Feedback and undo

- **F1. Legal targets are highlighted and illegal ones dimmed** when a board question is open.
- **F2. The selection is visible** (the chosen card, Lane or pile stays marked until the question is answered).
- **F3. Undo with Esc or Cancel** while building a play. A mid-effect decision (an engine choice) cannot be cancelled:
  the effect has begun. It has a skip if the rules allow one.
- **F4. Effects are shown after the answer** (the animation and the log), in the order given in the animation rules
  (`web/js/anim.js`; #100, #101).

## Accessibility and phones

- **X1. Focus** moves into a pop-up when it opens, stays inside it, and returns to where it was on close.
- **X2. Every control is a real button** with an accessible name and, for a toggle, `aria-pressed`. A board target has a
  label saying what it is ("Your Lane 2 Creature: Corn Dog").
- **X3. Esc** closes viewers and cancels a board selection; it never answers a decision for the player.
- **X4. Phones (375 px).** Nothing needs sideways scrolling; options wrap; a pop-up scrolls inside its own frame; touch
  targets follow B6. Check every new question at 375 px wide (see #34).
- **X5. Words** go through the string catalogue (#35), with plurals handled by the catalogue's `plural` form, and the
  catalogue check (`npm run i18n`) must pass.
- **X6. Reduced motion** shows the same information without the movement.

## Decision log

Decisions, who made them, when, and why. Add a line for each change to this guide; never edit an old line.

| Date | Decision | Why |
|---|---|---|
| 2026-11-21 | **A question that names a board object is answered on the board; anything else is a pop-up; a mixed question asks the board part first, then the pop-up** (W1-W3). | Choosing among things on the table is quicker by pointing than from a pop-up that hides the table. A non-board decision has nothing to point at. |
| 2026-11-21 | **Work is tracked by one sub-issue per phase under #88** (#107 guide and audit, #108 components, #109 self-describing choices, #110 gallery, tests and checklist), each with its own small pull requests and a design note on the issue before it starts. | Each phase can be reviewed on its own. |
| 2026-11-21 | **An option's result is shown where the client already knows it** (damage, heal, move, cost), not everywhere (A5). | Computing every result in the server is large; most of the benefit is in the cheap cases. |
| 2026-11-21 | **When the engine describes questions generically (phase 3), the old choice kinds are replaced outright in the same release.** | The client and server ship together and the protocol has no stability promise. Keeping both invites drift. |
| 2026-11-21 | **The words for skip, cancel, confirm, done and close, and the button roles, are those in V and B above.** Proposed in the first draft of this guide, to be confirmed in review. | They match how the game already uses most of them; the audit lists the exceptions. |

## Checklist for a new or changed interaction

Copy this into the pull request (it is in the pull request template).

- [ ] **Which pattern is this?** Name the rule or the audit row it follows (for example "W1, a board click; M4 mode
      choice"). If it does not follow one, say why it cannot, and propose a rule or a change to one in this guide.
- [ ] **Is it asked in the right place?** A board object is a board click (W1); anything else is a pop-up (W2); a mixture
      asks the board part first (W3).
- [ ] **Does it have the whole anatomy?** Source card (A1), one sentence naming the target (A2), options (A3), a skip only
      if the rules allow it (A4), results where cheap (A5), a confirmation only where it protects (A6).
- [ ] **Are the words from V?** Skip labels set on the effect (`skip_label`), written "Don't {verb} {thing}". Every string
      is in `web/locales/en.json`.
- [ ] **Are the button roles right?** (B1-B5), 44 px, one spacing step.
- [ ] **Does a dependent step only come after its step?** (M3), and are steps that cannot apply skipped with a reason (M2)?
- [ ] **Can the player see what they need?** Piles (I2), the target and its result (I1).
- [ ] **Does it work with the keyboard, a screen reader, and at 375 px?** (X1-X4)
- [ ] **Is it in the gallery, and is there a test?** *(from phase 4; until then, test the pure part: the picker, the prompt text.)*
- [ ] **Is the audit up to date?** If this adds or changes an interaction, update [the audit](interaction-audit.md).

## Where the code lives

For orientation; this is also where a rule is enforced.

| Part | File | What it does |
|---|---|---|
| The legal actions become clicks | `web/js/picker.js` | `ActionPicker` turns the server's legal actions into tokens (`lane:2:creature`, `discard_card:1:42`, `creature:0:1\|mode:0`), remembers the selection, and says what to ask next. Pure; tested in `web/test/picker.test.mjs`. |
| What the prompt bar says | `web/js/prompt.js` | `buildPrompt` words the guidance, Skip, Cancel and Confirm from the picker. Pure. |
| Words for actions and cards | `web/js/describe.js` | `describeAction`, `summaryText`, `slotFilled`, `describeRef`: the sentences. |
| The board and its prompt bar | `web/js/render.js` | The battlefield, the hand, the prompt bar (`promptBar`), and **`choiceModal`**: today one hand-built branch for each kind of engine choice (phase 2 replaces these with shared components). |
| The pile viewer | `web/js/render.js` (`pileModal`) | Both discard piles when the card may take from either. |
| Button and modal styles | `web/css/style.css` | `btn-*`, `.modal*`, `.chip`. |
| The strings | `web/locales/en.json` | Every sentence; checked by `npm run i18n`. |
| The engine's questions | `server/internal/engine/choice.go`, `server/docs/effects.md` | The choice kinds, and how a card's effect becomes a question (targets, `group`, `optional`, `requires`, `skip_label`, `together`). |
| The wire format | `server/docs/protocol.md` | The `choice` in the view and the legal actions that answer it. |
