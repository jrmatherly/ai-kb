import { readFileSync, readdirSync } from 'node:fs'
import { resolve } from 'node:path'
import { defineConfig, type DefaultTheme } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'
import './theme-config.d.ts'
import { repoVersionLabel } from './version'

const root = resolve(import.meta.dirname, '..')

const sections: { dir: string; label: string }[] = [
  { dir: '01-getting-started', label: 'Getting started' },
  { dir: '02-architecture', label: 'Architecture' },
  { dir: '03-features', label: 'Features' },
  { dir: '04-api', label: 'API reference' },
  { dir: '05-clients', label: 'Clients' },
  { dir: '06-development', label: 'Development' },
]

/** Sidebar entry text is the page's H1, verbatim. */
function itemText(dir: string, file: string): string {
  const raw = readFileSync(resolve(root, dir, file), 'utf-8')
  return (raw.match(/^#\s+(.+)$/m)?.[1] ?? file.replace(/\.md$/, '')).trim()
}

function itemsOf(dir: string): DefaultTheme.SidebarItem[] {
  const files = readdirSync(resolve(root, dir)).filter((f) => f.endsWith('.md'))
  return files.sort()
    .map((f) => ({
      text: itemText(dir, f),
      link: `/${dir}/${f.replace(/\.md$/, '')}`,
    }))
}

const sidebar: DefaultTheme.SidebarItem[] = sections.map((s) => ({
  text: s.label,
  collapsed: false,
  items: itemsOf(s.dir),
}))

const repo = 'https://github.com/jrmatherly/ai-kb'

export default withMermaid(
  defineConfig({
    title: 'AI KB',
    titleTemplate: ':title · AI KB Docs',
    description: 'AI KB documentation: deployment, configuration, features, API reference and extension development',
    lang: 'en-US',
    base: '/docs/',
    cleanUrls: true,
    appearance: { storageKey: 'vitepress-theme-appearance' },
    lastUpdated: true,
    srcExclude: ['README.md', 'MIGRATION.md', 'homepage/**', 'shared/**', 'scripts/**', 'deploy/**', 'static-site/**', 'releases/**'],
    metaChunk: true,
    transformPageData(pageData) {
      // The shared masthead replaces the default documentation navbar.
      pageData.frontmatter.navbar = false
    },

    head: [
      ['link', { rel: 'icon', href: '/docs/favicon.ico', type: 'image/x-icon' }],
      ['meta', { name: 'theme-color', content: '#101f38' }],
      ['meta', { property: 'og:type', content: 'website' }],
      ['meta', { property: 'og:title', content: 'AI KB Docs' }],
      [
        'meta',
        {
          property: 'og:description',
          content: 'Bring PDFs, Word files, web pages and Feishu, Notion or Yuque content into a knowledge base, and get answers with cited sources',
        },
      ],
    ],

    markdown: {
      theme: { light: 'github-light', dark: 'github-dark' },
      lineNumbers: false,
      toc: { level: [2, 3] },
      config(md) {
        // Wrap tables in a scroll container. The default theme makes <table> itself
        // display:block to scroll horizontally, which shrinks narrow tables below the
        // text column. With the wrapper scrolling, tables keep display:table and width:100%.
        md.renderer.rules.table_open = () => '<div class="wk-table">\n<table>\n'
        md.renderer.rules.table_close = () => '</table>\n</div>\n'
      },
    },

    themeConfig: {
      logoLink: { link: '/docs/', target: '_self' },
      siteTitle: 'AI KB',

      nav: [],

      aikbVersion: repoVersionLabel,

      sidebar,

      socialLinks: [{ icon: 'github', link: repo }],

      outline: { level: [2, 3], label: 'On this page' },

      docFooter: { prev: 'Previous page', next: 'Next page' },
      returnToTopLabel: 'Return to top',
      sidebarMenuLabel: 'Menu',
      darkModeSwitchLabel: 'Appearance',
      lightModeSwitchTitle: 'Switch to light theme',
      darkModeSwitchTitle: 'Switch to dark theme',

      lastUpdated: {
        text: 'Last updated',
        formatOptions: { dateStyle: 'medium', timeStyle: undefined },
      },

      editLink: {
        pattern: `${repo}/edit/main/website-docs/:path`,
        text: 'Edit this page on GitHub',
      },

      search: {
        provider: 'local',
        options: {
          translations: {
            button: { buttonText: 'Search docs', buttonAriaLabel: 'Search docs' },
            modal: {
              displayDetails: 'Display detailed list',
              resetButtonTitle: 'Reset search',
              backButtonTitle: 'Close search',
              noResultsText: 'No results for',
              footer: {
                selectText: 'to select',
                navigateText: 'to navigate',
                closeText: 'to close',
              },
            },
          },
          miniSearch: {
            searchOptions: {
              fuzzy: 0.2,
              prefix: true,
              boost: { title: 4, text: 2, titles: 1 },
            },
          },
        },
      },

      footer: {
        message: `AI KB ${repoVersionLabel} · MIT License`,
        copyright: '© 2026 Jason Matherly · Based on <a href="https://github.com/Tencent/WeKnora">Tencent/WeKnora</a>',
      },
    },

    mermaid: {
      theme: 'base',
      // mermaid 12 defaults to ELK, the neo look and 120px label wrapping; these keep the 11.x rendering.
      layout: 'dagre',
      look: 'classic',
      flowchart: { wrappingWidth: 200, minNodeWidth: 0 },
      state: { wrappingWidth: 200, minNodeWidth: 0 },
      fontFamily:
        '"PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", ui-sans-serif, sans-serif',
      themeVariables: {
        primaryColor: '#eef1f6',
        primaryTextColor: '#101f38',
        primaryBorderColor: '#9aa8bd',
        lineColor: '#7d8ba1',
        secondaryColor: '#faf7ef',
        tertiaryColor: '#f6f7f9',
        fontSize: '14px',
      },
    },
  }),
)
