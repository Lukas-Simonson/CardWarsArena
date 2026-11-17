# Sound credits

Every sound in this folder is from a pack by [Kenney](https://kenney.nl) (Kenney Vleugels), released under
[Creative Commons Zero (CC0 1.0)](https://creativecommons.org/publicdomain/zero/1.0/): free to use, change and
share, including in a game, with no credit required (it is given here anyway). The licence text each pack came with is in
[licenses/](licenses/).

A file's name starts with the pack it came from, then its name in the pack: `casino-card-slide-1` is `card-slide-1`
from Casino Audio. Each sound is here twice, as `.ogg` (Vorbis, the original file, unchanged) and `.m4a` (AAC, made
from it with `afconvert -f m4af -d aac -b 64000`) for browsers that do not play Ogg. Which files each cue uses is in
[`../js/sounds.js`](../js/sounds.js).

| Prefix | Pack | Author | Source | Licence | Licence text |
|---|---|---|---|---|---|
| casino | Casino Audio | Kenney | https://kenney.nl/assets/casino-audio | CC0 1.0 | licenses/casino-audio.txt |
| impact | Impact Sounds | Kenney | https://kenney.nl/assets/impact-sounds | CC0 1.0 | licenses/impact-sounds.txt |
| interface | Interface Sounds | Kenney | https://kenney.nl/assets/interface-sounds | CC0 1.0 | licenses/interface-sounds.txt |
| rpg | RPG Audio | Kenney | https://kenney.nl/assets/rpg-audio | CC0 1.0 | licenses/rpg-audio.txt |
| ui | UI Audio | Kenney | https://kenney.nl/assets/ui-audio | CC0 1.0 | licenses/ui-audio.txt |

## Adding a sound

Use only sounds that are original, made by you, or released under a licence that allows this use (CC0, or CC-BY with a
credit here). **Do not use sounds taken from Adventure Time, Card Wars or any other show or game.** Add the pack or
author to the table above with its source and licence, keep the licence text in `licenses/`, give the file a name that
starts with the table's prefix, and make both an `.ogg` and an `.m4a`. `npm test` checks all of this.
