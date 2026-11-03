import { test, beforeEach } from "node:test";
import assert from "node:assert/strict";
import { artToPreload, preloadArt, resetPreload } from "../js/preload.js";

const cards = {
  "the-pig": { id: "the-pig", image_url: "https://img/the-pig.webp" },
  "cool-dog": { id: "cool-dog", image_url: "https://img/cool-dog.webp" },
  "same-art": { id: "same-art", image_url: "https://img/the-pig.webp" },
  "no-art": { id: "no-art" },
};

beforeEach(resetPreload);

test("every hinted card with art is fetched once", () => {
  const made = [];
  const create = (url) => (made.push(url), { url });
  assert.deepEqual(preloadArt(["cool-dog", "the-pig", "same-art"], cards, { create, saveData: false }), ["https://img/cool-dog.webp", "https://img/the-pig.webp"]);
  assert.deepEqual(preloadArt(["the-pig", "cool-dog"], cards, { create, saveData: false }), [], "already started");
  assert.equal(made.length, 2);
});

test("cards the catalog does not know, or without art, are skipped and retried later", () => {
  const create = (url) => ({ url });
  assert.deepEqual(preloadArt(["ghost", "no-art", undefined].filter(Boolean), cards, { create, saveData: false }), []);
  assert.deepEqual(preloadArt(["the-pig"], {}, { create, saveData: false }), [], "no catalog yet");
  assert.deepEqual(preloadArt(["the-pig"], cards, { create, saveData: false }), ["https://img/the-pig.webp"], "fetched once the catalog is there");
});

test("a missing list or the save-data setting fetches nothing", () => {
  const create = () => assert.fail("should not fetch");
  assert.deepEqual(preloadArt(undefined, cards, { create, saveData: false }), []);
  assert.deepEqual(preloadArt(["the-pig"], cards, { create, saveData: true }), []);
  assert.deepEqual(artToPreload(["the-pig"], cards, new Set(["https://img/the-pig.webp"])), []);
});
