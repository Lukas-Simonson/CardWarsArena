import { test } from "node:test";
import assert from "node:assert/strict";
import { feedbackLinks } from "../js/feedback.js";

const links = [{ id: "bug", label: "Report a bug", url: "https://example.com/bug" }, { id: "repo", label: "GitHub", url: "https://example.com/repo" }];

test("the links show only when the server turns them on", () => {
  assert.deepEqual(feedbackLinks({ enabled: true, links }).map((l) => l.id), ["bug", "repo"]);
  assert.deepEqual(feedbackLinks({ enabled: false, links }), []);
  assert.deepEqual(feedbackLinks(null), []);
  assert.deepEqual(feedbackLinks({ enabled: true }), []);
});

test("only labelled https links are kept", () => {
  const bad = [{ label: "x", url: "javascript:alert(1)" }, { label: "", url: "https://example.com" }, { label: "y", url: "http://example.com" }, null, { label: "ok", url: "https://example.com" }];
  assert.deepEqual(feedbackLinks({ enabled: true, links: bad }).map((l) => l.label), ["ok"]);
});
