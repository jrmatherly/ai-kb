import { computed, reactive, toRaw, watch } from 'vue';
import { ApiError, errorMessage, type ApiClient } from '../shared/api';
import {
  checkLimits, codePoints, draftToBody, findRecentByUrl, pollParse, resyncCaptures, saveCapture, trimDraft, writableKnowledgeBases, type Clock, type LimitCheck,
} from '../shared/capture';
import type { ExtractError, ExtractMode, ExtractReply, Request, TabAccessReply } from '../shared/messages';
import { serial } from '../shared/serial';
import * as session from '../shared/session';
import type { Capture, CaptureDraft, Connection, Intent, KnowledgeBase } from '../shared/types';

type ParseResult = Awaited<ReturnType<typeof pollParse>>;
type Limit = Extract<LimitCheck, { ok: false }>;

export type SaveError = { text: string; field?: 'kb' };
// The match is kept apart from existing, so the duplicate offer never fires for a Recent edit.
type Target = { existing?: Capture; duplicate?: Capture };

export type Editor =
  | { phase: 'idle' }
  | ({ phase: 'editing'; draft: CaptureDraft; limit: Limit | null; saveError: SaveError | null } & Target)
  | ({ phase: 'saving'; draft: CaptureDraft } & Target)
  | { phase: 'saved'; draft: CaptureDraft; existing: Capture; lastStatus: ParseResult | null; quick?: true };

export type PageError = { kind: 'unreachable' | 'cold' } | { kind: 'failed'; message: string };
export type PageState = { kind: 'ok' } | PageError;

type Flow = { saveCapture: typeof saveCapture; pollParse: typeof pollParse; resyncCaptures: typeof resyncCaptures; clock: Clock };

type Deps = {
  api: ApiClient;
  send: <T>(request: Request) => Promise<T>;
  tabId: () => number;
  tabAccess: () => TabAccessReply;
  loadConnection: () => Promise<Connection | null>;
  onError: (error: unknown) => void;
  flow?: Flow;
};

const POLL = { intervalMs: 2000, timeoutMs: 120000 };

function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) return reject(signal.reason);
    const timer = setTimeout(resolve, ms);
    signal?.addEventListener('abort', () => { clearTimeout(timer); reject(signal.reason); }, { once: true });
  });
}

const realFlow: Flow = { saveCapture, pollParse, resyncCaptures, clock: { sleep, now: Date.now } };

function saveErrorOf(error: unknown): SaveError {
  if (error instanceof ApiError && error.status === 403) return { text: 'This key cannot write to this knowledge base', field: 'kb' };
  if (error instanceof ApiError && error.status === 401) return { text: 'Signed out elsewhere' };
  return { text: errorMessage(error) };
}

const today = () => new Date().toLocaleDateString('en-CA');

const pageErrorOf = (e: ExtractError): PageError =>
  e.error === 'failed' ? { kind: 'failed', message: e.message } : { kind: e.error === 'no_access' ? 'cold' : 'unreachable' };

export function useCapture(deps: Deps) {
  const { api, send } = deps;
  const flow = deps.flow ?? realFlow;
  const state = reactive({
    editor: { phase: 'idle' } as Editor,
    kbId: null as string | null,
    kbs: [] as KnowledgeBase[],
    recent: [] as Capture[],
    pageError: null as PageError | null,
    intent: null as Intent | null,
    // A clip intent that came with its draft is already done; the view only switches to it.
    intentDraft: false,
    // Bumped on every intent read, so a repeat of the same intent still reaches the view.
    intentSeq: 0,
  });
  // An extract error describes one page; it shows while the tab is on that page, which may be known only later.
  const accessUrl = () => { const access = deps.tabAccess(); return 'url' in access ? access.url : null; };
  let errorUrl: string | null = null;
  const pageState = computed<PageState>(() => {
    if (state.pageError && accessUrl() === errorUrl) return state.pageError;
    return accessUrl() ? { kind: 'ok' } : { kind: 'cold' };
  });
  // An update stays in its entry's base, so the picker shows that base and cannot change it.
  // A duplicate leaves it free, for Save as new.
  const lockedKbId = computed(() => {
    const e = state.editor;
    return e.phase !== 'idle' && e.existing && (e.phase === 'saved' || !e.duplicate) ? e.existing.kbId : null;
  });
  const charCount = computed(() => (state.editor.phase === 'idle' ? 0 : codePoints(draftToBody(state.editor.draft).content)));
  // Two loads must not both take the same draft.
  const takeSession = serial();
  let storedLastKbId: string | null = null;
  let kbsLoaded: Promise<string | null> | null = null;

  // Resolves with the failure text rather than rejecting, so a caller can start it early and show it later.
  function loadKbs(): Promise<string | null> {
    kbsLoaded ??= Promise.all([api.listKnowledgeBases(), api.listSharedKnowledgeBases()]).then(
      ([own, shared]) => { state.kbs = writableKnowledgeBases(own, shared); return null; },
      (error: unknown) => { kbsLoaded = null; return `Could not load knowledge bases: ${errorMessage(error)}`; },
    );
    return kbsLoaded;
  }

  async function showKbs(loading: Promise<string | null>): Promise<void> {
    const failure = await loading;
    if (failure && state.editor.phase === 'editing') state.editor.saveError ??= { text: failure };
  }

  function edit(draft: CaptureDraft, target: Target = {}, limit: Limit | null = null, saveError: SaveError | null = null): void {
    state.editor = { phase: 'editing', draft, existing: target.existing, duplicate: target.duplicate, limit, saveError };
  }

  function failPage(error: PageError, url = accessUrl()): void {
    errorUrl = url;
    state.pageError = error;
  }

  async function openDraft(draft: CaptureDraft): Promise<void> {
    state.recent = await session.readRecent();
    const match = findRecentByUrl(state.recent, draft.url);
    edit(draft, { existing: match, duplicate: match });
  }

  // Clears only the keys it read, so a clip written after the read is still there for the next load.
  const take = () => takeSession(async () => {
    const tabId = deps.tabId();
    const [draft, intent, clipError] = await Promise.all([session.readDraft(tabId), session.readIntent(tabId), session.readClipError(tabId)]);
    const got = { draft, intent, clipError };
    const taken = session.TAB_KEYS.filter((key) => got[key]);
    if (taken.length > 0) await session.clearTabKeys(tabId, taken);
    return got;
  });

  // The connection is read before the draft is taken, so a failed read never loses a clip.
  async function loadFromSession(): Promise<void> {
    const connection = await deps.loadConnection();
    const { draft, intent, clipError } = await take();
    storedLastKbId = connection?.lastKbId ?? null;
    state.kbId ??= storedLastKbId;
    if (clipError) failPage(pageErrorOf(clipError), clipError.url);
    if (intent) {
      state.intent = intent;
      state.intentDraft = !!draft;
      state.intentSeq += 1;
    }
    if (!draft) return;
    const kbs = loadKbs();
    await openDraft(draft);
    // Quick clip never posts without a last-used base; without one it is an ordinary clip.
    // It always saves a new entry, so a selection clipped after an article on the same page cannot replace it.
    if (intent?.startsWith('quick:') && connection?.quickClip && connection.lastKbId) {
      state.kbId = connection.lastKbId;
      await save(true, true);
    }
    await showKbs(kbs);
  }

  async function startClip(mode: ExtractMode): Promise<void> {
    state.pageError = null;
    if (!('url' in deps.tabAccess())) return failPage({ kind: 'cold' });
    const kbs = loadKbs();
    const reply = await send<ExtractReply>({ type: 'EXTRACT', tabId: deps.tabId(), mode });
    if ('error' in reply) return failPage(pageErrorOf(reply));
    if (!reply.draft) return;
    await openDraft(reply.draft);
    await showKbs(kbs);
  }

  function newNote(): void {
    edit({ kind: 'note', title: `Note ${today()}`, url: '', markdown: '', meta: {} });
    void showKbs(loadKbs());
  }

  // The stored content already holds the Source block, so the draft carries no URL to prepend it again.
  async function editCapture(c: Capture): Promise<void> {
    state.pageError = null;
    const kbs = loadKbs();
    let draft: CaptureDraft;
    try {
      const knowledge = await api.getKnowledge(c.knowledgeId);
      const metadata = typeof knowledge.metadata === 'string' ? (JSON.parse(knowledge.metadata) as { content?: string }) : knowledge.metadata;
      draft = { kind: 'note', title: knowledge.title || c.title, url: '', markdown: metadata?.content ?? '', meta: {} };
    } catch (error) {
      return failPage({ kind: 'failed', message: errorMessage(error) });
    }
    edit(draft, { existing: c });
    await showKbs(kbs);
  }

  function trim(): void {
    if (state.editor.phase !== 'editing') return;
    edit(trimDraft(state.editor.draft), state.editor);
  }

  // The list in memory is patched rather than re-read, so it shows the entry without a storage round trip.
  function showInRecent(capture: Capture): void {
    state.recent = [capture, ...state.recent.filter((c) => c.knowledgeId !== capture.knowledgeId)];
  }

  async function save(asNew: boolean, quick = false): Promise<void> {
    const editor = state.editor;
    if (editor.phase !== 'editing' && editor.phase !== 'saved') return;
    const { draft } = editor;
    const target: Target = { existing: editor.existing, duplicate: editor.phase === 'editing' ? editor.duplicate : undefined };
    const existing = asNew ? undefined : editor.existing;
    const { kbId } = state;
    const base = kbId ?? existing?.kbId;
    const limit = checkLimits(draft);
    if (!limit.ok) return edit(draft, target, limit);
    if (!base) return edit(draft, target, null, { text: 'Choose a knowledge base', field: 'kb' });
    state.editor = { phase: 'saving', draft, ...target };
    const saving = state.editor;
    let capture: Capture;
    try {
      capture = await flow.saveCapture(api, draft, base, existing);
    } catch (error) {
      if (state.editor === saving) edit(draft, target, null, saveErrorOf(error));
      return;
    }
    // The user moved on while the request ran; the entry is saved but this editor no longer shows it.
    if (state.editor !== saving) return;
    state.editor = { phase: 'saved', draft, existing: capture, lastStatus: null, ...(quick ? { quick: true as const } : {}) };
    const saved = state.editor;
    const polling = new AbortController();
    const stopWatching = watch(() => state.editor, () => polling.abort(), { flush: 'sync' });
    try {
      const [, , lastStatus] = await Promise.all([
        session.rememberCapture(capture).then(() => showInRecent(capture)),
        // The entry is saved; a failed preference write only means the picker forgets this base.
        !existing && kbId && kbId !== storedLastKbId ? send({ type: 'PATCH_CONNECTION', patch: { lastKbId: kbId } }).then(() => { storedLastKbId = kbId; }, () => undefined) : undefined,
        // A poll that cannot reach the server leaves the status to the next resync.
        flow.pollParse(api, capture.knowledgeId, POLL, flow.clock, polling.signal).catch((): ParseResult => ({ status: 'timeout' })),
      ]);
      if (state.editor === saved) saved.lastStatus = lastStatus;
      // A finished poll is as good as a resync for this one entry; a timeout leaves it to the next resync.
      if (lastStatus.status !== 'timeout') {
        const parsed = { ...capture, status: lastStatus.status };
        await session.updateCaptures([parsed]);
        showInRecent(parsed);
      }
    } finally {
      stopWatching();
    }
  }

  async function resync(): Promise<void> {
    state.recent = await session.resyncRecent(api, flow.resyncCaptures);
  }

  function cancel(): void {
    state.editor = { phase: 'idle' };
  }

  // tabId is read at event time, so the subscription follows the tab the panel follows.
  const stop = session.onSessionChange((key) => {
    const tabId = deps.tabId();
    if (session.TAB_KEYS.some((name) => key === `${name}:${tabId}`)) loadFromSession().catch(deps.onError);
  });

  return { state, lockedKbId, charCount, pageState, loadFromSession, startClip, newNote, editCapture, trim, save, resync, cancel, stop };
}
