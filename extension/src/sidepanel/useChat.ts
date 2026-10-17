import { reactive } from 'vue';
import { ApiError, type ApiClient } from '../shared/api';
import { sharedBases, uniqueById } from '../shared/capture';
import type { Agent, Auth, KnowledgeBase, Message } from '../shared/types';
import { applyError, applyEvent, quickAnswer, type UiMessage } from './chat';

type Deps = {
  api: ApiClient;
  authKind: Auth['kind'];
  serverUrl: string;
  // A getter: the App swaps its cached headers after every refresh, and stop must send the current token.
  getHeaders: () => Record<string, string>;
};

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
    const agent = state.agents.find((a) => a.id === state.agentId);
    const quick = agent ? quickAnswer(agent) : false;
    try {
      state.sessionId ??= await deps.api.createSession();
    } catch (e) {
      state.streaming = false;
      throw e;
    }
    state.messages.push({ id: crypto.randomUUID(), role: 'user', markdown: query, cards: [], references: [], done: true });
    state.messages.push({ id: crypto.randomUUID(), role: 'assistant', markdown: '', cards: [], references: [], done: false });
    // Late events land in this conversation's list even after newConversation replaces state.messages.
    const list = state.messages;
    const index = list.length - 1;
    state.assistantMessageId = null;
    controller = new AbortController();
    try {
      await deps.api.chatStream(
        quick ? 'knowledge-chat' : 'agent-chat',
        state.sessionId,
        { query, channel: 'browser_extension', knowledge_base_ids: [...state.kbIds], agent_id: state.agentId, agent_enabled: !quick },
        {
          onEvent: (e) => {
            const applied = applyEvent(list[index], e);
            list[index] = applied.msg;
            if (applied.sessionTitle) state.sessionTitle = applied.sessionTitle;
            if (applied.assistantMessageId) state.assistantMessageId = applied.assistantMessageId;
          },
          onError: (kind, message) => {
            list[index] = applyError(list[index], kind, message, deps.serverUrl);
          },
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

  async function stop(opts?: { keepalive: boolean }) {
    controller?.abort();
    settleOpen();
    if (!state.sessionId || !state.assistantMessageId) return;
    await deps.api.stopSession(state.sessionId, state.assistantMessageId, {
      keepalive: opts?.keepalive,
      direct: { serverUrl: deps.serverUrl, headers: deps.getHeaders() },
    });
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
    if (state.sessionId === sessionId) state.messages = [...page, ...state.messages];
  }

  return { state, load, selectAgent, ask, stop, newConversation, clear, openSession, loadOlder };
}
