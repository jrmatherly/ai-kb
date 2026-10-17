import type { ApiClient, ManualBody } from './api';
import type { Capture, CaptureDraft, KnowledgeBase, SharedKnowledgeBase } from './types';

export const MAX_CONTENT_CHARS = 200000;
const MIN_ARTICLE_CHARS = 50;
const RECENT_LIMIT = 50;

// internal/utils/security.go xssPatterns, which the server applies to titles; kept in the same order.
export const SERVER_REJECT_PATTERNS: RegExp[] = [
  /<script[^>]*>.*?<\/script>/i,
  /<iframe[^>]*>.*?<\/iframe>/i,
  /<object[^>]*>.*?<\/object>/i,
  /<embed[^>]*>.*?<\/embed>/i,
  /<embed[^>]*>/i,
  /<form[^>]*>.*?<\/form>/i,
  /<input[^>]*>/i,
  /<button[^>]*>.*?<\/button>/i,
  /javascript:/i,
  /vbscript:/i,
  /onload\s*=/i,
  /onerror\s*=/i,
  /onclick\s*=/i,
  /onmouseover\s*=/i,
  /onfocus\s*=/i,
  /onblur\s*=/i,
];
// The server also rejects control characters other than tab, LF and CR in a title.
const CONTROL_CHARS = /[\u0000-\u0008\u000b\u000c\u000e-\u001f]/;
const STRIP_PATTERNS = [...SERVER_REJECT_PATTERNS.map((p) => new RegExp(p.source, 'gi')), new RegExp(CONTROL_CHARS.source, 'g')];

function codePoints(text: string): number {
  let n = 0;
  for (const _ of text) n++;
  return n;
}

export type LimitCheck =
  | { ok: true }
  | { ok: false; reason: 'too_long' | 'too_short' | 'bad_title'; length: number; cleanedTitle?: string };

// The limit applies to the whole body the server receives; the minimum to what the user wrote or clipped.
export function checkLimits(draft: CaptureDraft): LimitCheck {
  const length = codePoints(draftToBody(draft).content);
  if (length > MAX_CONTENT_CHARS) return { ok: false, reason: 'too_long', length };
  const written = draft.markdown.trim();
  if (written === '' || (draft.kind === 'article' && codePoints(written) < MIN_ARTICLE_CHARS)) return { ok: false, reason: 'too_short', length };
  if (SERVER_REJECT_PATTERNS.some((p) => p.test(draft.title)) || CONTROL_CHARS.test(draft.title)) {
    const cleanedTitle = STRIP_PATTERNS.reduce((t, p) => t.replace(p, ''), draft.title).replace(/\s+/g, ' ').trim();
    return { ok: false, reason: 'bad_title', length, cleanedTitle };
  }
  return { ok: true };
}

function sourceLines(draft: CaptureDraft): string[] {
  if (!draft.url) return [];
  const lines = [`Source: ${draft.url}`];
  if (draft.meta.author) lines.push(`Author: ${draft.meta.author}`);
  if (draft.meta.published) lines.push(`Published: ${draft.meta.published}`);
  lines.push('');
  return lines;
}

export function draftToBody(draft: CaptureDraft): ManualBody {
  return { title: draft.title, content: [...sourceLines(draft), draft.markdown].join('\n') };
}

const BREAK_SEARCH = 2000;

// Trims the Markdown so the whole body, source lines included, fits the server limit,
// cutting at the last line break near the limit so no word, link or fence is split.
export function trimDraft(draft: CaptureDraft): CaptureDraft {
  const room = MAX_CONTENT_CHARS - codePoints([...sourceLines(draft), ''].join('\n'));
  if (codePoints(draft.markdown) <= room) return draft;
  const chars = Array.from(draft.markdown).slice(0, room);
  const lastBreak = chars.lastIndexOf('\n');
  const cut = lastBreak >= room - BREAK_SEARCH ? lastBreak : room;
  return { ...draft, markdown: chars.slice(0, cut).join('').trimEnd() };
}

const isDocumentBase = (kb: KnowledgeBase | null): kb is KnowledgeBase => !!kb && (!kb.type || kb.type === 'document');

export function uniqueById<T extends { id: string }>(items: T[]): T[] {
  const seen = new Set<string>();
  const unique: T[] = [];
  for (const item of items) {
    if (seen.has(item.id)) continue;
    seen.add(item.id);
    unique.push(item);
  }
  return unique;
}

export const sharedBases = (shared: SharedKnowledgeBase[]): KnowledgeBase[] => shared.flatMap((s) => (s.knowledge_base ? [s.knowledge_base] : []));

export function writableKnowledgeBases(own: KnowledgeBase[], shared: SharedKnowledgeBase[]): KnowledgeBase[] {
  const writable = shared.filter((s) => s.permission === 'editor' || s.permission === 'admin');
  return uniqueById([...own, ...sharedBases(writable)].filter(isDocumentBase));
}

// An update keeps the knowledge where it already lives, whatever base the picker shows.
export async function saveCapture(api: ApiClient, draft: CaptureDraft, kbId: string, existing?: Capture): Promise<Capture> {
  const body = draftToBody(draft);
  if (existing) await api.updateManual(existing.knowledgeId, body);
  const knowledgeId = existing?.knowledgeId ?? (await api.createManual(kbId, body)).id;
  return { knowledgeId, kbId: existing?.kbId ?? kbId, title: draft.title, url: existing?.url || draft.url, savedAt: new Date().toISOString(), status: 'pending' };
}

// Server parse states that end the wait. `finalizing` is already queryable; only enrichment is still running.
const TERMINAL: Record<string, Capture['status']> = {
  finalizing: 'completed',
  completed: 'completed',
  failed: 'failed',
  cancelled: 'cancelled',
  deleting: 'cancelled',
};

export type Clock = { sleep: (ms: number) => Promise<void>; now: () => number };

export async function pollParse(
  api: ApiClient,
  knowledgeId: string,
  opts: { intervalMs: number; timeoutMs: number },
  clock: Clock,
): Promise<{ status: Capture['status']; message?: string }> {
  const started = clock.now();
  while (clock.now() - started < opts.timeoutMs) {
    const knowledge = await api.getKnowledge(knowledgeId);
    const status = TERMINAL[knowledge.parse_status];
    if (status) return status === 'failed' && knowledge.error_message ? { status, message: knowledge.error_message } : { status };
    await clock.sleep(opts.intervalMs);
  }
  return { status: 'timeout' };
}

const canonicalUrl = (url: string) => url.replace(/#.*$/, '').replace(/\/$/, '');

export function findRecentByUrl(captures: Capture[], url: string): Capture | undefined {
  if (!url) return undefined;
  const wanted = canonicalUrl(url);
  return captures.find((c) => canonicalUrl(c.url) === wanted);
}

export function rememberCapture(captures: Capture[], capture: Capture): Capture[] {
  return [capture, ...captures.filter((c) => c.knowledgeId !== capture.knowledgeId)].slice(0, RECENT_LIMIT);
}

export async function resyncCaptures(api: ApiClient, captures: Capture[]): Promise<Capture[]> {
  const open = captures.filter((c) => c.status === 'pending' || c.status === 'timeout').map((c) => c.knowledgeId);
  if (open.length === 0) return captures;
  const statuses = new Map((await api.getKnowledgeBatch(open)).map((k) => [k.id, k.parse_status]));
  return captures.map((c) => {
    if (!open.includes(c.knowledgeId)) return c;
    const parseStatus = statuses.get(c.knowledgeId);
    // Missing from the batch means it was deleted on the server or is no longer visible.
    if (parseStatus === undefined) return { ...c, status: 'cancelled' };
    const status = TERMINAL[parseStatus];
    return status ? { ...c, status } : c;
  });
}
