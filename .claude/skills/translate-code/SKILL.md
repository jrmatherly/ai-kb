---
name: translate-code
description: Translate one batch of this repo's code (Go, Vue/TS, Python, SQL, YAML, shell) from Chinese to English without changing behavior, and prove it. Use for "translate the code", "English-only pass", "translate internal/<pkg>", "remove the Chinese from <file>", any batch of the CJK cleanup outside website-docs/, or when a delegate is handed a package or file list to translate. Encodes which Chinese must stay (as \u escapes), the persisted and wire-coupled strings, the decided defaults, the proof tool and the gates.
argument-hint: "<package dir or file list>"
allowed-tools: Bash(scripts/check-english-only.sh *) Bash(go run ./scripts/translation-check *) Bash(scripts/validate.sh *) Bash(make docs) Bash(git grep *) Bash(git diff *) Bash(git show *) Bash(rg *) Read Grep Glob Edit Write
---

# Translate a code batch

A batch is one Go package, one frontend area, or one producer plus every consumer of its strings. The goal is English text with byte-identical behavior: identifiers, keys, format verbs, placeholders and control flow never change. Terms follow the glossary in `.claude/skills/translate-docs/SKILL.md`, so the code and the docs say the same thing.

Find the work: `scripts/check-english-only.sh -n <pathspec>`.

## What each kind of Chinese becomes

| Kind | Do |
| --- | --- |
| Comments, docstrings, `<!-- -->`, SQL `--` comments | Translate. Keep a comment only if it states a non-obvious why; a comment that narrates the code can go. |
| Swag annotations (`// @Summary`, `// @Param`, `// @Description`) | Translate, then `make docs`; never edit `docs/` by hand. |
| Log messages, `fmt.Errorf`/`errors.New`, `AppError` messages | Translate. No client compares API error text; clients key on the numeric code. |
| User-visible server text (IM replies, tool display names, progress steps, placeholders) | Translate, together with every test that asserts it. |
| LLM prompts (Go raw strings, `config/prompt_templates/*.yaml`) | Translate outright, few-shot examples included. Keep template variables (`{{.Query}}`, `%s`), JSON keys and output-format instructions byte-exact. |
| Non-English locale blocks in `config/*.yaml` (`zh-CN:`, `ja-JP:`, ...) | Delete; the `default`/English entry is the fallback. |
| Matchers for Chinese input or provider output (chunker separators, query expansion, OCR "no text" replies, gateway rate-limit text, stopwords) | Keep the value. Rewrite as `\uXXXX` escapes and add an English comment naming what it matches. |
| Test inputs that exercise CJK handling (splitters, truncation, tokenizers) | Keep the value as `\u` escapes. |
| Test assertions on a production string you translated | Change in the same batch. |
| Data files of Chinese text (`internal/textconv/data/*.txt`) | Keep; exempt the path with `-english-only` in `.gitattributes`. |
| Persisted values (`types.UntaggedTagName`, `fallback_response` defaults, built-in agent seeds) | This is greenfield: no database has applied the migrations. Translate the constant and the migrations that write or match it (`000008`, the four init schemas, `000006`) in place, in one batch. |
| FAQ CSV/Excel headers and yes/no values | Export English. Import keeps accepting the Chinese aliases (`问题`, `是`/`否`), written as `\u` escapes. |
| Frontend locale bundles other than `en-US.ts`, miniprogram `zh`, non-English READMEs | Delete in their own batch, after copying each file to `private/removed/<path>`; rewire `frontend/src/i18n/index.ts` and `embed.ts`. |

## Coupled strings

A string is coupled when other code depends on its exact text. Translate both ends in one batch or neither. Known pairs: `internal/handler/session/qa.go` attachment summary with the regexes in `frontend/src/utils/attachmentParsingDisplay.ts`; `types.UntaggedTagName` with its lookups and migration `000008`. Four frontend tests slice source files at a Chinese comment (`Input-field.focus.test.mjs`, `Input-field.reasoning.test.ts`, `modelEditorFlow.test.ts`, `views/platform/fileDrop.test.mjs`); when you translate that comment, update the anchor string in the test.

## Proof

For every Go file in the batch:

```bash
go run ./scripts/translation-check <file.go>...            # comments and string literals only
go run ./scripts/translation-check -escape <file.go>...    # files where Chinese was only escaped
```

It exits 1 if any other token changed, including import paths, struct tags (`json:"..."` is wire format), `//go:` and `//nolint` directives, swag `@Router` lines and cgo preambles; translate around them. Wrapping a long line is fine: it folds a literal split into `"a" + "b"` (every operand a string literal), ignores a trailing comma before `)`, `}` or `]`, and compares rune literals by value, so `'报'` matches the rune it escapes. Any other added token, such as `_ = ` before an unchecked call, still fails. It lists every changed literal and flags `test`, `compared`, `case`, `matcher:<func>` and `old-text-used-elsewhere:<text>`. Resolve each flag before you report: find the other end (`git grep -n '<text>'`) and change it in the batch, or escape instead of translating. Non-Go files have no such tool; diff them and confirm by reading that only text changed.

## Gates

1. `scripts/check-english-only.sh <batch files>` exits 0 (escapes pass; a data file needs its `.gitattributes` line).
2. `translation-check` prints OK for every Go file.
3. `scripts/validate.sh` passes for the areas touched.
4. No em dash in added lines (the PostToolUse hook reports it).

## Report

Lines: batch, files, gates with exit codes, flagged literals and how each was resolved, anything left Chinese and why. A translation that reveals a bug is a CODE-BUG row in `private/findings/`, not a fix inside the batch.
