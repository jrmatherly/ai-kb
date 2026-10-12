---
name: translate-docs
description: Translate one page of website-docs/ from Chinese to English for this copy of WeKnora, applying the AI KB rebrand and verifying every checkable claim against the code. Use for "translate website-docs", "translate this doc page", "English docs pass", or when a delegate is handed a single docs page to translate and verify. Encodes the glossary, the rebrand contract, the anchor rule, the verification contract and the findings format.
argument-hint: "<website-docs/.../page.md>"
allowed-tools: Bash(scripts/check-english-only.sh *) Bash(scripts/rebrand/check.sh *) Bash(scripts/rebrand/apply.sh *) Bash(node website-docs/scripts/check-links.mjs) Bash(node website-docs/scripts/check-anchors.mjs) Bash(git grep *) Bash(rg *) Read Grep Glob Edit Write
---

# Translate a docs page

One page per run. The page is rewritten in place in English, rebranded, and every claim a reader could act on is checked against the code. The exemplar for voice and structure is `website-docs/01-getting-started/01-introduction.md`; read it first.

## Contract

1. **Read the whole page**, then the code it describes. Verify before you write: env var names and defaults, ports, image names, file paths, config keys, API paths, command names, feature lists, version numbers. Evidence is a `path:line` you looked at. `codegraph_explore` or `rg` over `internal/`, `docker-compose.yml`, `.env.example`, `config/`, `frontend/src`, `docreader/`, `Makefile`, `scripts/`.
2. **Write English prose, not a gloss.** Short declarative sentences. Keep headings, tables, code blocks, Mermaid diagrams, `<Screenshot>` blocks, `::: tip` containers and link targets. Translate Mermaid labels, screenshot `caption` and `hint`, table cells and comments inside code blocks. Keep commands and identifiers byte-exact except for the rebrand rows.
3. **Rebrand with the contract**, `scripts/rebrand/map.tsv`, and nothing else. Run `scripts/rebrand/apply.sh <page>` after writing, then `scripts/rebrand/check.sh <page>` must exit 0. `scripts/rebrand/unchanged.md` lists identifiers that stay as upstream has them (`weknora_*` database names, `WeKnoraCloud`, `dsh-weknora`, `bsk`). Do not invent other renames. Do not mention upstream by name in product pages.
4. **Decided deviations are written as the target state** (see "Deviations decided" in `.claude/skills/upstream-review/SKILL.md`): the UI ships in English only; non-English READMEs do not exist. Each such claim gets a DEVIATION-PENDING findings row so the code work is tracked.
5. **Doc disagrees with code: fix the doc, log the finding.** Never change code. If the code looks wrong (the doc described the intended behavior), keep the doc on the intended behavior, add one sentence on the actual behavior, and log CODE-BUG.
6. **Anchors.** Remove Chinese custom anchors (`{#...}` containing CJK). Keep ASCII custom anchors (other pages link to them). Links inside your page to headings you renamed get the new slug: lowercase, every run of whitespace or punctuation becomes one hyphen, leading and trailing hyphens trimmed, a leading digit gets `_` in front (`website-docs/scripts/check-anchors.mjs` exports the exact `slugify`; run it rather than guessing). Do not edit other pages: list inbound links from other pages that point to anchors you removed (`rg -n '<your-file>#' website-docs`) in your report.
7. **No em dashes** anywhere (U+2014). Use a plain dash, a comma or a new sentence.
8. **Gates on your page before you report**: `scripts/check-english-only.sh <page>` exit 0; `scripts/rebrand/check.sh <page>` exit 0; `rg -c -e '\x{2014}' <page>` prints nothing (exit 1); `node website-docs/scripts/check-links.mjs` and `node website-docs/scripts/check-anchors.mjs` report no failures caused by your page.

## Glossary

English terms follow the product UI (`frontend/src/i18n/locales/en-US.ts`). Code identifiers in parentheses stay as written.

| Chinese | English |
| --- | --- |
| 租户 / 空间 / 工作空间 | workspace (code: Tenant); "tenant" only when naming the code type |
| 知识库 | knowledge base (KnowledgeBase) |
| 知识 / 文档 | knowledge item / document (Knowledge) |
| 分块 | chunk |
| 入库 / 解析 | import / parsing; status values stay `pending`, `processing`, `finalizing`, `completed`, `failed` |
| 智能体 / Agent | agent |
| 快速问答 / 智能推理 / 数据分析 | Quick Answer / Smart Reasoning / Data Analysis (modes `quick-answer`, `smart-reasoning`) |
| 产物 | Artifacts |
| 技能 / 沙箱 | skills / sandbox |
| 长期记忆 | long-term memory |
| 数据源 | data source |
| 检索 / 混合检索 / 重排 | retrieval / hybrid retrieval / rerank |
| 向量模型 / 对话模型 / 视觉模型 / 语音转写 | embedding model / chat model (LLM) / vision model (VLM) / speech-to-text (ASR) |
| 兜底 / 改写 / 查询扩展 / 意图识别 | fallback / rewrite / query expansion / intent recognition |
| 文档画像 | document profile |
| 引用 / 引用来源 | citation / source |
| 预设 | preset |
| 邀请 / 组织 / 成员 / 角色 | invitation / organization / member / role (Owner, Admin, Contributor, Viewer) |
| 系统管理员 / 平台管理 | system administrator / platform administration |
| 「设置 → 系统」 / 系统信息 / 系统设置 | Settings → System / System Information / System Settings |
| 嵌入渠道 / IM 集成 / 渠道 | embed channel / IM integration / channel |
| 本机浏览器 | Local Browser |
| 工具箱 | Toolbox |
| 审计日志 | audit log |
| 迁移 | migration |
| 自建 / 托管 / 私有化 | self-hosted / managed / on your own infrastructure |
| 排障 / 常见问题 | troubleshooting / FAQ |
| 版本升级 | upgrading |

## Findings row

Append to the fragment file you were given (never the shared log), one row per claim you checked that was not simply VERIFIED, plus one VERIFIED row summarizing the page with the evidence paths:

```
| # | Doc | Claim (as shipped) | Evidence (path:line) | Verdict | Doc now says | Follow-up |
```

Verdicts: VERIFIED, CORRECTED, OPEN (needs investigation; doc states the code's behavior), CODE-BUG, DEVIATION-PENDING.

## Report

Lines, not prose: page, gates (each with its exit code), findings count by verdict, inbound links from other pages that now need updating (file:line, target), anything you could not verify and why.
