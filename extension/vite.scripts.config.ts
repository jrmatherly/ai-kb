import { resolve } from 'node:path';
import { defineConfig } from 'vite';

// One IIFE per script: `--mode background` and `--mode content` select the entry.
const ENTRIES: Record<string, string> = {
  background: 'src/background/index.ts',
  content: 'src/content/index.ts',
};

export default defineConfig(({ mode }) => {
  const entry = ENTRIES[mode];
  if (!entry) throw new Error(`unknown script mode ${mode}; use background or content`);
  return {
    publicDir: false,
    build: {
      outDir: 'dist',
      emptyOutDir: false,
      lib: {
        entry: resolve(__dirname, entry),
        name: mode,
        formats: ['iife'],
        fileName: () => `${mode}.js`,
      },
    },
  };
});
