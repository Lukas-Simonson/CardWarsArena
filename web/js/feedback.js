// The links to the project's feedback forms and repository, as GET /api/feedback says. Pure: no DOM, no network.

/**
 * The links to show: none unless the server says `enabled`, and only web addresses (https) with a label,
 * so an odd answer can never put a script or another kind of link on the page.
 */
export function feedbackLinks(answer) {
  if (!answer?.enabled || !Array.isArray(answer.links)) return [];
  return answer.links.filter((l) => typeof l?.label === "string" && l.label.trim() && typeof l.url === "string" && /^https:\/\//i.test(l.url))
    .map((l) => ({ id: String(l.id ?? l.label), label: l.label.trim(), url: l.url }));
}
