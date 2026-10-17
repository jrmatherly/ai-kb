import DOMPurify from 'dompurify';
import { Marked } from 'marked';

// Same list and shapes as frontend/src/utils/protectedFileAccess.ts (PROVIDER_SCHEME_PATTERN); keep them in step.
const PROVIDER_SCHEMES = 'resource|local|minio|cos|tos|s3|oss|ks3|obs';
const STORAGE_REF = new RegExp(`^(?:(?:${PROVIDER_SCHEMES}):\\/\\/\\S+|storage:\\/\\/[0-9A-Za-z_-]+\\/(?:${PROVIDER_SCHEMES}):\\/\\/\\S+)$`, 'i');
const REMOTE = /^https?:\/\//i;
const INLINE = /^data:image\//i;

const marked = new Marked({ gfm: true, breaks: true });

// Every image, from Markdown or raw HTML, passes through the sanitizer, so the policy lives in one hook
// that runs before URI filtering strips the storage schemes.
function imagePolicy(node: Element): void {
  const src = node.getAttribute('src') ?? '';
  if (STORAGE_REF.test(src)) {
    node.setAttribute('data-storage-ref', src);
    node.setAttribute('src', '');
    if (!node.getAttribute('alt')) node.setAttribute('alt', 'loading image');
    return;
  }
  if (INLINE.test(src)) return;
  const doc = node.ownerDocument;
  if (!REMOTE.test(src)) {
    node.replaceWith(doc.createTextNode(node.getAttribute('alt') ?? ''));
    return;
  }
  const button = doc.createElement('button');
  button.setAttribute('type', 'button');
  button.className = 'aikb-remote-image';
  button.dataset.src = src;
  button.textContent = `Load image from ${new URL(src).host}`;
  node.replaceWith(button);
}

// Pages only: the worker has no window and never renders Markdown.
const purifier = DOMPurify(window);
purifier.addHook('uponSanitizeElement', (node, data) => {
  if (data.tagName === 'img') imagePolicy(node as Element);
});
purifier.addHook('afterSanitizeAttributes', (node) => {
  if (node.tagName === 'A' && node.hasAttribute('href')) {
    node.setAttribute('target', '_blank');
    node.setAttribute('rel', 'noopener');
  }
});

export function renderMarkdown(md: string): string {
  return purifier.sanitize(marked.parse(md, { async: false }));
}
