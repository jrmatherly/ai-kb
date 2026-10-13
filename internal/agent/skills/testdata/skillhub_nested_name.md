---
name: Fortune Master
  version: 1.2.6
  description: |
    All-in-one fortune master - BaZi Four Pillars, Zi Wei Dou Shu, Qi Men Dun Jia, Liu Yao, Mei Hua Yi Shu, tarot, astrology charts,
    numerology, Nine Star feng shui, palm and face reading, naming, outfit matching, and auspicious-date matchmaking readings. For cultural reference only. It does not replace medical, legal, psychological, financial, marriage, naming or other
    professional advice; consult a professional before major decisions.
    [Data and privacy notice] This skill stores on the local file system the birth date and time, place,
    name, optional family member details, interaction topic history (used to track preferences, off by default) the user submits, and push task logs.
    No data is uploaded; it stays only in the local data/ directory. Users can view, edit or delete it at any time with profile.js.
    Daily push is opt-in and off by default.
metadata:
  displayName: "Fortune Master"
  author:
    - "Tencent Senior R&D - enoyao"
    - "Tencent Senior Product Operations - rekyhe"
  version: 1.2.0
  keywords: BaZi chart, Zi Wei Dou Shu, Qi Men Dun Jia, Liu Yao, Mei Hua Yi Shu, tarot cards, astrology chart, Nine Star, palm lines, palm reading, face reading, naming, name choosing, name picking, outfit, matching, colors, five-element colors, lucky colors, today's fortune, daily fortune, matchmaking, auspicious dates, life path number, geomancy, fortune reading, BaZi, ZiWei, QiMen, Tarot, feng shui, I Ching, numerology, daily horoscope, palmistry, physiognomy, naming, dressing
  # Trigger keywords tightened: 1.1.8 removed overly broad words such as "suan ming / divination / destiny / numerology / fortune telling / astrology",
  # so this skill does not activate in conversations where the user did not ask for divination; it triggers only on a named system or a specific topic.
  openclaw:
    emoji: "☯️"
    skillKey: "university-applications"
    runtime:
      node: ">=18"
      python3: true
    install:
      - kind: node
        package: iztro
    env: []  # This skill no longer reads any environment variables; the former OPENCLAW_KNOWLEDGE_DIR was removed to avoid file-system enumeration.
    security:
      network:
        default: none
        optional:
          - feature: "liuyao HTML LLM divination (user-initiated, in-browser, OFF by default)"
            allowed-endpoints:
              - "https://api.openai.com"
              - "https://api.anthropic.com"
              - "https://api.deepseek.com"
            custom-endpoint: "only after explicit in-UI consent dialog; HTTPS-only, no IP/localhost auto-trust"
            data-sent: "only the hexagram and the user's typed question; no profile, no env vars, no file paths"
            credential: "user-provided LLM API key, entered at runtime, stored in browser localStorage only"
          - feature: "liuyao HTML Google Fonts (commented out by default)"
            endpoint: "https://fonts.googleapis.com"
            data-sent: "none beyond standard font request"
            credential: none
      credentials:
        bundled: none
        required: none
        user-optional:
          - "LLM API key for liuyao/index.html divination feature (scope it to a separate limited key, never reused)"
      push-mechanism: openclaw-ipc
      push-optin: true
      push-default-state: disabled
      data-retention:
        location: "local filesystem under data/profiles/ and data/push-log.json"
        remote-upload: none
        user-controls:
          - "view:   node scripts/profile.js show <userId>"
          - "list:   node scripts/profile.js list"
          - "edit:   node scripts/profile.js save <userId> <field> <value>"
          - "delete: node scripts/profile.js delete <userId>"
          - "disable push: node scripts/push-toggle.js off <userId>"
      notes: |
        All bundled scripts perform local computation only — no fetch, axios,
        https.request, curl, wget, or any outbound network calls from the Node/Python
        side. Push delivery is handled entirely by the OpenClaw runtime via stdout/IPC
        protocol, and is OPT-IN (disabled until the user runs push-toggle.js on).
        The 'channels' field in user profiles (e.g. telegram) is a routing hint for
        the OpenClaw runtime, not a direct API integration. This skill does not hold
        or require any third-party API tokens (Telegram Bot Token, SMTP credentials,
        webhook URLs, etc.). The local-only release helper script is excluded
        from the published bundle via .clawhubignore and is not part of the
        installed skill surface.
        EXCEPTION — OPTIONAL LLM NETWORK USE: the browser-only file liuyao/index.html
        exposes an optional "LLM divination" button. If and only if the user clicks
        it and fills in their own API key + endpoint, the browser (not the skill
        process) will POST the hexagram and question to that user-configured endpoint.
        No key is bundled, hardcoded, or transmitted anywhere else. Users are advised
        to supply a scoped/limited API key rather than a primary account key.
        User profile data (birth details, optional family members, interaction log)
        is stored only on the local filesystem and can be viewed, edited, or deleted
        at any time via scripts/profile.js (see data-retention.user-controls above).
---

# body
