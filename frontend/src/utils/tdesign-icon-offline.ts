/**
 * Block the iconfont requests that tdesign-icons-vue-next sends to the external CDN.
 *
 * Background (issue #867 / #897):
 *   The Icon / IconFont components of tdesign-icons-vue-next use
 *   `checkScriptAndLoad` / `checkLinkAndLoad` in onMounted to inject into document a
 *   <script> / <link> pointing at `https://tdesign.gtimg.com/icon/<version>/fonts/index.(js|css)`.
 *   Without internet access the request fails and no icon renders.
 *
 * Fix: before the Vue app mounts, insert placeholder nodes that match tdesign's matching rules,
 *   so the dedupe check in `checkScriptAndLoad` / `checkLinkAndLoad` hits and returns early
 *   without appending real nodes that point at the CDN.
 *
 *   - Source of the matching selectors: tdesign-icons-vue-next/esm/utils/check-url-and-load.js
 *       `.t-svg-js-stylesheet--unique-class[src="<url>"]`
 *       `.t-iconfont-stylesheet--unique-class[href="<url>"]`
 *   - The placeholder <script> / <link> use a non-standard type / rel, so the browser sends no network request.
 *
 * The SVG sprite symbols are registered locally up front by <script src="/tdesign-icons/.../index.js"></script> in index.html,
 * so <t-icon name="..."> still renders.
 */

const SVG_SCRIPT_CLASS = "t-svg-js-stylesheet--unique-class";
const ICONFONT_LINK_CLASS = "t-iconfont-stylesheet--unique-class";

// Matches the URLs hard-coded inside tdesign-icons-vue-next 0.4.x; several versions side by side cover a possible upgrade.
const BLOCKED_ICON_VERSIONS = ["0.4.0", "0.4.1", "0.4.2", "0.4.3", "0.4.4", "0.4.5"];

const BLOCKED_SCRIPT_URLS = BLOCKED_ICON_VERSIONS.map(
  (version) => `https://tdesign.gtimg.com/icon/${version}/fonts/index.js`,
);

const BLOCKED_LINK_URLS = BLOCKED_ICON_VERSIONS.map(
  (version) => `https://tdesign.gtimg.com/icon/${version}/fonts/index.css`,
);

let installed = false;

export function installTDesignIconOfflineGuard(): void {
  if (installed || typeof document === "undefined") return;
  installed = true;

  const body = document.body;
  if (!body) {
    document.addEventListener(
      "DOMContentLoaded",
      () => installTDesignIconOfflineGuard(),
      { once: true },
    );
    installed = false;
    return;
  }

  BLOCKED_SCRIPT_URLS.forEach((src) => {
    const exists = document.querySelector(
      `script.${SVG_SCRIPT_CLASS}[src="${src}"]`,
    );
    if (exists) return;
    const stub = document.createElement("script");
    stub.setAttribute("class", SVG_SCRIPT_CLASS);
    stub.setAttribute("src", src);
    // A non-standard MIME type makes the browser skip fetching and running the script
    stub.setAttribute("type", "text/no-load");
    stub.setAttribute("data-ai-kb-blocked-cdn", "tdesign-icons");
    body.appendChild(stub);
  });

  BLOCKED_LINK_URLS.forEach((href) => {
    const exists = document.querySelector(
      `link.${ICONFONT_LINK_CLASS}[href="${href}"]`,
    );
    if (exists) return;
    const stub = document.createElement("link");
    stub.setAttribute("class", ICONFONT_LINK_CLASS);
    stub.setAttribute("href", href);
    // Without rel="stylesheet" the browser sends no stylesheet request
    stub.setAttribute("rel", "preload-blocked");
    stub.setAttribute("data-ai-kb-blocked-cdn", "tdesign-icons");
    document.head.appendChild(stub);
  });
}
