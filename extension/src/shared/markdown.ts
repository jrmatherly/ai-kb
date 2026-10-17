import DOMPurify from 'dompurify';
import { Marked } from 'marked';

// Same list and shapes as frontend/src/utils/protectedFileAccess.ts (PROVIDER_SCHEME_PATTERN); keep them in step.
const PROVIDER_SCHEMES = 'resource|local|minio|cos|tos|s3|oss|ks3|obs';
const STORAGE_REF = new RegExp(`^(?:(?:${PROVIDER_SCHEMES}):\\/\\/\\S+|storage:\\/\\/[0-9A-Za-z_-]+\\/(?:${PROVIDER_SCHEMES}):\\/\\/\\S+)$`, 'i');
const REMOTE = /^https?:\/\//i;
const INLINE = /^data:image\//i;

const marked = new Marked({ gfm: true, breaks: true });

// Pages only: the worker has no window and never renders Markdown.
const purifier = DOMPurify(window);
// What marked emits plus common inline HTML. Nothing here fetches on render except img, which the policy
// below gates; style, srcset, poster, background and data attributes stay out for the same reason.
purifier.setConfig({
  ALLOWED_TAGS: ['a', 'b', 'blockquote', 'br', 'code', 'del', 'details', 'div', 'em', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'hr', 'i', 'img', 'input', 'kbd', 'li', 'mark', 'ol', 'p', 'pre', 's', 'span', 'strong', 'sub', 'summary', 'sup', 'table', 'tbody', 'td', 'tfoot', 'th', 'thead', 'tr', 'u', 'ul'],
  ALLOWED_ATTR: ['align', 'alt', 'checked', 'class', 'disabled', 'href', 'open', 'src', 'start', 'title', 'type'],
  ALLOW_DATA_ATTR: false,
  RETURN_DOM: true,
});
// The URI filter would strip the storage schemes, which the panel resolves itself.
purifier.addHook('uponSanitizeAttribute', (node, data) => {
  if (data.attrName === 'src' && node.nodeName === 'IMG' && STORAGE_REF.test(data.attrValue)) data.forceKeepAttr = true;
});
purifier.addHook('afterSanitizeAttributes', (node) => {
  if (node.tagName === 'A' && node.hasAttribute('href') && !node.getAttribute('href')?.startsWith('#')) {
    node.setAttribute('target', '_blank');
    node.setAttribute('rel', 'noopener');
  }
});

// Runs on the sanitized tree, so the buttons it makes are the only ones and data attributes are its own.
function imagePolicy(node: Element): void {
  const src = node.getAttribute('src') ?? '';
  if (STORAGE_REF.test(src)) {
    node.setAttribute('data-storage-ref', src);
    node.setAttribute('src', '');
    if (!node.getAttribute('alt')) node.setAttribute('alt', 'loading image');
    return;
  }
  if (INLINE.test(src)) return;
  const url = REMOTE.test(src) && URL.canParse(src) ? new URL(src) : null;
  if (!url) {
    node.replaceWith(node.getAttribute('alt') ?? '');
    return;
  }
  const button = node.ownerDocument.createElement('button');
  button.setAttribute('type', 'button');
  button.className = 'aikb-remote-image';
  button.dataset.src = src;
  button.textContent = `Load image from ${url.host}`;
  node.replaceWith(button);
}

export function renderMarkdown(md: string): string {
  const body = purifier.sanitize(marked.parse(md, { async: false })) as unknown as HTMLElement;
  // Only an img may carry a src: an input of type image would load it.
  for (const input of body.querySelectorAll('input:not([type="checkbox"])')) input.remove();
  for (const img of body.querySelectorAll('img')) imagePolicy(img);
  return body.innerHTML;
}
