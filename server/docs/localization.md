# Translating the card data

The card catalogue (`data/cards`, written by `cardimport` from `data/CardWarsCatalogue.json`) is the English source. A
language lives apart from it, in `data/i18n/<tag>/`, so re-importing never overwrites a translation. The rules
(`data/effects`) are language-neutral and are not translated.

```
data/i18n/fr/cards.json   { "finn": { "name": "…", "text": "…", "notes": "…", "clarification": "…", "image_url": "…", "source": "ab12cd34ef56" } }
data/i18n/fr/decks.json   { "Finn Vs Jake|Finn's Deck": { "title": "…", "owner": "…", "set": "…" } }
```

- A field left out stays English, so a language can be translated a card at a time. A card with no entry is English.
- `fr-CA` is looked up as `fr-CA`, then `fr`, then English. The English catalogue is what `en` and `en-GB` get.
- `source` is a fingerprint of the English name, text, flavour text and ruling the translation was written for. When the English
  changes (a new import), `cardi18n status` lists the card as out of date; the translation is still used until someone updates it.
- `en_name` and `en_text` are the English words, for the translator to read. Nothing uses them.
- Decks are named by `Set|Deck` as they are in the catalogue's printings. The deck's English name stays its identity (it is what a
  client asks to play); `title`, `owner` and `set` are only what is shown.
- Unknown fields are errors when the server starts, so a typo cannot silently lose a translation.

## Starting or extending a language

```
go run ./cmd/cardi18n init fr      # creates data/i18n/fr/, or adds the cards and decks it does not have yet
go run ./cmd/cardi18n status       # how complete each language is, what is out of date, what is for a card that is gone
```

`init` writes a skeleton entry for each card (empty `name` and `text`, the English beside them) and never touches an entry
that is already there, so run it again after an import brings new cards. `status` exits 1 when a language has a translation
for a card that no longer exists (the tests check what ships the same way). A card counts as translated when it has a `name` or
`text`.

## How a client gets it

`GET /api/cards?lang=fr-CA` and `GET /api/decks?lang=fr-CA` answer in that language where there are translations; see
[protocol.md](protocol.md). The web client asks for its own language, so choosing French shows the interface (see `web/README.md`)
and the cards in French. `name_en` carries the English name of a translated card, so a deck list can be written and read in English
whatever language is shown, and the deck builder's search finds a card by either name.

Card images are pictures of the English cards. A translation may point `image_url` at a scan in its language.
