# Identifiers deliberately left as upstream has them

Not in `map.tsv`. They name upstream services, third-party projects, or persisted schema that a
docs rename cannot change without a code and data migration. Revisit per surface when the code rebrand lands.

- `weknora_*` database, table, index and queue names; `weknora.db` (SQLite file); `weknora` Postgres/MySQL database and role names
- `WeKnoraCloud` as a model provider name inside `AIKB_WEKNORACLOUD_TIMEOUT` and the provider list
- `dsh-weknora` (the Dsh plugin name), `bsk` (BrowserSkill daemon), `Tencent/BrowserSkill`, `WeKnora-Chrome-Extension` build name
- Cookie, header, localStorage keys and API paths
- `WeKnora-Chrome-Extension` build output name (gitignored, not in docs)
- Historical text: `CHANGELOG.md`, release notes under `website-docs/07-releases/`, upstream issue links `Tencent/WeKnora#NNNN`
