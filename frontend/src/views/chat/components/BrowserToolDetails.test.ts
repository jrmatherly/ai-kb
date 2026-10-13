import assert from 'node:assert/strict'
import { after, before, test } from 'node:test'
import { fileURLToPath } from 'node:url'
import { createServer, type ViteDevServer } from 'vite'
import vue from '@vitejs/plugin-vue'
import { createSSRApp, h, type Component } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { createI18n } from 'vue-i18n'
import enUS from '../../../i18n/locales/en-US'

let server: ViteDevServer
let Card: Component
before(async () => {
  server = await createServer({
    configFile: false, optimizeDeps: { noDiscovery: true, entries: [] }, plugins: [vue()],
    resolve: { alias: { '@': fileURLToPath(new URL('../../../', import.meta.url)) } },
    server: { middlewareMode: true, hmr: false }, appType: 'custom',
  })
  Card = (await server.ssrLoadModule('/src/views/chat/components/BrowserToolDetails.vue')).default
})
after(async () => { await server?.close() })
async function render(event: Record<string, unknown>) {
  const app = createSSRApp({ render: () => h(Card, { event }) })
  app.use(createI18n({ legacy: false, locale: 'en-US', messages: { 'en-US': enUS } }))
  return renderToString(app)
}
test('observation card renders readable content and escapes page HTML', async () => {
  const html = await render({success:true, output:JSON.stringify({text:'Search results <script>alert(1)</script>',tab_id:42,ref_count:3})})
  assert.match(html, /Search results &lt;script&gt;/)
  assert.doesNotMatch(html, /<script>|Technical details|tab_id|ref_count|\[object Object\]/)
})
test('screenshots render as images and tabs render as rows without raw JSON', async () => {
  const image = await render({success:true,output:{image_base64:'aGVsbG8=',format:'png'}})
  assert.match(image, /<img[^>]+src="data:image\/png;base64,aGVsbG8="/)
  assert.doesNotMatch(image, /<pre|image_base64/)
  const tabs = await render({success:true,output:JSON.stringify({tabs:[{tab_id:7,title:'News',url:'https://user:password@example.com/news?token=secret'}]})})
  assert.match(tabs, /<strong[^>]*>News<\/strong>/)
  assert.match(tabs, /https:\/\/example.com\/news/)
  assert.doesNotMatch(tabs, /password|secret|tab_id|Technical details/)
})
test('interrupted and historical actions do not claim success', async () => {
  const failed = await render({success:false,error:'browser command interrupted or timed out'})
  assert.match(failed, /The browser action was interrupted/)
  assert.doesNotMatch(failed, /Action completed/)
  const old = await render({output:'{}'})
  assert.match(old, /Browser action recorded/)
  assert.doesNotMatch(old, /Action completed/)
})

test('flat argument validation failures identify invalid tool input', async () => {
 const html = await render({success:false,error:"Parameter validation failed: navigate requires url at the top level"})
 assert.match(html, /Browser tool arguments are invalid or incomplete/)
 assert.doesNotMatch(html, /Check the page and try again/)
})

test('diagnostics, script values, and navigation errors are rendered safely', async () => {
 const logs = await render({arguments:{method:'console'},success:true,output:{entries:[{level:'error',text:'API <script>alert(1)</script>'}]}})
 assert.match(logs,/API &lt;script&gt;/)
 assert.doesNotMatch(logs,/<script>/)
 const value = await render({arguments:{method:'evaluate'},success:true,output:{ok:true,value:{count:123}}})
 assert.match(value,/count/)
 assert.match(value,/123/)
 const navigation = await render({arguments:{method:'navigate'},success:true,output:{reached:'timeout',error_text:'Loading did not finish'}})
 assert.match(navigation,/Loading did not finish/)
 assert.match(navigation,/did not reach the requested loading phase/)
 assert.doesNotMatch(navigation,/Action completed/)
 const image = await render({arguments:{method:'screenshot'},success:true,output:'{"width":1}',tool_data:{image_base64:'aGVsbG8=',format:'png'}})
 assert.match(image,/<img[^>]+src="data:image\/png;base64,aGVsbG8="/)
})
