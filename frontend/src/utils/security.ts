/**
 * Security utilities - XSS prevention
 */

import DOMPurify from 'dompurify';
import { applyProtectedFile, responseFileName, RESOURCE_PREVIEW_EVENT, type LoadedProtectedFile } from './protectedResource.ts';
import type { Config, NodeHook } from 'dompurify';
import {
  domPurifySecurityHooks,
  domPurifySecurityOptions,
  markdownDomPurifyConfig,
  markdownDomPurifySecurityHooks,
} from './markdownDomPurify.ts';
import {
  buildProtectedFileRequest,
  isProtectedFileProxyPath,
  isProviderFileURL,
  PROVIDER_SCHEME_PATTERN,
  resolveProtectedFileAccess,
  type ProtectedFileAccessContext,
} from './protectedFileAccess.ts';

const PROVIDER_IMAGE_PLACEHOLDER = 'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw==';
const PROVIDER_IMG_SRC_RE = new RegExp(
  `<img\\b([^>]*?)\\ssrc=(["'])(${PROVIDER_SCHEME_PATTERN}):(?:\\/\\/|&#x2f;&#x2f;|&#47;&#47;)([^"']+)\\2([^>]*)>`,
  'gi',
);
const STORAGE_BACKEND_IMG_SRC_RE = new RegExp(
  `<img\\b([^>]*?)\\ssrc=(["'])storage:\\/\\/([0-9A-Za-z_-]+)\\/(${PROVIDER_SCHEME_PATTERN}):(?:\\/\\/|&#x2f;&#x2f;|&#47;&#47;)([^"']+)\\2([^>]*)>`,
  'gi',
);

type SecurityHooks = {
  beforeSanitizeElements: NodeHook;
  afterSanitizeElements: NodeHook;
};

const DOCUMENT_PREVIEW_IMAGE_ATTRS = ['loading', 'decoding', 'fetchpriority'] as const;

function sanitizeWithSecurityHooks(
  html: string,
  config: Config,
  hooks: SecurityHooks,
): string {
  DOMPurify.addHook('beforeSanitizeElements', hooks.beforeSanitizeElements);
  DOMPurify.addHook('afterSanitizeElements', hooks.afterSanitizeElements);
  try {
    return DOMPurify.sanitize(html, config);
  } finally {
    DOMPurify.removeHook('afterSanitizeElements', hooks.afterSanitizeElements);
    DOMPurify.removeHook('beforeSanitizeElements', hooks.beforeSanitizeElements);
  }
}

// DOMPurify security policy
const DOMPurifyConfig = {
  // Allowed tags
  ALLOWED_TAGS: [
    'p', 'br', 'strong', 'em', 'u', 's', 'del', 'ins',
    'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
    'ul', 'ol', 'li', 'blockquote', 'pre', 'code',
    'a', 'img', 'table', 'thead', 'tbody', 'tr', 'th', 'td',
    'div', 'span', 'figure', 'figcaption', 'details', 'summary', 'think', 'button',
    // Tags Mermaid SVG needs
    'svg', 'g', 'path', 'rect', 'circle', 'ellipse', 'line', 'polygon',
    'polyline', 'text', 'tspan', 'defs', 'marker', 'filter', 'use',
    'clippath', 'lineargradient', 'radialgradient', 'stop', 'pattern',
    'image', 'foreignobject', 'desc', 'title', 'switch', 'symbol', 'mask',
    // Tags KaTeX MathML needs
    'math', 'annotation', 'semantics', 'mo', 'mi', 'mn', 'msup', 'mrow', 'mfrac', 'msqrt', 'mroot', 'mstyle'
  ],
  // Allowed attributes
  ALLOWED_ATTR: [
    'href', 'title', 'alt', 'src', 'class', 'id', 'style', 'data-protected-src', 'data-img-loading',
    'data-artifact-index', 'data-protected-resource', 'download',
    'target', 'rel', 'width', 'height', 'open',
    'type', 'aria-label', 'disabled', 'role', 'tabindex',
    // Attributes Mermaid SVG needs
    'd', 'fill', 'stroke', 'stroke-width', 'stroke-linecap', 'stroke-linejoin',
    'stroke-dasharray', 'stroke-dashoffset', 'stroke-miterlimit', 'stroke-opacity',
    'fill-opacity', 'opacity', 'transform', 'viewbox', 'preserveaspectratio',
    'x', 'y', 'x1', 'y1', 'x2', 'y2', 'cx', 'cy', 'rx', 'ry', 'r',
    'dx', 'dy', 'text-anchor', 'dominant-baseline', 'font-family', 'font-size',
    'font-weight', 'font-style', 'letter-spacing', 'word-spacing',
    'marker-start', 'marker-mid', 'marker-end', 'markerunits', 'markerwidth',
    'markerheight', 'refx', 'refy', 'orient', 'points', 'offset',
    'gradientunits', 'gradienttransform', 'spreadmethod', 'stop-color', 'stop-opacity',
    'patternunits', 'patterntransform', 'clippathunits', 'maskunits',
    'filterunits', 'primitiveunits', 'xmlns', 'xmlns:xlink', 'xlink:href',
    'version', 'baseprofile', 'enable-background', 'overflow', 'visibility',
    'display', 'pointer-events', 'cursor', 'data-emit', 'direction',
    // Attributes KaTeX MathML needs
    'mathvariant', 'encoding', 'aria-hidden'
  ],
  USE_PROFILES: { html: true, svg: true, mathMl: true },
  // USE_PROFILES replaces ALLOWED_ATTR wholesale, and the html profile does not include target,
  // so the target="_blank" that the afterSanitizeElements hook adds is stripped in the following
  // _sanitizeAttributes. ADD_ATTR appends the attribute on top of the profile.
  ADD_ATTR: ['target'],
  ...domPurifySecurityOptions,
};

/**
 * Sanitize HTML content
 * @param html HTML string to sanitize
 * @returns Sanitized, safe HTML string
 */
export function sanitizeHTML(html: string): string {
  if (!html || typeof html !== 'string') {
    return '';
  }
  
  try {
    const preparedHTML = protectProviderImageSrcInHTML(html);
    return sanitizeWithSecurityHooks(
      preparedHTML,
      DOMPurifyConfig as unknown as Config,
      domPurifySecurityHooks,
    );
  } catch (error) {
    console.error('HTML sanitization failed:', error);
    // If sanitizing fails, return escaped plain text
    return escapeHTML(html);
  }
}

export function applyDocumentPreviewImageAttributes(currentNode: Node): void {
  if (!('tagName' in currentNode) || !('setAttribute' in currentNode)) return;
  const element = currentNode as Element;
  if (element.tagName !== 'IMG') return;
  element.setAttribute('loading', 'lazy');
  element.setAttribute('decoding', 'async');
  element.setAttribute('fetchpriority', 'low');
}

export const documentPreviewDomPurifyConfig = {
  ...DOMPurifyConfig,
  // Append to the base ADD_ATTR instead of replacing it, otherwise the links' target="_blank" is stripped again.
  ADD_ATTR: [...DOMPurifyConfig.ADD_ATTR, ...DOCUMENT_PREVIEW_IMAGE_ATTRS],
};

const documentPreviewSecurityHooks: SecurityHooks = {
  beforeSanitizeElements: domPurifySecurityHooks.beforeSanitizeElements,
  afterSanitizeElements: (currentNode) => {
    domPurifySecurityHooks.afterSanitizeElements(currentNode);
    applyDocumentPreviewImageAttributes(currentNode);
  },
};

/** Sanitize DocumentPreview Markdown and enforce a single image loading policy. */
export function sanitizeDocumentPreviewHTML(html: string): string {
  if (!html || typeof html !== 'string') {
    return '';
  }

  try {
    const preparedHTML = protectProviderImageSrcInHTML(html);
    return sanitizeWithSecurityHooks(
      preparedHTML,
      documentPreviewDomPurifyConfig as unknown as Config,
      documentPreviewSecurityHooks,
    );
  } catch (error) {
    console.error('Document preview HTML sanitization failed:', error);
    return escapeHTML(html);
  }
}

/** Sanitize assistant markdown HTML (code/mermaid toolbars, KaTeX, SVG). */
export function sanitizeMarkdownHTML(html: string): string {
  if (!html || typeof html !== 'string') {
    return '';
  }

  try {
    const preparedHTML = protectProviderImageSrcInHTML(html);
    return sanitizeWithSecurityHooks(
      preparedHTML,
      markdownDomPurifyConfig as unknown as Config,
      markdownDomPurifySecurityHooks,
    );
  } catch (error) {
    console.error('Markdown HTML sanitization failed:', error);
    return escapeHTML(html);
  }
}

function isRasterProtectedImage(file: LoadedProtectedFile): boolean {
  return file.blob.type.startsWith('image/') && !file.blob.type.includes('svg');
}

function imageAltFromTag(before: string, after: string): string {
  const match = `${before} ${after}`.match(/\salt=(["'])(.*?)\1/i);
  return match?.[2] ?? '';
}

function escapeAttr(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#x27;')
    .replace(/</g, '&lt;');
}

function buildProtectedFileCardTag(file: LoadedProtectedFile, source: string, alt: string): string {
  const name = file.fileName || alt || 'download';
  const label = alt || name;
  return `<a href="${escapeAttr(file.blobURL)}" download="${escapeAttr(name)}" class="protected-resource-card" data-protected-resource="${escapeAttr(source)}" title="${escapeAttr(name)}">${escapeHTML(label)}</a>`;
}

function buildProtectedImageTag(
  before: string,
  quote: string,
  protectedSrc: string,
  after: string,
): string {
  // Reuse the already-hydrated file if we have one, so repeated re-renders
  // (typewriter streaming) keep the same stable image or download card
  // instead of flashing back to the placeholder every frame.
  const cached = protectedFileBySource.get(protectedSrc);
  if (cached) {
    if (isRasterProtectedImage(cached)) {
      return `<img${before} src=${quote}${cached.blobURL}${quote} data-protected-src=${quote}${protectedSrc}${quote}${after}>`;
    }
    return buildProtectedFileCardTag(cached, protectedSrc, imageAltFromTag(before, after));
  }
  // Not hydrated yet: render the 1x1 placeholder but tag it so CSS can give
  // it a stable skeleton box. Otherwise width:auto/height:auto collapse the
  // 1x1 gif to a ~1px line that violently jumps to full size once loaded.
  return `<img${before} src=${quote}${PROVIDER_IMAGE_PLACEHOLDER}${quote} data-protected-src=${quote}${protectedSrc}${quote} data-img-loading=${quote}1${quote}${after}>`;
}

export function protectProviderImageSrcInHTML(html: string): string {
  if (!html) return html;
  const withProviderImages = html.replace(
    PROVIDER_IMG_SRC_RE,
    (_m, before, quote, provider, restPathRaw, after) => {
      const restPath = decodeProviderURL(restPathRaw);
      return buildProtectedImageTag(before, quote, `${provider}://${restPath}`, after);
    },
  );
  return withProviderImages.replace(
    STORAGE_BACKEND_IMG_SRC_RE,
    (_m, before, quote, backendID, provider, restPathRaw, after) => {
      const restPath = decodeProviderURL(restPathRaw);
      return buildProtectedImageTag(
        before,
        quote,
        `storage://${backendID}/${provider}://${restPath}`,
        after,
      );
    },
  );
}

function decodeProviderURL(raw: string): string {
  return raw
    .trim()
    .replace(/&#x2f;/gi, '/')
    .replace(/&#47;/g, '/')
    .replace(/&amp;/g, '&')
    .replace(/&quot;/g, '"');
}

function providerSourceFromImageSrc(src: string): string | null {
  const decodedSrc = decodeProviderURL(src);
  if (isProviderFileURL(decodedSrc)) {
    return decodedSrc;
  }

  try {
    const baseURL = typeof window !== 'undefined' ? window.location.origin : 'http://localhost';
    const url = new URL(decodedSrc, baseURL);
    if (!isProtectedFileProxyPath(url.pathname)) {
      return null;
    }

    const filePath = (url.searchParams.get('file_path') || '').trim();
    return isProviderFileURL(filePath) ? filePath : null;
  } catch {
    return null;
  }
}

function normalizeProtectedImageElement(img: HTMLImageElement): string | null {
  const protectedSrc = providerSourceFromImageSrc(
    img.getAttribute('data-protected-src') || '',
  );
  const src = img.getAttribute('src') || '';
  const sourceURL = protectedSrc || providerSourceFromImageSrc(src);
  if (!sourceURL) {
    return null;
  }

  img.setAttribute('data-protected-src', sourceURL);
  if (!src.trim().startsWith('blob:')) {
    img.setAttribute('src', PROVIDER_IMAGE_PLACEHOLDER);
  }
  return sourceURL;
}

/**
 * Escape HTML special characters
 * @param text Text to escape
 * @returns Escaped text
 */
export function escapeHTML(text: string): string {
  if (!text || typeof text !== 'string') {
    return '';
  }
  
  const map: { [key: string]: string } = {
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#x27;',
    '/': '&#x2F;',
    '`': '&#x60;',
    '=': '&#x3D;'
  };
  
  return text.replace(/[&<>"'`=\/]/g, (s) => map[s]);
}

/**
 * Check whether a URL is safe
 * @param url URL to check
 * @returns Whether the URL is safe
 */
export function isValidURL(url: string): boolean {
  if (!url || typeof url !== 'string') {
    return false;
  }
  const trimmed = url.trim();
  if (!trimmed) {
    return false;
  }

  // Allow same-site relative paths starting with / (such as local storage /files/images/xxx.jpg)
  if (trimmed.startsWith('/') && !trimmed.startsWith('//')) {
    return true;
  }

  // Allow the provider:// form, which the frontend later fetches with auth and replaces with a blob URL
  if (isProviderFileURL(trimmed)) {
    return true;
  }
  
  try {
    const urlObj = new URL(trimmed);
    return ['http:', 'https:'].includes(urlObj.protocol);
  } catch {
    return false;
  }
}

/**
 * Process Markdown content safely
 * @param markdown Markdown text
 * @returns Safe HTML string
 */
export function safeMarkdownToHTML(markdown: string): string {
  if (!markdown || typeof markdown !== 'string') {
    return '';
  }
  
  // Escape possible HTML tags first
  const escapedMarkdown = markdown
    .replace(/<script\b[^<]*(?:(?!<\/script>)<[^<]*)*<\/script>/gi, '')
    .replace(/<iframe\b[^<]*(?:(?!<\/iframe>)<[^<]*)*<\/iframe>/gi, '')
    .replace(/<object\b[^<]*(?:(?!<\/object>)<[^<]*)*<\/object>/gi, '')
    .replace(/<embed\b[^<]*(?:(?!<\/embed>)<[^<]*)*<\/embed>/gi, '');
  
  return escapedMarkdown;
}

/**
 * Sanitize user input
 * @param input User input
 * @returns Sanitized, safe input
 */
export function sanitizeUserInput(input: string): string {
  if (!input || typeof input !== 'string') {
    return '';
  }
  
  // Remove control characters
  let cleaned = input.replace(/[\x00-\x1F\x7F-\x9F]/g, '');
  
  // Limit the length
  if (cleaned.length > 10000) {
    cleaned = cleaned.substring(0, 10000);
  }
  
  return cleaned.trim();
}

/**
 * Check whether an image URL is safe
 * @param url Image URL
 * @returns Whether the image URL is safe
 */
export function isValidImageURL(url: string): boolean {
  if (!isValidURL(url)) {
    return false;
  }
  
  return true;
}

/**
 * Create a safe image element
 * @param src Image source
 * @param alt Alt text
 * @param title Title
 * @returns Safe image HTML
 */
export function createSafeImage(src: string, alt: string = '', title: string = ''): string {
  if (!isValidImageURL(src)) {
    return '';
  }
  
  // src is validated by isValidImageURL; keep URL structure unchanged.
  // Only escape quotes to avoid breaking attributes.
  const safeSrc = src.replace(/"/g, '&quot;');
  const safeAlt = escapeHTML(alt);
  const safeTitle = escapeHTML(title);
  
  return `<img src="${safeSrc}" alt="${safeAlt}" title="${safeTitle}" class="markdown-image" style="max-width: 100%; height: auto;">`;
}

type ProtectedFileLoadResult =
  | ({ status: 'loaded' } & LoadedProtectedFile)
  | { status: 'missing' }
  | { status: 'failed' };

type HiddenProtectedImage = {
  display: string;
  parent: HTMLElement | null;
  parentDisplay: string;
};

type ProtectedFileCacheState = {
  blobByRequest: Map<string, LoadedProtectedFile>;
  fileBySource: Map<string, LoadedProtectedFile>;
  missingRequests: Set<string>;
  failures: Map<string, number>;
  inflight: Map<string, Promise<ProtectedFileLoadResult>>;
  retryGeneration: number;
  imageRequests: WeakMap<HTMLImageElement, string>;
  hiddenImages: WeakMap<HTMLImageElement, HiddenProtectedImage>;
};

// Keep object URLs alive across Vite hot updates. A hot update replaces this
// module but not the page document, so module-local Maps would forget valid
// blob URLs and make already-loaded images fall back to the 1x1 placeholder.
const protectedFileCacheState = (() => {
  const fresh = (): ProtectedFileCacheState => ({
    blobByRequest: new Map(),
    fileBySource: new Map(),
    missingRequests: new Set(),
    failures: new Map(),
    inflight: new Map(),
    retryGeneration: 0,
    imageRequests: new WeakMap(),
    hiddenImages: new WeakMap(),
  });
  if (typeof window === 'undefined') return fresh();
  const scope = window as typeof window & {
    __aikbProtectedFileCacheV4__?: ProtectedFileCacheState;
  };
  scope.__aikbProtectedFileCacheV4__ ||= fresh();
  return scope.__aikbProtectedFileCacheV4__;
})();

const protectedFileBlobCache = protectedFileCacheState.blobByRequest;
// File keyed by the protected source URL (e.g. `resource://...`). Once an
// image or download card has been hydrated, re-renders of the same markdown
// can emit the blob src / card HTML directly instead of the placeholder.
const protectedFileBySource = protectedFileCacheState.fileBySource;
// A 404 may mean a temporary message ID was not found, not that the resource
// is missing in every message/workspace. Cache it only under that request.
const protectedFileMissingRequests = protectedFileCacheState.missingRequests;
// Throttle retries of failed file fetches. During streaming the same markdown
// is re-rendered on every chunk, producing brand-new <img> elements (so the
// per-element `authHydrated` flag is reset each time). Without throttling a
// not-yet-generated file (404) would be re-requested on every chunk. We record
// the last failure time per URL and skip re-fetching within a cooldown window,
// while still allowing a later attempt once the file becomes available.
const protectedFileFailureCache = protectedFileCacheState.failures;
const protectedFileInflight = protectedFileCacheState.inflight;
const PROTECTED_FILE_RETRY_COOLDOWN_MS = 5000;
const protectedImageRequests = protectedFileCacheState.imageRequests;
const hiddenProtectedImages = protectedFileCacheState.hiddenImages;

/**
 * Display Markdown images served through the /files proxy by fetching them with an auth header.
 * Avoids exposing the token in the URL.
 */
/**
 * Clear the failed-retry cooldown records. Call it when a stream ends and similar, so images that returned 404
 * because the file was not generated yet can retry at once instead of waiting out the cooldown window.
 */
export function clearProtectedFileFailureCache(): void {
  // An earlier request can still be in flight when completion arrives. Its
  // failure must get one fresh attempt against the newly persisted message.
  protectedFileCacheState.retryGeneration++;
  protectedFileFailureCache.clear();
  protectedFileMissingRequests.clear();
}

function protectedImageSource(img: HTMLImageElement): string {
  return normalizeProtectedImageElement(img)
    || (img.getAttribute('data-protected-src') || '').trim()
    || (img.getAttribute('src') || '').trim();
}

function forEachProtectedImageWithSource(
  root: ParentNode,
  sourceURL: string,
  requestKey: string,
  callback: (img: HTMLImageElement) => void,
): void {
  root.querySelectorAll<HTMLImageElement>('img[data-protected-src]').forEach((candidate) => {
    if (protectedImageRequests.get(candidate) === requestKey && protectedImageSource(candidate) === sourceURL) callback(candidate);
  });
}

function hideMissingProtectedImages(root: ParentNode, sourceURL: string, requestKey: string): void {
  forEachProtectedImageWithSource(root, sourceURL, requestKey, (img) => {
    // Keep the node addressable for completion/scope retries. v-stable-html
    // skips unchanged HTML, so removing it would make a settled 404 permanent
    // even after failure state is cleared and the resource becomes readable.
    if (!hiddenProtectedImages.has(img)) {
      const parent = img.parentElement;
      const standalone = parent?.tagName === 'P' && !parent.textContent?.trim() && parent.children.length === 1
        ? parent : null;
      hiddenProtectedImages.set(img, {
        display: img.style.display, parent: standalone, parentDisplay: standalone?.style.display || '',
      });
    }
    img.style.display = 'none';
    img.setAttribute('data-protected-hidden', '1');
    const parent = hiddenProtectedImages.get(img)?.parent;
    if (parent) {
      parent.style.display = 'none';
      parent.setAttribute('data-protected-hidden', '1');
    }
    img.dataset.authHydrated = '0';
  });
}

function applyHydratedProtectedImage(root: ParentNode, sourceURL: string, file: LoadedProtectedFile, requestKey: string): void {
  forEachProtectedImageWithSource(root, sourceURL, requestKey, (img) => {
    const hidden = hiddenProtectedImages.get(img);
    if (hidden) {
      img.style.display = hidden.display;
      img.removeAttribute('data-protected-hidden');
      if (hidden.parent) {
        hidden.parent.style.display = hidden.parentDisplay;
        hidden.parent.removeAttribute('data-protected-hidden');
      }
      hiddenProtectedImages.delete(img);
    }
    if (!isRasterProtectedImage(file)) {
      applyProtectedFile(img, file, sourceURL);
      return;
    }
    img.src = file.blobURL;
    img.dataset.authHydrated = '1';
    img.removeAttribute('data-img-loading');
  });
}

function ensureProtectedResourceCardClicks(): void {
  if (typeof window === 'undefined') return;
  const scope = window as typeof window & { __aikbProtectedCardClicks__?: boolean };
  if (scope.__aikbProtectedCardClicks__) return;
  scope.__aikbProtectedCardClicks__ = true;
  window.addEventListener('click', (event) => {
    const target = event.target as Element | null;
    const link = target?.closest?.('a.protected-resource-card');
    if (!(link instanceof HTMLAnchorElement)) return;
    if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
    const source = link.dataset.protectedResource || '';
    const file = protectedFileBySource.get(source);
    if (!file) return;
    const preview = new CustomEvent(RESOURCE_PREVIEW_EVENT, {
      detail: file,
      cancelable: true,
    });
    if (!window.dispatchEvent(preview)) event.preventDefault();
    event.stopPropagation();
  }, true);
}

/**
 * Fetch protected images in the content (resource:// and so on) with auth through the matching file proxy,
 * then display them as blob URLs.
 *
 * {@link resolveProtectedFileAccess} decides which proxy: the default context the app entry registered
 * (such as the Embed plane of the embed app) wins, and components only narrow the scope within the same auth plane
 * through `access` (such as a knowledge base).
 */
export async function hydrateProtectedFileImages(
  root: ParentNode | null | undefined,
  access?: ProtectedFileAccessContext,
): Promise<void> {
  if (!root || typeof window === 'undefined') {
    return;
  }

  ensureProtectedResourceCardClicks();

  const images = root.querySelectorAll<HTMLImageElement>(
    'img[data-protected-src], img[src^="resource://"], img[src^="storage://"], img[src^="local://"], img[src^="minio://"], img[src^="cos://"], img[src^="tos://"], img[src^="s3://"], img[src^="oss://"], img[src^="ks3://"], img[src^="obs://"]',
  );
  if (!images.length) {
    return;
  }

  const resolvedAccess = resolveProtectedFileAccess(access);

  await Promise.all(Array.from(images).map(async (img) => {
    const normalizedSourceURL = normalizeProtectedImageElement(img);
    const protectedSrc = (img.getAttribute('data-protected-src') || '').trim();
    const src = (img.getAttribute('src') || '').trim();
    const sourceURL = normalizedSourceURL || protectedSrc || src;
    if (!sourceURL) {
      return;
    }
    // A null request means this source cannot be fetched under the current
    // access context (not a storage path, or the embed token has not arrived
    // yet). Leave the placeholder so a later pass can retry.
    const request = buildProtectedFileRequest(sourceURL, resolvedAccess);
    if (!request) {
      img.dataset.authHydrated = '0';
      return;
    }
    const { url: requestURL, headers } = request;
    const requestKey = JSON.stringify([requestURL, headers]);
    if (img.dataset.authHydrated === '1' && src.startsWith('blob:') && protectedImageRequests.get(img) === requestKey) {
      return;
    }
    protectedImageRequests.set(img, requestKey);
    if (protectedFileMissingRequests.has(requestKey)) {
      hideMissingProtectedImages(root, sourceURL, requestKey);
      return;
    }
    img.dataset.authHydrated = '1';

    const cachedBlobURL = protectedFileBlobCache.get(requestKey);
    if (cachedBlobURL) {
      applyHydratedProtectedImage(root, sourceURL, cachedBlobURL, requestKey);
      return;
    }

    const lastFailure = protectedFileFailureCache.get(requestKey);
    if (lastFailure !== undefined && Date.now() - lastFailure < PROTECTED_FILE_RETRY_COOLDOWN_MS) {
      img.dataset.authHydrated = '0';
      return;
    }

    // Every component that references the same image awaits the shared task.
    // The previous Set-based de-dupe made later components return immediately;
    // only the component that started the fetch was updated, leaving all other
    // occurrences stuck on the transparent placeholder forever.
    let loadTask = protectedFileInflight.get(requestKey);
    if (!loadTask) {
      loadTask = (async (): Promise<ProtectedFileLoadResult> => {
        for (let attempt = 0; ; attempt++) {
          const generation = protectedFileCacheState.retryGeneration;
          try {
            const resp = await fetch(requestURL, {
              method: 'GET',
              headers,
              credentials: 'include',
            });
            if (!resp.ok) {
              if (attempt === 0 && generation !== protectedFileCacheState.retryGeneration) continue;
              if (resp.status === 404) {
                protectedFileFailureCache.set(requestKey, Date.now());
                return { status: 'missing' };
              }
              throw new Error(`HTTP ${resp.status}`);
            }
            const blob = await resp.blob();
            const blobURL = URL.createObjectURL(blob);
            const file = { blobURL, blob, fileName: responseFileName(resp.headers.get("Content-Disposition"), sourceURL) };
            protectedFileBlobCache.set(requestKey, file);
            protectedFileFailureCache.delete(requestKey);
            return { status: 'loaded', ...file };
          } catch (error) {
            if (attempt === 0 && generation !== protectedFileCacheState.retryGeneration) continue;
            console.warn('[security] hydrateProtectedFileImages failed:', error);
            protectedFileFailureCache.set(requestKey, Date.now());
            return { status: 'failed' };
          }
        }
      })().finally(() => protectedFileInflight.delete(requestKey));
      protectedFileInflight.set(requestKey, loadTask);
    }

    const result = await loadTask;
    // A late response for an old message ID must not remove or overwrite an
    // image that has since been reauthorized under its persisted message ID.
    if (protectedImageRequests.get(img) !== requestKey) return;
    if (result.status === 'loaded') {
      protectedFileBySource.set(sourceURL, result);
      protectedFileMissingRequests.delete(requestKey);
      applyHydratedProtectedImage(root, sourceURL, result, requestKey);
      return;
    }
    if (result.status === 'missing') {
      protectedFileMissingRequests.add(requestKey);
      hideMissingProtectedImages(root, sourceURL, requestKey);
      return;
    }
    if (result.status === 'failed') {
      img.dataset.authHydrated = '0';
    }
  }));
}
