# Interaction audit

Every place the game asks the player something, checked against [the interaction guide](interactions.md). It is the list
the fixes come from: each deviation has an ID (D1, D2...) and the phase of the
[UI consistency work (#88)](https://github.com/Lukas-Simonson/CardWarsArena/issues/88) that fixes it.

- **Audited:** 2026-11-21, from the code at the start of phase 1 (#107): `web/js/render.js`, `picker.js`, `prompt.js`,
  `describe.js`, `web/css/style.css`, `server/internal/engine/choice.go` and the card data in `server/data/effects/`. Not
  recorded in a running game, so a visual problem that cannot be read from code is not here yet (the gallery in phase 4
  will show them).
- **Keep it current.** A pull request that adds or changes an interaction updates this file (it is in the checklist).
  The tables in sections 2 and 3 were generated from the card data; if you add a card, add its row by hand or regenerate.

## Summary

- **117** card interactions choose something (targets chosen up front or by a trigger), in **44** different shapes.
  The board-click flow handles all of them except the five cards whose trigger asks mid-effect (see D6).
- **14** kinds of mid-effect engine choice, each a hand-built branch of `choiceModal` (#88 counted thirteen; `defend`
  was added since).
- **Buttons** in `render.js`: purple 13, steel 13, red 5, green 4, ice 1, with no stated rule until
  [the guide](interactions.md#button-roles).
- **Skips and declines** are worded seven ways ("Leave it", "Let it go", "Choose nothing", "Take it instead", "{name}
  fights", "Let it fight where it is going", "Destroy {name}").
- **No pop-up shows the card that caused it**, only its name.

## 1. The engine's mid-effect choices (`choiceModal`)

Each row is one branch of `choiceModal` in `web/js/render.js`. "Guide" says whether it follows the rules; the codes are
those of [the guide](interactions.md).

| Kind | What it asks | Shown today as | Follows the guide? | Fixed in |
|---|---|---|---|---|
| `discard` | Pick N cards from your hand to discard | Pop-up: your hand as cards; title "Discard {n}" | W2 ok. **A1**: only the name of the asking card in the sentence (D1). No skip (correct: required). | 2b |
| `ransom` | Discard a card of at least a cost, **or** take the effect | Pop-up: hand cards + red "Take it" | W2 ok; B4 ok (it costs the player). **V6/A5**: "Take it" does not say what is taken ("Take 5 damage") (D3). **A1** (D1). | 2b / 2c |
| `upkeep` | Discard N cards, **or** destroy the asking Creature | Pop-up: hand cards + red "Destroy {name}" | W2 ok; B4 ok. **A1** (D1). | 2b |
| `rebound` | Discard a card to return the Creature, **or** not | Pop-up: hand cards + steel "Leave it" | W2 ok. **V1**: "Leave it" is a retired word, should be "Don't return {name}" (D2). **A1** (D1). | 2b |
| `reveal` | Pick one of the revealed cards (the rest are discarded) | Pop-up: the cards | W2 ok. **A1** (D1). No skip (correct). | 2b |
| `search` | Pick a Building from the deck | Pop-up: the cards | W2 ok. **A1** (D1). | 2b |
| `redirect` | Pick a Flooped Creature of yours to fight instead, **or** let the attacker fight | Pop-up: cards + red "{name} fights" | **W1**: it picks a Creature on the board and should be a board click (D4). **B4**: red is used for "stay put", not a cost (D5). **V1** (D2). | 2d |
| `defend` | Pick a Creature of yours to take an attack, **or** let it go on | Pop-up: cards + steel "Let it fight where it is going" | **W1** (D4). **V1**: should be "Don't take the Fight" (D2). | 2d |
| `move_damage` | Pick how much damage to move between two Creatures | Pop-up: number buttons | W2 ok. **A5**: the buttons are bare numbers and do not say between which Creatures or the result (D3). | 2c |
| `move` | Pick the empty Landscape a Creature moves to | Pop-up: "Lane 1".."Lane 4" buttons | **W1**: it picks a Landscape on the board and should be a board click (D4). **A3**: "Lane n" does not say what is there. | 2d |
| `camel` | Put the leaving Creature under your Camel, **or** let it go | Pop-up: the card + purple "Put it under {name}" + steel "Let it go" | W2 ok. **V1**: "Let it go" is retired, "Don't put it under {name}" (D2). | 2c |
| `landscape` | Pick the Landscape type your Lane becomes | Pop-up: type buttons | W2 ok. **A1**: no card shown (D1). | 2c |
| `target` | Pick what a trigger acts on (a Creature, Landscape or pile card), optionally | Pop-up: one button per target ("Lane 2 Creature..."), steel "Choose nothing" | **W1**: it picks board objects and should be a board click (D4). **V1**: "Choose nothing" is retired, the card's `skip_label` should be used (D2). | 2d |
| `amount` | Pick one of the numbers (The Big Hen House: 1 or 2 damage) | Pop-up: number buttons | W2 ok. **A5**: bare numbers; "1 damage" / "2 damage" and the heal that follows (D3). | 2c |

Which cards ask each kind (from the card data):

| Choice kind | Cards that ask it |
|---|---|
| `discard` | Abraca Amadeus, Bone Wand, Drained Cleric, Fiddling Ferret, Frosty Frolic, Glorious Gramophone, Jinxed Parrotrooper, Mace Stump, Night Tower, Skeletal Hand, Unicyclops |
| `ransom` | Blood Bath, Bums Away! |
| `upkeep` | Steakchop |
| `rebound` | Impossible Possum |
| `reveal` | Ogre Gas |
| `search` | Deforestation |
| `redirect` | Pied Piper |
| `defend` | Genteel Armourdillo, Gracious Armourdillo |
| `move_damage` | Friendship Bracelet |
| `move` | Boarder Collie, Drained Cleric, Tired Wombat, Woadic Ring Leader |
| `camel` | Carmel Camel |
| `landscape` | Silo of Freedom |
| `target` | Crystal Palace, Flameingo Hotshot, Frozen Fish, Resonant Spellhold, Scorchcrawler |
| `amount` | The Big Hen House |
| `play_pending` | Gallant Armourdillo, Silo of Truth |
| `offer (may)` | Apple Pieclops, Grand Mummy, Mossglen Retreat, Ms. Mummy, Sandmagus |

## 2. Targets chosen up front, and by triggers

The shapes of the targets of every card (the effect data's `targets`). `?` is optional, `*` several, `[group]` a
"one of" pair, `^` a slot that depends on an earlier one. The board-click flow (`picker.js` and `prompt.js`) asks all
of these, except the rows marked pop-up, which the engine asks mid-effect (D6).

| When it asks | Shape (what is chosen, in order) | Cards | Asked how |
|---|---|---|---|
| spell | `creature` | 25: Black Hole Pendant, Blue Candy, Brief Power, Broils Coils, Champion's Emblem, Corn Scepter, Drop Zone, Frozen Heart, Furious Furor, Ham Fist, Hot Shot, Molten Potion, Pentaid, Quick Pick Me Up, Ragepop Totem, Ring Of Fluffy, Sacred Scythe, Singing Sword, Subliminal Strength, Sword Bouquet, Tapped Out, Titanbane Amulet, Toilet Of Doom, Woad Talisman, Zazo's Magic Seeds | board (click, then confirm) |
| floop | `creature` | 12: Beach Mummy, Cottonpult, Darling Cobnoblin, Dr. Stuffenstein, Industrial Assassin, Mr. Slicer, Pyrovent Fortress, Sand Sphinx, Scorchstone Gateway, Slay Rider, The Mariachi, Travelin Skeleton | board (click, then confirm) |
| activate | `hand_card` | 8: Cave of Solitude, Dogboy, Dragon Foot, Herculeye, Infinite Figure, Pink Merwitch, Silo of Freedom, Squatting Bald Man | board (click, then confirm) |
| activate | `creature` | 6: Bag Ban-She Angel, Flameingo Burnwarden, Hot Wheel, Princess Bubblegum, Snow Baller, Uni-Knight | board (click, then confirm) |
| trigger: enter_play | `creature?` | 5: Chivalrous Armourdillo, Fire Dog, Legion of Earlings, Psionic Architect, Psionic Swashbuckler | board (click, then confirm) |
| floop | `creature + lane` | 4: Dragon Claw, Spirit Tower, Weary Trading Post, Woad Mobile Home | board (click, then confirm) |
| trigger: enter_play | `lane` | 4: Dragoon Parrotrooper, Husky Parrotrooper, Jinxed Parrotrooper, Static Parrotrooper | board (click, then confirm) |
| activate | `landscape` | 3: Frost Dragon, Snow Bunny, Snow Business | board (click, then confirm) |
| floop | `discard_card` | 3: Heavenly Gazer, Helping Hand, Red Eyeling | board (click, then confirm) |
| floop | `landscape` | 3: Big Foot, Icy Intruder, The Pig | board (click, then confirm) |
| spell | `creature + lane` | 3: Magenapping, Pants Of Awesome, Teleport | board (click, then confirm) |
| trigger: enter_play | `landscape` | 3: Husker Worm, Mantle Masher, Rock 'n Roller | board (click, then confirm) |
| floop | `building` | 2: Archer Dan, Archer Danica | board (click, then confirm) |
| floop | `creature?[mode] + creature?[mode]` | 2: Auto-Plucker, Precious Cobnoblin | board (click, then confirm) |
| spell | `creature + creature` | 2: Friendship Bracelet, Kernel Renewal | board (click, then confirm) |
| spell | `landscape` | 2: Freeze Ray, Rock Out! | board (click, then confirm) |
| spell | `lane` | 2: Deforestation, Snowblower | board (click, then confirm) |
| trigger: enter_play | `landscape?` | 2: Ban-She Princess, Ban-She Queen | board (click, then confirm) |
| activate | `hand_card + creature` | 1: BMO | board (click, then confirm) |
| additional_cost | `creature` | 1: Deforestation | board (click, then confirm) |
| additional_cost | `hand_card` | 1: Field Reaper | board (click, then confirm) |
| floop | `building + lane` | 1: Lumbercaddy | board (click, then confirm) |
| floop | `creature + creature` | 1: Dr. Death | board (click, then confirm) |
| floop | `creature*` | 1: Patchy the Pumpkin | board (click, then confirm) |
| floop | `creature?[mode] + creature?[mode] + creature` | 1: Cherished Cobnoblin | board (click, then confirm) |
| floop | `hand_card` | 1: Woadic Weirdo | board (click, then confirm) |
| floop | `hand_card + landscape` | 1: Sinkhole | board (click, then confirm) |
| floop | `lane` | 1: Flameingo Flameweaver | board (click, then confirm) |
| spell | `building + creature?` | 1: Volcano | board (click, then confirm) |
| spell | `creature?*` | 1: Cute Overload | board (click, then confirm) |
| spell | `creature?[mode] + creature?[mode]` | 1: Green Candy | board (click, then confirm) |
| spell | `discard_card + creature` | 1: Blood Transfusion | board (click, then confirm) |
| spell | `discard_card + lane` | 1: Harvest Moon | board (click, then confirm) |
| spell | `landscape + landscape` | 1: Snow Way | board (click, then confirm) |
| spell | `landscape? + building? + lane^` | 1: Reclaim Landscape | board (click, then confirm) |
| spell | `player` | 1: Frosty Frolic | board (click, then confirm) |
| trigger: attacks | `creature?` | 1: Flameingo Hotshot | pop-up (`target` choice) |
| trigger: creature_enters | `discard_card?` | 1: Resonant Spellhold | pop-up (`target` choice) |
| trigger: deals_creature_damage | `landscape?` | 1: Frozen Fish | pop-up (`target` choice) |
| trigger: enter_play | `creature` | 1: Sandsnake | board (click, then confirm) |
| trigger: enter_play | `creature + lane` | 1: Field Reaper | board (click, then confirm) |
| trigger: enter_play | `discard_card?` | 1: Grape Djinni | board (click, then confirm) |
| trigger: foe_burning_leaves | `creature?` | 1: Scorchcrawler | pop-up (`target` choice) |
| trigger: landscape_frozen | `creature` | 1: Crystal Palace | pop-up (`target` choice) |

## 3. Other places the client asks something

| Place | What it asks | Where | Follows the guide? | Fixed in |
|---|---|---|---|---|
| **Mulligan** | Keep this hand, or mulligan (once) | Buttons in an overlay: green "Keep", red "Mulligan" | B2 ok for Keep. **B4**: a mulligan is not irreversible or costly in the guide's sense; it is a choice (D5). No source card (nothing asks). | 2c |
| **End of main phase** | Fight (end the turn's actions) | Red "Fight!" in the hand bar | B4 ok (it cannot be taken back). | n/a |
| **End turn** | Nothing left to Fight | Green "End turn" | B2 ok. | n/a |
| **Play a card from your hand** | The lane and targets, then Confirm | Prompt bar: guidance, Skip, Cancel, Confirm | W4 ok. **V1**: Skip is labelled "None" or "Done" where the card has no `skip_label` (D2). **M1**: steps are not numbered (D7). | 2d |
| **What should this card do?** | One of several things a card could do (play, Floop...) | Prompt bar: purple buttons | B1 ok. | n/a |
| **Heal or damage?** (modes) | What to do to the chosen Creature | Prompt bar: purple buttons | B1 ok (M4). | n/a |
| **Pending free play** (Silo of Truth, Gallant Armourdillo) | The Lane for a card that must be played free | "Play {card}" purple button in the hand bar, then the board | W4 ok. **A1**: the card is shown in the bar. | n/a |
| **Offers** (`may`: Apple Pieclops, Grand Mummy, Mossglen Retreat, Ms. Mummy, Sandmagus) | An optional ability that lasts until you do something else | Extra purple ability buttons on the card, like a Floop | B1 ok. **F**: nothing says the offer will lapse (D8). | 2d |
| **Hero abilities** | Use the Hero's ability | Purple button by the Hero chip | B1 ok. | n/a |
| **Thaw a Landscape** | Discard a card to remove a frozen token | Ice-blue button on the Landscape, then pick the card | **B5**: its own colour and flow, and the only use of `btn-ice` (D9). | 2d |
| **Target a player** (Frosty Frolic) | Which player | "Choose yourself / your opponent" purple button on the player bar | W1 ok. | n/a |
| **Discard pile viewer** | Look at a pile; pick a card from it when a card asks | Pop-up (`pileModal`) | W5/I2 ok since #85. **V5**: closes with a steel "Close" here but a green "Close" in the practice dialog (D10). | 2c |
| **Practice dialog** | Pick a bot and decks | Dialog with a green "Close" and a green "Start" | **V5/B2/B3**: Close should be neutral steel (D10). | 2c |
| **Confirm summary** | What the card will do, when parts were skipped | Text in the prompt bar | A6 ok. | n/a |
| **Game over** | Back to the lobby | Green button | B2 ok. | n/a |

## 4. Deviations

Each is a candidate for a small issue, in the phase that fixes it.

| ID | Rule | The problem | Where | Phase |
|---|---|---|---|---|
| **D1** | A1 | A pop-up shows the name of the card that asked as text, never the card | `choiceModal`: every branch | 2a (the shared question dialog shows it) |
| **D2** | V1 | Skip and decline words: "None", "Choose nothing", "Leave it", "Let it go", "{name} fights", "Let it fight where it is going", "Take it instead" | `choiceModal`, `prompt.js`, `locales/en.json` | 2c |
| **D3** | A5 | Options that are bare numbers or verbs and do not say their result ("1", "Take it") | `amount`, `move_damage`, `ransom` | 2c |
| **D4** | W1 | Board objects chosen in a pop-up instead of on the board | `redirect`, `defend`, `move`, `target` | 2d |
| **D5** | B4 | Red means "stay put" (`redirect`) and "mulligan" where the guide reserves it for a cost or an attack | `redirect`, the mulligan overlay | 2c |
| **D6** | W1/W3 | Five cards ask mid-effect, by a trigger, where a board click would fit (Crystal Palace, Flameingo Hotshot, Frozen Fish, Resonant Spellhold, Scorchcrawler) | the `target` choice | 2d |
| **D7** | M1 | Steps of a multi-step card are not numbered or named | the prompt bar | 2d |
| **D8** | F | An offer (`may`) lapses when the player does something else, and nothing says so | the card's ability buttons | 2d |
| **D9** | B5 | Thaw is the only `btn-ice` and has its own flow | the Landscape's thaw button | 2d |
| **D10** | V5/B3 | "Close" is green in the practice dialog and steel in the pile viewer | `main.js`, `render.js` | 2c |
| **D11** | B7 | Spacing differs between pop-ups (`.modalrow` and `.modalrow2`, per-pop-up margins) | `style.css` | 2a |

Already filed and folded into the phases: **#79** (Reclaim Landscape's chained skips, done), **#85** (Blood
Transfusion's piles, done), **#87** (The Big Hen House's pop-up has no card context and touching buttons: D1, D11),
**#84** (deck editing, done).

## 5. Not decided yet

Left for the phase that needs it (each is a note on its issue before work starts):

- Which choice kinds show a result on their options (A5): the audit suggests `amount`, `move_damage`, `ransom` and the
  `target` choices, as their results are known to the client. (#108)
- Whether the mulligan's "Mulligan" button keeps the danger role (D5) or becomes a plain choice. (#108)
- How a mid-effect board choice is shown when the board is mostly hidden by the opponent's turn banner. (#108, 2d)
