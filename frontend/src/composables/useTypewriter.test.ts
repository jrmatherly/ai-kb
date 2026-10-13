import assert from 'node:assert/strict'
import test from 'node:test'

import { computed, createRenderer, nextTick, reactive } from 'vue'
import { nextTypewriterReveal, useTypewriter } from './useTypewriter.ts'
import { applyFinalArtifactContent } from '../utils/finalArtifactContent.ts'
import { KB_WEB_TAG_RE } from '../utils/citationMarkdown.ts'

async function withTypewriter(
  run: (harness: {
    answer: { content: string; done: boolean }
    displayed: ReturnType<typeof useTypewriter>['displayed']
    frame: () => string
    drain: () => string[]
    pending: () => number
  }) => Promise<void>,
  initial = { content: '', done: false },
) {
  const originalRequest = globalThis.requestAnimationFrame
  const originalCancel = globalThis.cancelAnimationFrame
  const frames = new Map<number, FrameRequestCallback>()
  let id = 0
  let time = 0
  globalThis.requestAnimationFrame = (callback) => { frames.set(++id, callback); return id }
  globalThis.cancelAnimationFrame = (frame) => { frames.delete(frame) }
  const answer = reactive(initial)
  let displayed!: ReturnType<typeof useTypewriter>['displayed']
  const renderer = createRenderer<any, any>({
    patchProp() {}, insert() {}, remove() {}, setText() {}, setElementText() {},
    createElement: () => ({}), createText: () => ({}), createComment: () => ({}),
    parentNode: () => null, nextSibling: () => null,
  })
  const app = renderer.createApp({
    setup() {
      displayed = useTypewriter(() => answer.content, () => answer.done).displayed
      return () => null
    },
  })
  const frame = () => {
    time += 16
    const pending = [...frames.values()]
    frames.clear()
    pending.forEach(callback => callback(time))
    return displayed.value
  }
  const drain = () => {
    const values: string[] = []
    for (let count = 0; frames.size && count < 1000; count += 1) values.push(frame())
    assert.equal(frames.size, 0, 'animation should eventually finish')
    return values
  }
  try {
    app.mount({})
    await run({ answer, displayed, frame, drain, pending: () => frames.size })
  } finally {
    app.unmount()
    globalThis.requestAnimationFrame = originalRequest
    globalThis.cancelAnimationFrame = originalCancel
  }
}

test('citations reveal atomically and their hidden attributes do not change the cadence', async () => {
  // Chinese content exercises CJK text and punctuation break points.
  const kb = '<kb doc="\u6e38\u620f\u4ea4\u4e92\u8bbe\u8ba1\u4ea4\u5e95\u4e66\u8303\u4f8b.docx" chunk_id="3c67efd5-f2ff-4e26-9032-9e44e6861178" />'
  const web = `<web title="Reference" url="https://example.com/${'long-path/'.repeat(80)}" />`
  for (const tags of [kb, web, kb + web]) {
    const content = `\u8fd9\u662f\u6b63\u6587🙂\u3002${tags} \u63a5\u4e0b\u6765\u7ee7\u7eed\u8f93\u51fa\u6587\u5b57\u3002`
    const normalized = content.replace(KB_WEB_TAG_RE, '\uFFFC')
    let expected: string[] = []
    await withTypewriter(async ({ answer, drain }) => {
      answer.content = normalized
      await nextTick()
      expected = drain()
    })
    await withTypewriter(async ({ answer, displayed, drain }) => {
      answer.content = content
      await nextTick()
      const actual = drain().map(value => {
        const visible = value.replace(KB_WEB_TAG_RE, '\uFFFC')
        assert.ok(!visible.includes('<'), 'a partial citation must never be emitted')
        return visible
      })
      assert.deepEqual(actual, expected, 'each badge should cost only one visible unit')
      assert.equal(displayed.value, content, 'source attributes must be preserved exactly')
    })
  }
})

test('split citation tags wait for the closing bracket without spending reveal time on attributes', async () => {
  for (const tag of [
    '<kb doc="\u6587\u6863.docx" chunk_id="123" />',
    '<web url="https://example.com" title="Example">',
  ]) {
    await withTypewriter(async ({ answer, displayed, drain, frame, pending }) => {
      const prefix = '\u6b63\u6587\u3002'
      answer.content = prefix
      await nextTick()
      drain()
      for (let n = 1; n < tag.length; n += 1) {
        answer.content = prefix + tag.slice(0, n)
        await nextTick()
        frame()
        assert.equal(displayed.value, prefix)
        assert.equal(pending(), 0, 'hidden attributes should not run the animation loop')
      }
      answer.content = prefix + tag + '\u540e\u7eed\u6b63\u6587\u7ee7\u7eed\u3002'
      await nextTick()
      for (let n = 0; n < 3; n += 1) frame()
      assert.ok(displayed.value.includes(tag), 'a complete badge should appear within a normal reveal interval')
      drain()
      assert.equal(displayed.value, answer.content)
    })
  }
})

test('completion during a citation still catches up to the raw answer length', async () => {
  await withTypewriter(async ({ answer, displayed, drain }) => {
    answer.content = '\u56de\u7b54\u6b63\u6587\u3002<kb doc="unfinished'
    answer.done = true
    await nextTick()
    assert.equal(displayed.value, '', 'completion must still drain ordinary text')
    drain()
    assert.equal(displayed.value, answer.content)
  })
})

test('completed history and reconciliation preserve raw citation markup immediately', async () => {
  const content = '\u5386\u53f2\u56de\u7b54\u3002<kb doc="\u6587\u6863.docx" chunk_id="123" />'
  await withTypewriter(async ({ answer, displayed, pending }) => {
    assert.equal(displayed.value, content)
    assert.equal(pending(), 0)
    answer.content = content + '\u66f4\u65b0\u540e\u7684\u6b63\u6587\u3002'
    await nextTick()
    assert.equal(displayed.value, answer.content)
    assert.equal(pending(), 0)
  }, { content, done: true })
})

test('ordinary angle brackets and surrogate pairs survive citation pacing', async () => {
  await withTypewriter(async ({ answer, displayed, drain }) => {
    answer.content = 'Value < 5 🙂 <kbd>key</kbd> \u7ed3\u675f\u3002'
    await nextTick()
    for (const value of drain()) assert.doesNotMatch(value, /[\uD800-\uDBFF]$/u)
    assert.equal(displayed.value, answer.content)
  })
})

test('reveals Chinese text in compact phrase groups', () => {
  const text = '\u8fd9\u662f\u4e00\u4e2a\u81ea\u7136\u6d41\u7545\u7684\u56de\u7b54\u3002'
  assert.equal(nextTypewriterReveal(text, 0, 1), 0)
  assert.equal(nextTypewriterReveal(text, 0, 2), 2)
  assert.equal(nextTypewriterReveal(text, 0, 20), 4)
})

test('waits for a complete English word instead of revealing letter by letter', () => {
  const text = 'Hello world'
  assert.equal(nextTypewriterReveal(text, 0, 3), 0)
  assert.equal(nextTypewriterReveal(text, 0, 6), 6)
  assert.equal(text.slice(0, nextTypewriterReveal(text, 0, 6)), 'Hello ')
})

test('uses punctuation as a natural reveal boundary', () => {
  const text = '\u4f60\u597d\uff0c\u63a5\u4e0b\u6765\u7ee7\u7eed\u3002'
  assert.equal(nextTypewriterReveal(text, 0, 3), 3)
})

test('never splits a surrogate pair', () => {
  const text = '🙂 hello'
  assert.equal(nextTypewriterReveal(text, 0, 1), 2)
})

test('artifact reconciliation replaces completed text without replaying the typewriter', async () => {
  const originalRequest = globalThis.requestAnimationFrame
  const originalCancel = globalThis.cancelAnimationFrame
  const frames = new Map<number, FrameRequestCallback>()
  let id = 0
  globalThis.requestAnimationFrame = (callback) => { frames.set(++id, callback); return id }
  globalThis.cancelAnimationFrame = (frame) => { frames.delete(frame) }
  const message = reactive<any>({ content: '', agentEventStream: [{ type: 'answer', content: '', done: false }] })
  const answer = computed(() => message.agentEventStream[0])
  let displayed!: ReturnType<typeof useTypewriter>['displayed']
  // Mount the real composable with a minimal renderer so watches and lifecycle
  // cleanup run normally, without a browser-emulation dependency.
  const renderer = createRenderer<any, any>({
    patchProp() {}, insert() {}, remove() {}, setText() {}, setElementText() {},
    createElement: () => ({}), createText: () => ({}), createComment: () => ({}),
    parentNode: () => null, nextSibling: () => null,
  })
  const app = renderer.createApp({
    setup() {
      displayed = useTypewriter(() => answer.value.content, () => answer.value.done).displayed
      return () => null
    },
  })
  try {
    app.mount({})
    answer.value.content = 'PPT \u5df2\u751f\u6210\u3002![\u6bd4\u8d5b\u6587\u4ef6](sandbox:\u5f88\u957f\u7684\u6bd4\u8d5b\u4fe1\u606f\u6587\u4ef6\u540d.pptx)'
    await nextTick()
    assert.equal(displayed.value, '', 'live content must still animate')
    answer.value.done = true
    await nextTick()
    assert.equal(displayed.value, '', 'normal completion must let the remaining animation finish')
    for (let time = 16; frames.size && time < 10000; time += 16) {
      const pending = [...frames.values()]
      frames.clear()
      pending.forEach(callback => callback(time))
    }
    assert.equal(displayed.value, answer.value.content)
    for (const content of [
      'PPT \u5df2\u751f\u6210\u3002![\u6bd4\u8d5b\u6587\u4ef6](resource://short)',
      'PPT \u5df2\u751f\u6210\u3002![\u6bd4\u8d5b\u6587\u4ef6](resource://a-much-longer-persistent-resource-reference)',
      'PPT \u5df2\u751f\u6210\u3002![\u6bd4\u8d5b\u6587\u4ef6](resource://a-much-longer-persistent-resource-reference)\n\u5386\u53f2\u7248\u672c\u8bf4\u660e',
    ]) {
      applyFinalArtifactContent(message, content)
      await nextTick()
      assert.equal(displayed.value, content, 'completed corrections must appear immediately')
      assert.equal(frames.size, 0, 'completed corrections must not restart animation')
    }
  } finally {
    app.unmount()
    globalThis.requestAnimationFrame = originalRequest
    globalThis.cancelAnimationFrame = originalCancel
  }
})
