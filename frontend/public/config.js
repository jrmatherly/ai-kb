// Runtime config: local dev defaults. In Docker, docker-entrypoint.sh overwrites this file.
window.__RUNTIME_CONFIG__ = {
  MAX_FILE_SIZE_MB: 50,
  MAX_SKILL_BUNDLE_SIZE_MB: 256,
  // Optional: serve embed on a dedicated origin, e.g. 'https://embed.example.com'
  EMBED_BASE_URL: '',
  // Optional: default UI locale for first-time visitors (en-US is the only supported value)
  DEFAULT_LOCALE: '',
};
