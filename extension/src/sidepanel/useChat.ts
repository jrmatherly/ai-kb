import { computed, reactive } from 'vue';
import { ApiError, type ApiClient } from '../shared/api';
import { sharedBases, uniqueById } from '../shared/capture';
import type { Agent, Auth, KnowledgeBase, Message } from '../shared/types';
import { applyError, applyEvent, kbChoice, quickAnswer, type UiMessage } from './chat';

type Deps = { api: ApiClient; authKind: Auth['kind'] };

const fromHistory = (m: Message): UiMessage => ({
  id: m.id,
  role: m.role,
  markdown: m.content,
  cards: [],
  references: m.knowledge_references ?? [],
  done: true,
  createdAt: m.created_at,
});

export function useChat(deps: Deps) {
  const state = reactive({
    agents: [] as Agent[],
    kbs: [] as KnowledgeBase[],
    agentId: null as string | null,
    kbIds: [] as string[],
    sessionId: null as string | null,
    sessionTitle: '',
    assistantMessageId: null as string | null,
    messages: [] as UiMessage[],
    streaming: false,
    suggestions: [] as string[],
    historyMode: (deps.authKind === 'bearer' ? 'public' : 'handle') as 'handle' | 'public',
  });
  let controller: AbortController | null = null;
  const agent = computed(() => state.agents.find((a) => a.id === state.agentId));
  const choice = computed(() => kbChoice(agent.value, state.kbs, state.kbIds));

  function setKbIds(ids: string[]) {
    state.kbIds = ids;
  }

  async function loadPage(sessionId: string, beforeTime?: string): Promise<UiMessage[]> {
    const page = (resourceUrls: 'handle' | 'public') => deps.api.loadMessages(sessionId, { ...(beforeTime ? { beforeTime } : {}), resourceUrls });
    const rows = await page(state.historyMode).catch((e: unknown) => {
      if (!(e instanceof ApiError && e.status === 403 && state.historyMode === 'public')) throw e;
      state.historyMode = 'handle';
      return page('handle');
    });
    return rows.map(fromHistory);
  }

  async function selectAgent(id: string) {
    const agent = state.agents.find((a) => a.id === id);
    state.agentId = id;
    if (agent?.config.kb_selection_mode === 'selected') state.kbIds = [...(agent.config.knowledge_bases ?? [])];
    else if (agent?.config.kb_selection_mode === 'none') state.kbIds = [];
    state.suggestions = await deps.api.suggestedQuestions(id).catch(() => []);
  }

  async function load() {
    const [agents, own, shared] = await Promise.all([deps.api.listAgents(), deps.api.listKnowledgeBases(), deps.api.listSharedKnowledgeBases()]);
    state.agents = agents;
    state.kbs = uniqueById([...own, ...sharedBases(shared)]);
    if (!state.agentId && agents[0]) await selectAgent(agents[0].id);
  }

  async function ask(query: string) {
    if (state.streaming) return;
    state.streaming = true;
    const quick = agent.value ? quickAnswer(agent.value) : false;
    try {
      state.sessionId ??= await deps.api.createSession();
    } catch (e) {
      state.streaming = false;
      throw e;
    }
    state.messages.push({ id: crypto.randomUUID(), role: 'user', markdown: query, cards: [], references: [], done: true });
    const answerId = crypto.randomUUID();
    state.messages.push({ id: answerId, role: 'assistant', markdown: '', cards: [], references: [], done: false });
    // Late events land in this conversation's list even after newConversation replaces state.messages, and
    // by id, because loadOlder prepends to the same list mid-stream.
    const list = state.messages;
    const update = (apply: (m: UiMessage) => UiMessage) => {
      const i = list.findIndex((m) => m.id === answerId);
      if (i >= 0) list[i] = apply(list[i]);
    };
    state.assistantMessageId = null;
    controller = new AbortController();
    try {
      await deps.api.chatStream(
        quick ? 'knowledge-chat' : 'agent-chat',
        state.sessionId,
        { query, channel: 'browser_extension', knowledge_base_ids: [...state.kbIds], agent_id: state.agentId, agent_enabled: !quick },
        {
          onEvent: (e) => {
            update((m) => {
              const applied = applyEvent(m, e);
              if (applied.sessionTitle) state.sessionTitle = applied.sessionTitle;
              if (applied.assistantMessageId) state.assistantMessageId = applied.assistantMessageId;
              return applied.msg;
            });
          },
          onError: (kind, message) => update((m) => applyError(m, kind, message, deps.api.lastTarget?.serverUrl)),
        },
        controller.signal,
      );
    } finally {
      state.streaming = false;
      controller = null;
    }
  }

  function settleOpen() {
    const last = state.messages.at(-1);
    if (last && !last.done) state.messages[state.messages.length - 1] = applyEvent(last, { response_type: 'stop' }).msg;
  }

  async function stop(opts: { keepalive?: boolean } = {}) {
    controller?.abort();
    settleOpen();
    if (!state.sessionId || !state.assistantMessageId) return;
    await deps.api.stopSession(state.sessionId, state.assistantMessageId, opts);
  }

  function newConversation() {
    controller?.abort();
    state.sessionId = null;
    state.sessionTitle = '';
    state.assistantMessageId = null;
    state.messages = [];
  }

  async function clear() {
    if (state.sessionId) await deps.api.clearSession(state.sessionId);
    newConversation();
  }

  async function openSession(id: string) {
    const messages = await loadPage(id);
    newConversation();
    state.sessionId = id;
    state.messages = messages;
  }

  async function loadOlder() {
    const oldest = state.messages[0]?.createdAt;
    if (!state.sessionId || !oldest) return;
    const sessionId = state.sessionId;
    const page = await loadPage(sessionId, oldest);
    if (state.sessionId === sessionId) state.messages.unshift(...page);
  }

  return { state, api: deps.api, agent, kbChoice: choice, setKbIds, load, selectAgent, ask, stop, newConversation, clear, openSession, loadOlder };
}
