import type { StreamErrorKind } from '../shared/api';
import type { Agent, KnowledgeBase, Reference, StreamEvent } from '../shared/types';
import { toolLabel } from './toolLabels';

// Card identity follows the server (internal/handler/session/agent_stream_handler.go): thinking cards by
// data.event_id, tool cards by data.tool_call_id, approval cards by data.pending_id.
export type Card = {
  kind: 'thinking' | 'tool' | 'approval';
  id: string;
  name?: string;
  text: string;
  status: 'running' | 'done' | 'failed' | 'waiting';
};

export const STATUS_LABEL: Record<Card['status'], string> = {
  running: 'Running',
  done: 'Done',
  failed: 'Failed',
  waiting: 'Waiting for approval',
};

export type UiMessage = {
  id: string;
  role: 'user' | 'assistant';
  markdown: string;
  cards: Card[];
  references: Reference[];
  error?: string;
  done: boolean;
  createdAt?: string;
};

export type Applied = { msg: UiMessage; sessionTitle?: string; assistantMessageId?: string };

export const APPROVAL_TEXT = 'This step needs your approval in the web UI';

const str = (v: unknown): string | undefined => (typeof v === 'string' && v !== '' ? v : undefined);

const settle = (cards: Card[], status: 'done' | 'failed'): Card[] => cards.map((c) => (c.status === 'running' ? { ...c, status } : c));

function patchCard(msg: UiMessage, kind: Card['kind'], id: string | undefined, patch: (c: Card) => Partial<Card>, open = false): UiMessage {
  for (let i = msg.cards.length - 1; i >= 0; i--) {
    const c = msg.cards[i];
    if (c.kind === kind && c.id === id && (!open || c.status === 'running' || c.status === 'waiting')) return { ...msg, cards: msg.cards.map((x, j) => (j === i ? { ...x, ...patch(x) } : x)) };
  }
  return msg;
}

// Patches the newest open card with that identity, or appends one; `created` tells the caller which happened.
function upsertCard(msg: UiMessage, card: Card, patch: (c: Card) => Partial<Card>): { msg: UiMessage; created: boolean } {
  const patched = patchCard(msg, card.kind, card.id, patch, true);
  if (patched !== msg) return { msg: patched, created: false };
  return { msg: { ...msg, cards: [...msg.cards, card] }, created: true };
}

const approval = (msg: UiMessage, e: StreamEvent): UiMessage =>
  upsertCard(msg, { kind: 'approval', id: str(e.data?.pending_id) ?? e.id ?? `approval-${msg.cards.length}`, text: APPROVAL_TEXT, status: 'waiting' }, () => ({})).msg;

const resolved = (msg: UiMessage, e: StreamEvent): UiMessage =>
  patchCard(msg, 'approval', str(e.data?.pending_id), () => ({ status: e.data?.approved === false ? 'failed' : 'done' }));

const handlers: Record<string, (msg: UiMessage, e: StreamEvent) => UiMessage> = {
  answer: (msg, e) => ({ ...msg, markdown: msg.markdown + (e.content ?? '') }),

  thinking: (msg, e) => {
    const id = str(e.data?.event_id) ?? e.id ?? 'thinking';
    const status = e.done ? 'done' : 'running';
    return upsertCard(msg, { kind: 'thinking', id, text: e.content ?? '', status }, (c) => ({ text: c.text + (e.content ?? ''), status })).msg;
  },

  tool_call: (msg, e) => {
    const name = str(e.data?.tool_name) ?? '';
    if (name === 'final_answer') return msg;
    const id = str(e.data?.tool_call_id) ?? `tool-${msg.cards.length}`;
    const label = toolLabel(name);
    const next = upsertCard(msg, { kind: 'tool', id, name: label, text: '', status: 'running' }, () => ({ name: label }));
    // Answer text before a new tool call was a round's preamble; the server drops it from the stored answer too.
    return next.created ? { ...next.msg, markdown: '' } : next.msg;
  },

  tool_result: (msg, e) =>
    patchCard(msg, 'tool', str(e.data?.tool_call_id), () => ({ text: e.content ?? '', status: e.data?.success === false ? 'failed' : 'done' })),

  references: (msg, e) => ({ ...msg, references: e.knowledge_references ?? [] }),

  // Tool failures arrive as tool_result with success false; an error event always ends the turn.
  error: (msg, e) => ({ ...msg, error: e.content || str(e.data?.error), done: true, cards: settle(msg.cards, 'failed') }),

  complete: (msg, e) => ({ ...msg, markdown: str(e.data?.final_content) ?? msg.markdown, done: true, cards: settle(msg.cards, 'done') }),

  stop: (msg) => ({ ...msg, done: true, cards: settle(msg.cards, 'done') }),

  tool_approval_required: approval,
  mcp_oauth_required: approval,
  tool_approval_resolved: resolved,
  mcp_oauth_resolved: resolved,
};

const meta: Record<string, (e: StreamEvent) => Omit<Applied, 'msg'>> = {
  session_title: (e) => ({ sessionTitle: e.content || str(e.data?.title) }),
  agent_query: (e) => ({ assistantMessageId: e.assistant_message_id || str(e.data?.assistant_message_id) }),
};

export function applyEvent(msg: UiMessage, e: StreamEvent): Applied {
  const handle = handlers[e.response_type];
  return { msg: handle ? handle(msg, e) : msg, ...meta[e.response_type]?.(e) };
}

export function applyError(msg: UiMessage, kind: StreamErrorKind, message?: string, serverUrl?: string): UiMessage {
  const text: Record<StreamErrorKind, string> = {
    unauthorized: 'Signed out elsewhere',
    network: `Cannot reach ${serverUrl ?? 'the server'}`,
    incomplete: 'The answer was cut off',
    server: message ?? 'The server returned an error',
  };
  return { ...msg, error: text[kind], done: true, cards: settle(msg.cards, 'failed') };
}

export const quickAnswer = (agent: Agent): boolean => agent.id === 'builtin-quick-answer' || agent.config.agent_mode === 'quick-answer';

export type KbChoice = { show: boolean; locked: boolean; kbs: KnowledgeBase[] };

// `selected` agents carry their own bases (selectAgent copies them into kbIds), so the picker shows them locked.
export function kbChoice(agent: Agent | undefined, kbs: KnowledgeBase[], kbIds: string[]): KbChoice {
  const mode = agent?.config.kb_selection_mode;
  if (!agent || mode === 'none') return { show: false, locked: false, kbs: [] };
  if (mode === 'selected') return { show: true, locked: true, kbs: kbs.filter((kb) => kbIds.includes(kb.id)) };
  return { show: true, locked: false, kbs };
}

const TAG_RE = /<(kb|web)\b([^>]*?)\s*\/?>/g;
const ENTITIES: Record<string, string> = { amp: '&', quot: '"', apos: "'", lt: '<', gt: '>', '#39': "'", '#34': '"' };
// Entity decoding mirrors parseTagAttributes in frontend/src/utils/citationMarkdown.ts.
const attr = (attrs: string, name: string) =>
  new RegExp(`(?:^|\\s)${name}\\s*=\\s*"([^"]*)"`).exec(attrs)?.[1].replace(/&(amp|quot|apos|lt|gt|#39|#34);/g, (_m, e: string) => ENTITIES[e]);

// Mirrors frontend/src/utils/citationMarkdown.ts: a tag still streaming in is hidden until it closes.
function stripOpenTag(markdown: string): string {
  const start = markdown.lastIndexOf('<');
  if (start < 0 || markdown.slice(start).includes('>')) return markdown;
  return /^<(k(b(\s[\s\S]*)?)?|w(e(b(\s[\s\S]*)?)?)?)?$/i.test(markdown.slice(start)) ? markdown.slice(0, start) : markdown;
}

// The model often cites by context position (DOC-1, FAQ-2, 1) or by document title instead of the chunk id.
function citedIndex(attrs: string, references: Reference[]): number {
  const chunkId = attr(attrs, 'chunk_id') ?? '';
  const byId = references.findIndex((r) => r.id === chunkId);
  if (byId >= 0) return byId;
  const n = Number(/^(?:[a-z]+-)?(\d+)$/i.exec(chunkId.trim())?.[1]);
  if (n >= 1 && n <= references.length) return n - 1;
  const doc = attr(attrs, 'doc');
  return doc ? references.findIndex((r) => r.knowledge_title === doc) : -1;
}

// Turns the server's inline <kb chunk_id=.../> and <web url=.../> tags into numbered markers that match the references list.
export function inlineCitations(markdown: string, references: Reference[], anchor = 'cite'): string {
  return stripOpenTag(markdown).replace(TAG_RE, (_m, kind: string, attrs: string) => {
    if (kind === 'web') {
      try {
        const url = new URL(attr(attrs, 'url') ?? '');
        // An angle-bracket destination keeps `)` and spaces intact through marked.
        return ` [${url.hostname}](<${url.href}>)`;
      } catch {
        return '';
      }
    }
    const index = citedIndex(attrs, references);
    return index < 0 ? '' : ` [[${index + 1}]](#${anchor}-${index + 1})`;
  });
}
