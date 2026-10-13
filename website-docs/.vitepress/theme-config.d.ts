import 'vitepress'

declare module 'vitepress/dist/client/theme-default/config' {
  export interface ThemeConfig {
    /** AI KB release version shown on the docs site (from VERSION at the repository root) */
    aikbVersion?: string
  }
}
