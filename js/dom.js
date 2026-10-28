// A tiny element builder: h("div", {class: "x", onclick: fn}, child, [children], "text").
export function h(tag, attrs = {}, ...kids) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (v == null || v === false) continue;
    if (k === "class") el.className = v;
    else if (k.startsWith("on")) el.addEventListener(k.slice(2), v);
    else el.setAttribute(k, v === true ? "" : v);
  }
  for (const kid of kids.flat()) if (kid != null && kid !== false) el.append(kid);
  return el;
}

const SVG = "http://www.w3.org/2000/svg";

/** An inline SVG from markup. The markup is always a literal in our own source. */
export function svg(markup, attrs = {}) {
  const el = document.createElementNS(SVG, "svg");
  for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v);
  el.setAttribute("aria-hidden", "true");
  el.innerHTML = markup;
  return el;
}
