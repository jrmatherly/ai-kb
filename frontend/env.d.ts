/// <reference types="vite/client" />
// Fixes error ts(2307): cannot find module "@/views/login/index.vue" or its type declarations.
// Tells TypeScript that every file ending in .vue is a Vue component that can be imported.
declare module '*.vue' {
    import { Component } from 'vue'; const component: Component; export default component;
}

declare const __FRONTEND_VERSION__: string;
declare const __FRONTEND_COMMIT__: string;