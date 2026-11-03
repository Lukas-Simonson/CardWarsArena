// Fetching card art before it is needed. The server's "preload" list names the
// cards in the opponent's hand and on top of your deck, mixed together, so the
// art of a card they play or one you draw is already in the cache.

const started = new Map(); // url -> Image, kept so a slow load is not dropped

/** The image urls in `preload` that have not been fetched yet and that the catalog knows. */
export function artToPreload(preload, cards, have = started) {
  const urls = [];
  for (const id of preload ?? []) {
    const url = cards?.[id]?.image_url;
    if (url && !have.has(url) && !urls.includes(url)) urls.push(url);
  }
  return urls;
}

/**
 * Start fetching the art for the ids in `preload`. Returns the urls it started.
 * Does nothing when the browser asks to save data.
 */
export function preloadArt(preload, cards, { create = defaultCreate, saveData = globalThis.navigator?.connection?.saveData } = {}) {
  if (saveData) return [];
  const urls = artToPreload(preload, cards);
  for (const url of urls) started.set(url, create(url));
  return urls;
}

function defaultCreate(url) {
  const img = new Image();
  img.decoding = "async";
  img.src = url;
  return img;
}

/** Forget what was fetched (tests). */
export const resetPreload = () => started.clear();
