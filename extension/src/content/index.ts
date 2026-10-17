import type { ExtractMode, ExtractResult } from '../shared/messages';
import { extractArticle, extractSelect } from './extract';

declare global {
  // eslint-disable-next-line no-var
  var __aikbExtract: (mode: ExtractMode) => Promise<ExtractResult>;
}

// Injected on demand under the activeTab grant; nothing runs until the worker calls this.
globalThis.__aikbExtract = async (mode) => {
  if (mode === 'article') return { draft: extractArticle(document) };
  const draft = await extractSelect(document);
  return draft ? { draft } : { draft: null, reason: 'cancelled' };
};

export {};
