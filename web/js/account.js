// Who is signed in, and how to sign in. Pure: the fetching is in main.js.
//
// The server says what it offers (GET /api/auth/config) and who the player is
// (GET /api/auth/me). Signing in is a full-page trip to the provider and back,
// so there is nothing to keep here between visits: the cookie holds it.

const REASONS = {
  denied: "Sign-in was cancelled.",
  state: "That sign-in expired or came from somewhere else. Please try again.",
  failed: "The sign-in did not work. Please try again.",
};

/** The address that starts signing in with a provider and brings the player back to this page. */
export function loginUrl(server, provider, here) {
  return `${server}/api/auth/login/${encodeURIComponent(provider)}?return=${encodeURIComponent(here)}`;
}

/**
 * What the lobby shows for the account.
 * @param {{mode?:string, providers?:{name:string,label:string}[]}|null} config
 * @param {{user:{id:string,name:string}|null}|null} me
 * @returns {{show:boolean, user:{id:string,name:string}|null, providers:{name:string,label:string}[]}}
 */
export function accountView(config, me) {
  if (config?.mode !== "oauth" || !config.providers?.length) return { show: false, user: null, providers: [] };
  return { show: true, user: me?.user ?? null, providers: config.providers };
}

/** The message for a ?login_error=... the server sent the player back with, or "". */
export function loginErrorText(search) {
  const code = new URLSearchParams(search).get("login_error");
  return code ? REASONS[code] ?? REASONS.failed : "";
}

/** The address with the login_error removed, so a reload does not show it again. */
export function withoutLoginError(href) {
  const u = new URL(href);
  u.searchParams.delete("login_error");
  return u.pathname + (u.search || "") + u.hash;
}
