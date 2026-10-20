// Real storage calls yield to other work; these do too, so races can interleave.
export const tick = (ms = 1) => new Promise<void>((r) => setTimeout(r, ms));

type Listener = (changes: Record<string, { oldValue?: unknown; newValue?: unknown }>, area: string) => void;

function area(name: string, data: Map<string, unknown>, listeners: Set<Listener>) {
  const emit = (changes: Record<string, { oldValue?: unknown; newValue?: unknown }>) => {
    for (const listener of listeners) listener(changes, name);
  };
  return {
    get: async (key: string | string[] | null) => {
      await tick();
      if (key === null) return Object.fromEntries(data);
      return Object.fromEntries([key].flat().filter((k) => data.has(k)).map((k) => [k, data.get(k)]));
    },
    set: async (items: Record<string, unknown>) => {
      await tick();
      const changes = Object.fromEntries(Object.entries(items).map(([key, value]) => [key, { oldValue: data.get(key), newValue: value }]));
      for (const [key, value] of Object.entries(items)) data.set(key, value);
      emit(changes);
    },
    remove: async (keys: string | string[]) => {
      await tick();
      const changes = Object.fromEntries([keys].flat().filter((key) => data.has(key)).map((key) => [key, { oldValue: data.get(key) }]));
      for (const key of [keys].flat()) data.delete(key);
      if (Object.keys(changes).length > 0) emit(changes);
    },
  };
}

// Installs a fresh chrome.storage on globalThis and returns its two areas' contents.
export function installFakeChrome() {
  const session = new Map<string, unknown>();
  const local = new Map<string, unknown>();
  const listeners = new Set<Listener>();
  const onChanged = { addListener: (l: Listener) => listeners.add(l), removeListener: (l: Listener) => listeners.delete(l) };
  (globalThis as unknown as { chrome: unknown }).chrome = { storage: { session: area('session', session, listeners), local: area('local', local, listeners), onChanged } };
  return { session, local, listeners };
}
