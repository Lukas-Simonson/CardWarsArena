import { test } from "node:test";
import assert from "node:assert/strict";
import { loginUrl, accountView, loginErrorText, withoutLoginError } from "../js/account.js";

const providers = [{ name: "discord", label: "Discord" }, { name: "github", label: "GitHub" }];

test("the login address names the provider and where to come back to", () => {
  assert.equal(
    loginUrl("https://cards.example.com", "github", "https://cards.example.com/"),
    "https://cards.example.com/api/auth/login/github?return=https%3A%2F%2Fcards.example.com%2F",
  );
  assert.match(loginUrl("http://localhost:8080", "a/b", "http://localhost:5173/"), /\/login\/a%2Fb\?/);
});

test("nothing is offered unless the server signs people in with some provider", () => {
  assert.equal(accountView(null, null).show, false);
  assert.equal(accountView({ mode: "none", providers: [] }, { user: null }).show, false);
  assert.equal(accountView({ mode: "oauth", providers: [] }, { user: null }).show, false);
});

test("signed out offers the providers; signed in shows the account", () => {
  const out = accountView({ mode: "oauth", providers }, { user: null });
  assert.deepEqual([out.show, out.user, out.providers.length], [true, null, 2]);
  const inn = accountView({ mode: "oauth", providers }, { user: { id: "u1", name: "Ada" } });
  assert.deepEqual([inn.show, inn.user?.name], [true, "Ada"]);
  assert.equal(accountView({ mode: "oauth", providers }, null).user, null); // /me unreachable: treated as signed out
});

test("sign-in errors have words, and are removed from the address", () => {
  assert.match(loginErrorText("?login_error=denied"), /cancelled/);
  assert.match(loginErrorText("?login_error=state"), /expired/);
  assert.match(loginErrorText("?login_error=whatever"), /did not work/);
  assert.equal(loginErrorText(""), "");
  assert.equal(loginErrorText("?server=x"), "");
  assert.equal(withoutLoginError("http://localhost:5173/?login_error=denied&server=x#a"), "/?server=x#a");
  assert.equal(withoutLoginError("http://localhost:5173/?login_error=denied"), "/");
});
