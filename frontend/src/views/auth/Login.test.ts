import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { runInNewContext } from 'node:vm'
import test from 'node:test'
import ts from 'typescript'
import { compileScript, parse } from '@vue/compiler-sfc'
import * as vue from 'vue'

const source = readFileSync(new URL('./Login.vue', import.meta.url), 'utf8')
const { descriptor } = parse(source)
const compiled = ts.transpileModule(compileScript(descriptor, {
  id: 'login-test', inlineTemplate: true,
  // The animated background compiles to static HTML strings the object renderer cannot insert.
  templateOptions: { compilerOptions: { hoistStatic: false } },
}).content, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
}).outputText
type Host = { type: string; props: Record<string, any>; children: Host[]; text: string; parent?: Host }
const node = (type: string): Host => ({ type, props: {}, children: [], text: '' })
const renderer = vue.createRenderer<Host, Host>({
  createElement: node,
  createText: text => ({ ...node('#text'), text }),
  createComment: text => ({ ...node('#comment'), text }),
  patchProp: (el, key, _old, value) => { el.props[key] = value },
  setText: (el, text) => { el.text = text },
  setElementText: (el, text) => { el.children = []; el.text = text },
  parentNode: el => el.parent || null,
  nextSibling: el => el.parent?.children[el.parent.children.indexOf(el) + 1] || null,
  insert(el, parent, anchor) {
    if (el.parent) el.parent.children.splice(el.parent.children.indexOf(el), 1)
    el.parent = parent
    const index = anchor ? parent.children.indexOf(anchor) : -1
    parent.children.splice(index < 0 ? parent.children.length : index, 0, el)
  },
  remove(el) {
    el.parent?.children.splice(el.parent.children.indexOf(el), 1)
    el.parent = undefined
  },
})
const all = (el: Host, predicate: (node: Host) => boolean): Host[] => [
  ...(predicate(el) ? [el] : []), ...el.children.flatMap(child => all(child, predicate)),
]
const hasClass = (name: string) => (el: Host) => String(el.props.class || '').split(/\s+/).includes(name)

async function fixture({ oidc, registration = 'self_serve' }: { oidc: boolean; registration?: string }) {
  let answerOIDC!: () => void
  const oidcAnswered = new Promise<void>(resolve => { answerOIDC = resolve })
  const api = {
    autoSetup: async () => { throw new Error('not lite') },
    getAuthConfig: async () => ({ success: true, registration_mode: registration, complex_password_enabled: false }),
    getOIDCConfig: async () => {
      await oidcAnswered
      return { success: true, enabled: oidc, provider_display_name: 'PocketID' }
    },
  }
  const exports: { default?: vue.Component } = {}
  runInNewContext(compiled, { exports, console, localStorage: { removeItem() {} }, require(name: string) {
    if (name === 'vue') return vue
    if (name === 'vue-router') return { useRoute: () => ({ query: {} }), useRouter: () => ({ replace() {} }) }
    if (name === 'vue-i18n') return { useI18n: () => ({ t: (key: string) => key, tm: () => [] }) }
    if (name === '@/api/auth') return api
    if (name === '@/stores/auth') return { useAuthStore: () => ({ isLoggedIn: false }) }
    if (name === '@/stores/siteLinks') return { useSiteLinksStore: () => ({ docsUrl: '', githubUrl: '', ensureLoaded: async () => {} }) }
    if (name === './registrationPolicy') return { canRegister: (mode: string) => mode === 'self_serve' }
    if (name === '@/composables/useRoleLabel') return { useRoleLabel: () => ({}) }
    if (name === '@/utils/passwordPolicy') return { newPasswordRules: () => [] }
    if (name === 'tdesign-vue-next') return { MessagePlugin: {} }
    if (name === 'swiper/vue') return { Swiper: stub('swiper'), SwiperSlide: stub('swiper-slide') }
    return { default: '' }
  } })
  const root = node('root')
  const app = renderer.createApp(exports.default!)
  app.config.globalProperties.$t = ((key: string) => key) as typeof app.config.globalProperties.$t
  for (const name of ['t-form', 't-form-item', 't-input', 't-button', 't-icon']) app.component(name, stub(name))
  app.mount(root)
  const settle = async () => { for (let i = 0; i < 5; i++) { await new Promise(r => setImmediate(r)); await vue.nextTick() } }
  await settle()
  const count = (predicate: (el: Host) => boolean) => all(root, predicate).length
  return {
    count,
    answer: async () => { answerOIDC(); await settle() },
    click: async (predicate: (el: Host) => boolean) => { await all(root, predicate)[0].props.onClick(); await settle() },
    close: () => app.unmount(),
  }
}

function stub(type: string) {
  return vue.defineComponent({ setup: (_props, { slots }) => () => vue.h(type, {}, slots.default?.()) })
}

const form = (el: Host) => el.type === 't-form'
const oidcButton = hasClass('oidc-button')
const toggle = hasClass('password-login-toggle')
const register = hasClass('register-cta')

test('the login form waits for the OIDC config instead of painting and collapsing', async t => {
  const f = await fixture({ oidc: true }); t.after(f.close)
  assert.equal(f.count(form), 0)
  assert.equal(f.count(oidcButton), 0)
  assert.equal(f.count(hasClass('login-config-placeholder')), 1)
})

test('with OIDC off the password form shows and no OIDC parts do', async t => {
  const f = await fixture({ oidc: false }); t.after(f.close)
  await f.answer()
  assert.equal(f.count(form), 1)
  assert.equal(f.count(oidcButton), 0)
  assert.equal(f.count(toggle), 0)
  assert.equal(f.count(register), 1)
})

test('with OIDC on the form starts collapsed, registration stays reachable, the toggle expands it', async t => {
  const f = await fixture({ oidc: true }); t.after(f.close)
  await f.answer()
  assert.equal(f.count(form), 0)
  assert.equal(f.count(oidcButton), 1)
  assert.equal(f.count(toggle), 1)
  assert.equal(f.count(register), 1)
  await f.click(toggle)
  assert.equal(f.count(form), 1)
  assert.equal(f.count(toggle), 0)
  assert.equal(f.count(register), 1)
})

test('no registration entry when registration is off', async t => {
  const f = await fixture({ oidc: true, registration: '' }); t.after(f.close)
  await f.answer()
  assert.equal(f.count(register), 0)
})
