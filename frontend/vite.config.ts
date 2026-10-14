import { fileURLToPath, URL } from 'node:url'
import { resolve, dirname } from 'node:path'
import { existsSync } from 'node:fs'
import { execSync } from 'node:child_process'
import { createRequire } from 'node:module'
import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueJsx from '@vitejs/plugin-vue-jsx'

const __dirname = dirname(fileURLToPath(import.meta.url))
const require = createRequire(import.meta.url)

const pkg = require('./package.json') as { version?: string }
const FRONTEND_VERSION = pkg.version ?? 'unknown'

function resolveFrontendCommit(): string {
  const fromEnv = process.env.VITE_FRONTEND_COMMIT || process.env.GITHUB_SHA
  if (fromEnv) {
    return fromEnv.slice(0, 7)
  }
  try {
    return execSync('git rev-parse --short HEAD', { stdio: ['ignore', 'pipe', 'ignore'] })
      .toString()
      .trim()
  } catch {
    return 'unknown'
  }
}

const FRONTEND_COMMIT = resolveFrontendCommit()

/** Dev parity with nginx: serve embed.html for /embed/:channelId (not the main SPA). */
function embedHtmlDevFallback(): Plugin {
  return {
    name: 'embed-html-dev-fallback',
    configureServer(server) {
      server.middlewares.use((req, _res, next) => {
        const raw = req.url ?? ''
        const qIdx = raw.indexOf('?')
        const path = qIdx >= 0 ? raw.slice(0, qIdx) : raw
        const qs = qIdx >= 0 ? raw.slice(qIdx) : ''
        if (path.startsWith('/embed/') && path !== '/embed.html' && !path.includes('.')) {
          req.url = `/embed.html${qs}`
        }
        next()
      })
    },
  }
}
const DEV_PROXY_TARGET =
  process.env.VITE_DEV_PROXY_TARGET ||
  process.env.FRONTEND_BACKEND_URL ||
  'http://localhost:8080'

function resolveVueOfficePptxEntry(): string {
  try {
    const pkgDir = dirname(require.resolve('@vue-office/pptx/package.json'))
    const candidates = [
      resolve(pkgDir, 'lib/v3/index.js'),
      resolve(pkgDir, 'lib/index.js'),
      resolve(pkgDir, 'lib/v3/vue-office-pptx.mjs'),
    ]
    const matched = candidates.find((candidate) => existsSync(candidate))
    return matched ?? '@vue-office/pptx'
  } catch {
    return '@vue-office/pptx'
  }
}

export default defineConfig({
  define: {
    __FRONTEND_VERSION__: JSON.stringify(FRONTEND_VERSION),
    __FRONTEND_COMMIT__: JSON.stringify(FRONTEND_COMMIT),
  },
  build: {
    modulePreload: {
      resolveDependencies(_filename, deps, { hostId }) {
        // Embed iframe bootstraps with token exchange only; defer heavy chat chunks.
        if (hostId?.includes('embed')) {
          return deps.filter((dep) => !(
            dep.includes('vendor-mermaid')
            || dep.includes('vendor-highlight')
            || dep.includes('vendor-markdown')
            || dep.includes('vendor-tdesign')
            || dep.includes('botmsg')
            || dep.includes('usermsg')
            || dep.includes('EmbedBotMessage')
            || dep.includes('EmbedUserMessage')
            || dep.includes('AgentStreamDisplay')
            || dep.includes('EmbedChatCore')
            || dep.includes('vendor-markdown')
            || dep.includes('fonts-')
          ))
        }
        return deps
      },
    },
    rollupOptions: {
      input: {
        main: resolve(__dirname, 'index.html'),
        embed: resolve(__dirname, 'embed.html'),
      },
      output: {
        // Match package directories, not substrings: tdesign-icons ships highlight.js and
        // mermaid.js icon modules. A manual chunk also absorbs shared deps it reaches, so
        // helpers used everywhere are pinned to vendor-shared; otherwise the entry would
        // statically import the heavy chunks to get Vue, lodash-es or the preload helper.
        manualChunks(id) {
          if (id.includes('vite/preload-helper') || id.includes('commonjsHelpers')) return 'vendor-shared'
          if (!id.includes('/node_modules/')) return
          if (/\/node_modules\/(mermaid|@mermaid-js|cytoscape[^/]*|dagre-d3-es|layout-base|cose-base)\//.test(id)) {
            return 'vendor-mermaid'
          }
          if (/\/node_modules\/(marked|marked-katex-extension|katex)\//.test(id)) return 'vendor-markdown'
          if (/\/node_modules\/highlight\.js\//.test(id)) return 'vendor-highlight'
          if (/\/node_modules\/(lodash-es|dompurify|es-toolkit)\//.test(id)) return 'vendor-shared'
        },
      },
    },
  },
  plugins: [
    vue(),
    vueJsx(),
    embedHtmlDevFallback(),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      '@vue-office/pptx': resolveVueOfficePptxEntry(),
    },
  },
  server: {
    port: 5173,
    host: true,
    // Proxy config for the dev environment
    proxy: {
      '/mcp/': {
        target: DEV_PROXY_TARGET,
        changeOrigin: true,
        secure: false,
        // Streamable HTTP may keep an SSE response open for long-running tools.
        timeout: 3_600_000,
        proxyTimeout: 3_600_000,
      },
      '/api': {
        target: DEV_PROXY_TARGET,
        changeOrigin: true,
        secure: false,
        // WebSocket upgrades such as the sandbox terminal also go through /api, so WS forwarding must be on,
        // otherwise the browser handshake fails and the frontend shows "connecting" forever.
        ws: true,
        // Cube fork snapshots pause a live MicroVM; 30s axios/proxy defaults
        // abort the POST and the backend then 500s on a canceled persist.
        timeout: 180_000,
        proxyTimeout: 180_000,
      },
      '/files': {
        target: DEV_PROXY_TARGET,
        changeOrigin: true,
        secure: false,
      }
    }
  },
  // `vite preview` serves the production build (dist) locally, the closest environment to the release image:
  // the same minification, chunking and CSS load order, so problems that only appear in the production build show up early
  // (such as theme variables overridden by bundle order). Usage: npm run build && npm run preview
  preview: {
    port: 4173,
    host: true,
    proxy: {
      '/mcp/': {
        target: DEV_PROXY_TARGET,
        changeOrigin: true,
        secure: false,
        timeout: 3_600_000,
        proxyTimeout: 3_600_000,
      },
      '/api': {
        target: DEV_PROXY_TARGET,
        changeOrigin: true,
        secure: false,
        ws: true,
        timeout: 180_000,
        proxyTimeout: 180_000,
      },
      '/files': {
        target: DEV_PROXY_TARGET,
        changeOrigin: true,
        secure: false,
      }
    }
  }
})
