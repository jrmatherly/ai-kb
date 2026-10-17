import Defuddle from 'defuddle/full';
import type { CaptureDraft } from '../shared/types';

const OVERLAY_ID = 'aikb-region-picker';

// The full build is UMD: at runtime the named export is a static on the default export.
const createMarkdownContent = (Defuddle as unknown as { createMarkdownContent: (html: string, url: string) => string }).createMarkdownContent;

export function htmlToMarkdown(html: string, url = ''): string {
  return createMarkdownContent(html, url).trim();
}

// Defuddle clones internally and reads the live window for its hidden-element, media-query and
// shadow-DOM passes, so it gets the live document; it only normalizes attributes on the way in.
export function extractArticle(doc: Document): CaptureDraft {
  const url = doc.location.href;
  const result = new Defuddle(doc, { url, markdown: true }).parse();
  return {
    kind: 'article',
    title: (result.title || doc.title).trim(),
    url,
    markdown: result.content.trim(),
    meta: { author: result.author, published: result.published, site: result.site },
  };
}

const selectDraft = (doc: Document, markdown: string): CaptureDraft =>
  ({ kind: 'select', title: doc.title.trim(), url: doc.location.href, markdown, meta: {} });

export type RectsFor = (node: Text) => DOMRect[];

function defaultRectsFor(doc: Document): RectsFor {
  const range = doc.createRange();
  return (node) => {
    range.selectNodeContents(node);
    return Array.from(range.getClientRects());
  };
}

function selectionHtml(doc: Document): string | null {
  const selection = doc.getSelection();
  if (!selection || selection.rangeCount === 0 || selection.isCollapsed) return null;
  const holder = doc.createElement('div');
  for (let i = 0; i < selection.rangeCount; i++) holder.appendChild(selection.getRangeAt(i).cloneContents());
  return holder.innerHTML;
}

export async function extractSelect(doc: Document, rectsFor?: RectsFor): Promise<CaptureDraft | null> {
  const html = selectionHtml(doc);
  if (html !== null) return selectDraft(doc, htmlToMarkdown(html, doc.location.href));
  return startRegionPicker(doc, rectsFor);
}

type Box = { left: number; top: number; right: number; bottom: number };

const intersects = (a: Box, r: DOMRect) => r.left < a.right && r.right > a.left && r.top < a.bottom && r.bottom > a.top;

const SKIPPED = new Set(['SCRIPT', 'STYLE', 'NOSCRIPT', 'TEMPLATE']);
const BLOCK = new Set(['ADDRESS', 'ARTICLE', 'ASIDE', 'BLOCKQUOTE', 'BODY', 'DD', 'DETAILS', 'DIV', 'DL', 'DT', 'FIELDSET', 'FIGCAPTION', 'FIGURE', 'FOOTER', 'FORM', 'H1', 'H2', 'H3', 'H4', 'H5', 'H6', 'HEADER', 'HR', 'LI', 'MAIN', 'NAV', 'OL', 'P', 'PRE', 'SECTION', 'SUMMARY', 'TABLE', 'TD', 'TH', 'TR', 'UL']);

const blockOf = (node: Text): Node => {
  let el = node.parentElement;
  while (el && !BLOCK.has(el.tagName)) el = el.parentElement;
  return el ?? node.ownerDocument;
};

// Text nodes whose boxes touch the rectangle, in document order, one line per block ancestor.
function textInBox(doc: Document, box: Box, rectsFor: RectsFor): string {
  const walker = doc.createTreeWalker(doc.body, 5 /* SHOW_ELEMENT | SHOW_TEXT */, {
    acceptNode: (node) => {
      if (node.nodeType === 1) return SKIPPED.has((node as Element).tagName) || (node as Element).id === OVERLAY_ID ? 2 /* REJECT */ : 3 /* SKIP */;
      return 1 /* ACCEPT */;
    },
  });
  const lines: string[] = [];
  let lastBlock: Node | null = null;
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    const text = node as Text;
    const value = text.data.replace(/\s+/g, ' ').trim();
    if (!value || !rectsFor(text).some((r) => intersects(box, r))) continue;
    const block = blockOf(text);
    if (block === lastBlock) lines[lines.length - 1] += ` ${value}`;
    else lines.push(value);
    lastBlock = block;
  }
  return lines.join('\n').replace(/ ([.,;:!?])/g, '$1');
}

let cancelActivePicker: (() => void) | null = null;

// Drag to select a rectangle; mouseup confirms, Escape cancels. The overlay is removed on every exit.
export function startRegionPicker(doc: Document, rectsFor: RectsFor = defaultRectsFor(doc)): Promise<CaptureDraft | null> {
  cancelActivePicker?.();
  if (!doc.body) return Promise.resolve(null);
  return new Promise((resolve) => {
    const overlay = doc.createElement('div');
    overlay.id = OVERLAY_ID;
    overlay.setAttribute('style', 'position:fixed;inset:0;z-index:2147483647;cursor:crosshair;background:rgba(0,0,0,0.08)');
    const box = doc.createElement('div');
    box.setAttribute('style', 'position:fixed;border:2px solid #2d6cdf;background:rgba(45,108,223,0.12);display:none;pointer-events:none');
    overlay.appendChild(box);

    let origin: { x: number; y: number } | null = null;

    const finish = (draft: CaptureDraft | null) => {
      doc.removeEventListener('keydown', onKey, true);
      doc.removeEventListener('mouseup', onUp, true);
      overlay.remove();
      cancelActivePicker = null;
      resolve(draft);
    };
    cancelActivePicker = () => finish(null);

    const onKey = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      event.preventDefault();
      finish(null);
    };
    const update = (x: number, y: number): Box | null => {
      if (!origin) return null;
      const current = { left: Math.min(origin.x, x), top: Math.min(origin.y, y), right: Math.max(origin.x, x), bottom: Math.max(origin.y, y) };
      box.style.display = 'block';
      box.style.left = `${current.left}px`;
      box.style.top = `${current.top}px`;
      box.style.width = `${current.right - current.left}px`;
      box.style.height = `${current.bottom - current.top}px`;
      return current;
    };
    // Listened on the document so a release outside the viewport still ends the drag.
    const onUp = (event: MouseEvent) => {
      const current = update(event.clientX, event.clientY);
      const text = current ? textInBox(doc, current, rectsFor) : '';
      finish(text ? selectDraft(doc, text) : null);
    };

    overlay.addEventListener('mousedown', (event) => {
      event.preventDefault();
      origin = { x: event.clientX, y: event.clientY };
      update(event.clientX, event.clientY);
    });
    overlay.addEventListener('mousemove', (event) => update(event.clientX, event.clientY));
    doc.addEventListener('mouseup', onUp, true);
    doc.addEventListener('keydown', onKey, true);
    doc.body.appendChild(overlay);
  });
}
