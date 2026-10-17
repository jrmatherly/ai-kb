import { test } from 'node:test';
import assert from 'node:assert/strict';
import type { Agent, Reference, StreamEvent } from '../shared/types';
import { APPROVAL_TEXT, applyError, applyEvent, quickAnswer, type UiMessage } from './chat';
import { toolLabel } from './toolLabels';

const empty = (): UiMessage => ({ id: 'a1', role: 'assistant', markdown: '', cards: [], references: [], done: false });
const ref: Reference = { id: 'c1', knowledge_id: 'k1', knowledge_base_id: 'kb1', knowledge_title: 'Doc', content: 'text' };

function run(events: StreamEvent[], start = empty()): UiMessage {
  return events.reduce((msg, e) => applyEvent(msg, e).msg, start);
}

const toolCall = (id: string): StreamEvent => ({ response_type: 'tool_call', data: { tool_call_id: id, tool_name: 'knowledge_search' } });

test('a full agent stream folds into one thinking card, one tool card, the answer and its references', () => {
  const msg = run([
    { response_type: 'thinking', content: 'a', data: { event_id: 'th1' } },
    { response_type: 'thinking', content: 'b', data: { event_id: 'th1' } },
    { response_type: 'tool_call', data: { tool_call_id: 't1', tool_name: 'knowledge_search' } },
    { response_type: 'tool_result', data: { tool_call_id: 't1' } },
    { response_type: 'answer', content: 'Hi' },
    { response_type: 'references', knowledge_references: [ref] },
    { response_type: 'complete', done: true },
  ]);
  assert.deepEqual(msg, {
    id: 'a1',
    role: 'assistant',
    markdown: 'Hi',
    cards: [
      { kind: 'thinking', id: 'th1', text: 'ab', status: 'done' },
      { kind: 'tool', id: 't1', name: 'Knowledge Search', text: '', status: 'done' },
    ],
    references: [ref],
    done: true,
  });
});

test('applyEvent does not mutate the message it is given', () => {
  const start = empty();
  applyEvent(start, { response_type: 'answer', content: 'Hi' });
  assert.equal(start.markdown, '');
});

test('the server shape keys tool cards by tool_call_id and a failed result fails the card', () => {
  const msg = run([toolCall('call_1'), { response_type: 'tool_result', content: 'no access', data: { tool_call_id: 'call_1', success: false } }]);
  assert.deepEqual(msg.cards, [{ kind: 'tool', id: 'call_1', name: 'Knowledge Search', text: 'no access', status: 'failed' }]);
});

test('tool_result resolves the newest card with that key', () => {
  const msg = run([toolCall('t1'), { response_type: 'tool_result', data: { tool_call_id: 't1' } }, toolCall('t1'), { response_type: 'tool_result', data: { tool_call_id: 't1' } }]);
  assert.deepEqual(msg.cards.map((c) => c.status), ['done', 'done']);
  const half = run([toolCall('t1'), { response_type: 'tool_result', data: { tool_call_id: 't1' } }, toolCall('t1')]);
  assert.deepEqual(half.cards.map((c) => c.status), ['done', 'running']);
});

test('an error ends the turn and fails every running card', () => {
  const msg = run([{ response_type: 'answer', content: 'Hi' }, toolCall('t1'), { response_type: 'error', content: 'model failed', done: true }]);
  assert.equal(msg.error, 'model failed');
  assert.equal(msg.done, true);
  assert.deepEqual(msg.cards.map((c) => c.status), ['failed']);
});

test('the first tool call retracts the preamble, a repeated call and final_answer do not', () => {
  const retracted = run([{ response_type: 'answer', content: 'Let me search. ' }, toolCall('t1'), { response_type: 'answer', content: 'Found it.' }]);
  assert.equal(retracted.markdown, 'Found it.');
  const repeated = run([toolCall('t1'), { response_type: 'answer', content: 'A' }, toolCall('t1')]);
  assert.equal(repeated.markdown, 'A');
  assert.equal(repeated.cards.length, 1);
  const final = run([{ response_type: 'answer', content: 'A' }, { response_type: 'tool_call', data: { tool_call_id: 'f', tool_name: 'final_answer' } }]);
  assert.deepEqual([final.markdown, final.cards.length], ['A', 0]);
});

test('thinking cards are keyed by event_id even when a done marker arrives after a tool call', () => {
  const msg = run([
    { response_type: 'thinking', content: 'a', data: { event_id: 'th1' } },
    toolCall('t1'),
    { response_type: 'thinking', content: '', done: true, data: { event_id: 'th1' } },
    { response_type: 'thinking', content: 'b', data: { event_id: 'th2' } },
  ]);
  assert.deepEqual(msg.cards.map((c) => [c.id, c.text, c.status]), [['th1', 'a', 'done'], ['t1', '', 'running'], ['th2', 'b', 'running']]);
});

test('complete takes the server final_content and stop settles the cards without one', () => {
  const completed = run([{ response_type: 'answer', content: 'draft' }, toolCall('t1'), { response_type: 'complete', done: true, data: { final_content: 'final' } }]);
  assert.deepEqual([completed.markdown, completed.done, completed.cards[0].status], ['final', true, 'done']);
  const stopped = run([toolCall('t1'), { response_type: 'answer', content: 'part' }, { response_type: 'stop', content: 'Generation stopped by user', done: true }]);
  assert.deepEqual([stopped.markdown, stopped.done, stopped.cards[0].status, stopped.error], ['part', true, 'done', undefined]);
});

test('resolved approvals leave waiting, approved as done and rejected as failed', () => {
  const msg = run([
    { response_type: 'tool_approval_required', data: { pending_id: 'p1' } },
    { response_type: 'tool_approval_required', data: { pending_id: 'p2' } },
    { response_type: 'tool_approval_resolved', data: { pending_id: 'p1', approved: true } },
    { response_type: 'mcp_oauth_resolved', data: { pending_id: 'p2', approved: false } },
    { response_type: 'complete', done: true },
  ]);
  assert.deepEqual(msg.cards.map((c) => c.status), ['done', 'failed']);
});

test('tool_approval_required and mcp_oauth_required each add a waiting approval card', () => {
  const msg = run([
    { response_type: 'tool_approval_required', id: 'e1', content: 'MCP tool requires human approval', data: { pending_id: 'p1' } },
    { response_type: 'mcp_oauth_required', id: 'e2', content: 'MCP service requires OAuth authorization', data: { pending_id: 'p2' } },
  ]);
  assert.deepEqual(msg.cards, [
    { kind: 'approval', id: 'p1', text: APPROVAL_TEXT, status: 'waiting' },
    { kind: 'approval', id: 'p2', text: APPROVAL_TEXT, status: 'waiting' },
  ]);
});

test('agent_query returns the assistant message id and session_title returns the title', () => {
  assert.equal(applyEvent(empty(), { response_type: 'agent_query', assistant_message_id: 'am1', data: { assistant_message_id: 'am1' } }).assistantMessageId, 'am1');
  assert.equal(applyEvent(empty(), { response_type: 'agent_query', data: { assistant_message_id: 'am2' } }).assistantMessageId, 'am2');
  assert.equal(applyEvent(empty(), { response_type: 'session_title', content: 'Pricing', data: { title: 'Pricing' } }).sessionTitle, 'Pricing');
});

test('an unknown event type returns the message unchanged', () => {
  const start = run([{ response_type: 'answer', content: 'Hi' }]);
  assert.deepEqual(applyEvent(start, { response_type: 'memory_recalled', content: 'x' }), { msg: start });
});

test('applyError maps each stream failure to its panel text and keeps the partial answer', () => {
  const partial = run([{ response_type: 'answer', content: 'Hi' }]);
  assert.deepEqual(applyError(partial, 'incomplete'), { ...partial, error: 'The answer was cut off', done: true });
  assert.equal(applyError(partial, 'unauthorized').error, 'Signed out elsewhere');
  assert.equal(applyError(partial, 'network', 'Failed to fetch', 'https://kb.example.com').error, 'Cannot reach https://kb.example.com');
  assert.equal(applyError(partial, 'server', 'quota exceeded').error, 'quota exceeded');
});

test('quickAnswer is the builtin id or the quick-answer mode', () => {
  const agent = (id: string, agent_mode: string): Agent => ({ id, name: id, config: { agent_mode, kb_selection_mode: 'all' } });
  assert.equal(quickAnswer(agent('builtin-quick-answer', 'smart-reasoning')), true);
  assert.equal(quickAnswer(agent('custom', 'quick-answer')), true);
  assert.equal(quickAnswer(agent('custom', 'smart-reasoning')), false);
});

test('toolLabel uses the web UI label and falls back to the spaced raw name', () => {
  assert.equal(toolLabel('knowledge_search'), 'Knowledge Search');
  assert.equal(toolLabel('todo_write'), 'Make Plan');
  assert.equal(toolLabel('web_fetch_page'), 'web fetch page');
});
