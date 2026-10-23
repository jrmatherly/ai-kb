# MDX Import Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `.mdx` files import as readable Markdown through upload, URL import and every data-source sync.

**Architecture:** A line-based preprocessor in docreader turns MDX into Markdown (drops ESM and expression blocks, unwraps JSX components keeping their text and title attributes, converts admonitions, keeps the frontmatter title), then the existing Markdown pipeline runs. The Go side and frontend learn the extension in their static lists; the type routes to the remote docreader by default.

**Tech Stack:** Python 3.14 (docreader, `uv`, `unittest`), Go 1.26 (testify), TypeScript (`node:test` via `tsx --test`).

**Spec:** `docs/superpowers/specs/completed/2026-10-22-sync-import-capabilities-and-mdx.md`, Part 2. Revised 2026-10-22 after a three-model plan review.

## Global Constraints

- English only in code, comments, tests and docs; no em dashes (`scripts/validate.sh` gates both).
- No new docreader dependency and no Node in the docreader image.
- Do not add `mdx` to `simpleFormats` in `internal/infrastructure/docparser/builtin_converter.go` or to the pass-through locator in `knowledge_source_locators.go:36`.
- Go formatting is gofumpt, lines at most 120; the pre-commit hook lints the whole root module, so every commit must build.
- Commit subjects follow `type(scope): imperative subject`.
- This plan lands before the capabilities plan; `repofile.IsSupported` still exists while it runs.

## Review Focus

1. JSX or `import` text inside a fenced code block must survive untouched (Task 1, `test_fenced_code_is_untouched`).
2. A `<` in prose (`x < 5`) and `{x}` in inline code must not be treated as a tag or expression (Task 1, `test_prose_angle_and_inline_braces_stay`).
3. An unclosed tag, and an `import` or `export` whose terminator never arrives, must fail open, leaving the text as it was (Task 1, `test_unclosed_tag_fails_open`, `test_unterminated_export_fails_open`).
4. CRLF line endings and a UTF-8 BOM must not break fence or frontmatter detection (Task 1, `test_crlf_and_bom`).
5. A file with no frontmatter and no JSX must come out identical apart from blank-line collapsing (Task 1, `test_plain_markdown_passes_through`).

---

### Task 1: MDX preprocessor

**Files:**

- Create: `docreader/parser/mdx_parser.py`
- Create: `docreader/tests/fixtures/mdx/docusaurus.mdx` (frontmatter `title: Quick start`; `import Tabs from '@theme/Tabs'`; a `:::note[Heads up]` block; `<Tabs><TabItem value="a" label="npm">Run npm</TabItem></Tabs>`; a fenced ```` ```jsx ```` block containing `<Tabs>`; a closing paragraph)
- Create: `docreader/tests/fixtures/mdx/nextra.mdx` (`import { Callout, Tabs } from 'nextra/components'`; multi-line `export const meta = {\n  title: 'X',\n}`; `<Callout type="info">Info text</Callout>`; `<Tabs items={['npm', 'pnpm']}><Tabs.Tab>npm text</Tabs.Tab><Tabs.Tab>pnpm text</Tabs.Tab></Tabs>`)
- Create: `docreader/tests/fixtures/mdx/mintlify.mdx` (`<Card title="Quickstart" icon="rocket" href="/q">Start here</Card>`; `<Steps><Step title="Install">Run install</Step><Step title="Run">Run it</Step></Steps>`; `<Note>A note</Note>`)
- Create: `docreader/tests/fixtures/mdx/fumadocs.mdx` (`<Callout title="Warning">Hot</Callout>`; `<Accordions><Accordion title="Why?">Because</Accordion></Accordions>`; `<TypeTable type={{ a: { type: 'string' } }} />`)
- Test: `docreader/tests/test_mdx_parser.py`

The component names in the fixtures come from each framework's documentation; the implementer confirms them against docusaurus.io, nextra.site, mintlify.com and fumadocs.dev before writing the fixtures and adjusts the fixture (not the rules) if a name differs.

**Interfaces:**

- Produces: `class MdxPreprocessor(BaseParser)` with `parse_into_text(self, content: bytes) -> Document` (decodes with `endecode.decode_bytes`, returns `Document(content=<markdown str>)`), and a pure function `mdx_to_markdown(text: str) -> str` that the class calls and the tests exercise directly.

- [ ] **Step 1: Write the failing tests**

```python
import os
import unittest

from docreader.parser.mdx_parser import mdx_to_markdown

FIXTURES = os.path.join(os.path.dirname(__file__), "fixtures", "mdx")


def fixture(name: str) -> str:
    with open(os.path.join(FIXTURES, name), encoding="utf-8") as f:
        return f.read()


class MdxToMarkdownTest(unittest.TestCase):
    def test_frontmatter_title_becomes_h1_when_body_has_none(self):
        out = mdx_to_markdown("---\ntitle: Quick start\ndescription: Install it\nsidebar: 3\n---\n\nText\n")
        self.assertEqual(out, "# Quick start\n\nInstall it\n\nText\n")

    def test_frontmatter_title_is_dropped_when_body_has_h1(self):
        out = mdx_to_markdown("---\ntitle: Quick start\n---\n# Quick start\n\nText\n")
        self.assertEqual(out, "# Quick start\n\nText\n")

    def test_imports_and_multiline_export_are_dropped(self):
        src = "import Tabs from '@theme/Tabs'\nimport {\n  Callout,\n} from 'nextra/components'\nexport const meta = {\n  title: 'X',\n}\n\nBody\n"
        self.assertEqual(mdx_to_markdown(src), "Body\n")

    def test_unterminated_export_fails_open(self):
        src = "export const meta = {\n  title: 'X',\n\nBody\n"
        self.assertEqual(mdx_to_markdown(src), src)

    def test_component_tags_are_unwrapped_keeping_inner_text(self):
        src = "<Tabs>\n<TabItem value=\"a\" label=\"npm\">\nRun npm\n</TabItem>\n</Tabs>\n"
        self.assertEqual(mdx_to_markdown(src), "**npm**\n\nRun npm\n")

    def test_self_closing_component_keeps_title_and_description(self):
        src = "<Card title=\"Quickstart\" description=\"Start here\" icon=\"rocket\" href=\"/q\" />\n"
        self.assertEqual(mdx_to_markdown(src), "**Quickstart**\n\nStart here\n")

    def test_multiline_attributes_and_expression_props(self):
        src = "<TypeTable\n  type={{ a: 1 }}\n  title=\"Props\"\n/>\nAfter\n"
        self.assertEqual(mdx_to_markdown(src), "**Props**\n\nAfter\n")

    def test_image_components_become_markdown_images(self):
        src = "<img src=\"/a.png\" alt=\"Diagram\" />\n<Image src=\"/b.png\" alt=\"Flow\" width={300} />\n"
        self.assertEqual(mdx_to_markdown(src), "![Diagram](/a.png)\n\n![Flow](/b.png)\n")

    def test_expression_blocks_and_comments_are_dropped(self):
        src = "{/* hidden */}\n{\n  props.note\n}\nText {/* inline */} here\n"
        self.assertEqual(mdx_to_markdown(src), "Text  here\n")

    def test_docusaurus_admonitions(self):
        src = ":::note[Heads up]\nCareful\n:::\n:::warning\nHot\n:::\n"
        self.assertEqual(mdx_to_markdown(src), "**Note: Heads up**\n\nCareful\n\n**Warning**\n\nHot\n")

    def test_fenced_code_is_untouched(self):
        src = "````md\n```jsx\n<Tabs>\nimport x from 'y'\n```\n````\n"
        self.assertEqual(mdx_to_markdown(src), src)

    def test_prose_angle_and_inline_braces_stay(self):
        src = "If x < 5 then `{x}` is small and a <= b.\n"
        self.assertEqual(mdx_to_markdown(src), src)

    def test_unclosed_tag_fails_open(self):
        src = "<Callout type=\"info\"\nStill text\n"
        self.assertEqual(mdx_to_markdown(src), src)

    def test_crlf_and_bom(self):
        src = "\\ufeff---\r\ntitle: T\r\n---\r\n<Note>\r\nHi\r\n</Note>\r\n"
        self.assertEqual(mdx_to_markdown(src), "# T\n\nHi\n")

    def test_plain_markdown_passes_through(self):
        src = "# H\n\nPara\n\n\n\n- a\n- b\n"
        self.assertEqual(mdx_to_markdown(src), "# H\n\nPara\n\n- a\n- b\n")

    def test_lowercase_html_block_tags_are_unwrapped(self):
        src = "<div className=\"x\">\n<p>Inner</p>\n</div>\n"
        self.assertEqual(mdx_to_markdown(src), "Inner\n")

    def test_docusaurus_fixture_has_no_jsx_left_outside_code(self):
        out = mdx_to_markdown(fixture("docusaurus.mdx"))
        body, _, code = out.partition("```jsx")
        for leftover in ("import ", "<Tab", ":::"):
            self.assertNotIn(leftover, body)
        for kept in ("# Quick start", "**Note: Heads up**", "**npm**", "Run npm"):
            self.assertIn(kept, body)
        self.assertIn("<Tabs>", code)

    def test_framework_fixtures_keep_text_and_titles(self):
        cases = {
            "nextra.mdx": (["Info text", "npm text", "pnpm text"], ["import ", "export ", "<Callout", "<Tabs", "items="]),
            "mintlify.mdx": (["**Quickstart**", "Start here", "**Install**", "Run install", "**Run**", "A note"], ["<Card", "<Step", "<Note"]),
            "fumadocs.mdx": (["**Warning**", "Hot", "**Why?**", "Because"], ["<Callout", "<Accordion", "<TypeTable", "type={{"]),
        }
        for name, (kept, gone) in cases.items():
            with self.subTest(name=name):
                out = mdx_to_markdown(fixture(name))
                for s in kept:
                    self.assertIn(s, out)
                for s in gone:
                    self.assertNotIn(s, out)
```

The `\ufeff` in `test_crlf_and_bom` is the escape sequence, written as `\ufeff` in the source so no editor can strip an invisible byte.

The expected strings fix the output policy, which Step 3 implements exactly:

- A deleted line (import, export, expression block, tag, `:::` closer, frontmatter) leaves no trace.
- Every line the pass produces (an H1 from the frontmatter, a description, a `**title**` line, a `![alt](src)` image) is emitted with one blank line before it and one after it.
- Any run of two or more blank lines collapses to one; leading and trailing blank lines are stripped; the output ends with exactly one newline.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `uv run --project docreader --locked python -m unittest docreader.tests.test_mdx_parser -v`
Expected: FAIL with `ModuleNotFoundError: No module named 'docreader.parser.mdx_parser'`

- [ ] **Step 3: Implement `mdx_to_markdown(text: str) -> str` and `MdxPreprocessor` in `docreader/parser/mdx_parser.py`**

Normalise once up front: strip a leading BOM, turn `\r\n` into `\n`. Then a state machine over lines:

- Frontmatter: only at file start, `---` to the next `---`; keep `title` as `# <title>` unless the body has a line starting with a single `#` and a space, keep `description` as a plain line, drop the rest.
- Fences: open on ``` ^\s{0,3}(`{3,}|~{3,}) ```, close on the same character with length at least the opener's; everything inside passes through.
- Outside fences, in this order: `import` (continuation until a line containing `from '` or `from "` or ending in `;`), `export` (depth of `{[(` counted, dropped until depth returns to zero on a line that ends the statement), a line starting with `{` (brace depth, dropped), `:::` open (`**Note: Title**` or `**Note**`, the word capitalised) and close (dropped), then tags.
- Tags: a `<` followed by `[A-Za-z/>]` starts one; scan forward across lines to the matching `>` while tracking `"`, `'` and `{}` depth. An opening tag or a self-closing tag that carries a `title`, `label` or `value` string attribute emits `**<value>**`, then its `description` or `alt` string attribute as a plain line when present; `img` and `Image` emit `![alt](src)` instead; every other opening tag, every closing tag and every other self-closing tag is removed. Inner text always stays.
- Fail open: if a tag's `>`, an import's terminator or an export's closing depth never arrives, emit the original lines unchanged from that point.
- Inline: remove `{/* ... */}` on prose lines; leave other inline braces and any `<` not followed by `[A-Za-z/>]`.
- Finally apply the output policy stated under Step 1.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `uv run --project docreader --locked python -m unittest docreader.tests.test_mdx_parser -v`
Expected: `OK` with 18 tests

- [ ] **Step 5: Commit**

```bash
git add docreader/parser/mdx_parser.py docreader/tests/test_mdx_parser.py docreader/tests/fixtures/mdx
git commit -m "feat(docreader): preprocess MDX into Markdown"
```

### Task 2: MDX parser and registry routing

**Files:**

- Modify: `docreader/parser/mdx_parser.py` (add the pipeline class)
- Modify: `docreader/parser/registry.py:11-19` (import) and `:165-176` (builtin dict)
- Test: `docreader/tests/test_parser_routing.py`, `docreader/tests/test_mdx_parser.py`

**Interfaces:**

- Consumes: `MdxPreprocessor` from Task 1; `MarkdownTableFormatter`, `MarkdownImageBase64` from `docreader/parser/markdown_parser.py`; `PipelineParser` from `docreader/parser/chain_parser.py:94`.
- Produces: `class MdxParser(PipelineParser)` with `_parser_cls = (MdxPreprocessor, MarkdownTableFormatter, MarkdownImageBase64)`; `registry.get_parser_class(BUILTIN_ENGINE, "mdx") is MdxParser`.

- [ ] **Step 1: Write the failing tests**

In `test_parser_routing.py`, add `from docreader.parser.mdx_parser import MdxParser` to the imports and, beside `test_builtin_registry_routes_xmind_to_xmind_parser`:

```python
def test_builtin_registry_routes_mdx_to_mdx_parser(self):
    self.assertIs(registry.get_parser_class(BUILTIN_ENGINE, "mdx"), MdxParser)
```

In `test_mdx_parser.py`, add `from docreader.parser.mdx_parser import MdxParser` and:

```python
class MdxParserTest(unittest.TestCase):
    def test_pipeline_runs_preprocessor_then_tables(self):
        src = b"import X from 'y'\n\n|a|b|\n|-|-|\n|1|2|\n"
        doc = MdxParser(file_name="t.mdx").parse_into_text(src)
        self.assertNotIn("import", doc.content)
        self.assertIn("| a | b |", doc.content)
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `uv run --project docreader --locked python -m unittest docreader.tests.test_parser_routing docreader.tests.test_mdx_parser -v`
Expected: FAIL with `ImportError: cannot import name 'MdxParser'`.

- [ ] **Step 3: Add `MdxParser` and register `"mdx": MdxParser` in the builtin engine dict after `"markdown": MarkdownParser`**

- [ ] **Step 4: Run the full docreader suite**

Run: `uv run --project docreader --locked python -m unittest discover -s docreader/tests -p "test_*.py"`
Expected: `OK`

- [ ] **Step 5: Commit**

```bash
git add docreader/parser/mdx_parser.py docreader/parser/registry.py docreader/tests/test_parser_routing.py docreader/tests/test_mdx_parser.py
git commit -m "feat(docreader): route .mdx through the MDX parser"
```

### Task 3: Go, frontend and docs learn the extension

**Files:**

- Modify: `internal/utils/import_filetypes.go:15` (add `"mdx": {}` after `"markdown": {}`)
- Modify: `internal/infrastructure/docparser/engines.go:78` (add `"mdx"` after `"markdown"`)
- Modify: `frontend/src/utils/fileTypeVerification.ts:1-27` (add `"mdx"` after `"md"`)
- Modify, existing tests that pin `.mdx` as unsupported, each flipped to supported: `internal/application/service/knowledge_util_filetype_test.go:66-70` (`want` gains `"mdx"` between `"md"` and `"mhtml"`), `internal/datasource/connector/repofile/repofile_test.go:36` (becomes `require.True(..., "docs/guide.mdx")`), `internal/datasource/connector/gitlab/client_test.go:311` (`want: true`), `internal/datasource/connector/seafile/sync_test.go:271-277` (`skip.mdx` becomes `skip.go` so the case still tests an unsupported file), `internal/datasource/connector/seafile/connector_test.go:40-43` (`page.mdx` becomes `page.go`)
- Modify, every other place that restates the import set or the builtin parser list: `website-docs/03-features/03-document-parsing.md:10` (Text row gains MDX) and `:122` (builtin row gains `mdx` (MdxParser)), plus the lists at `website-docs/02-architecture/03-document-pipeline.md:80-84` and `:280`, `website-docs/06-development/03-extension-points.md:87`, and `docreader/README.md`; confirm the set with `grep -rn '"xlsb", "xmind"\|mhtml, xmind\|xmind (XMindParser)' website-docs docreader/README.md` before editing
- Modify: `CHANGELOG.md` under `## [Unreleased]`, creating a `### Changes` heading if none exists
- Test: `internal/utils/import_filetypes_test.go`, `frontend/src/utils/fileTypeVerification.test.ts`

**Interfaces:**

- Produces: `utils.IsSupportedImportExtension("mdx") == true`, so `repofile.IsSupported("docs/a.mdx")` and every upload path accept it.

- [ ] **Step 1: Write the failing tests**

In `import_filetypes_test.go` `TestIsSupportedImportExtension`, add the case `{"mdx", true}` in the table's existing shape. In `fileTypeVerification.test.ts`, which uses `node:test` and `node:assert/strict`:

```ts
test('MDX files are accepted in the fallback whitelist', () => {
  assert.equal(shouldRejectKnowledgeFileType('guide.mdx'), false)
})
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/utils/ -run TestIsSupportedImportExtension` and `cd frontend && npx tsx --test src/utils/fileTypeVerification.test.ts`
Expected: both FAIL on the `mdx` case.

- [ ] **Step 3: Add `mdx` to the three lists, flip the five existing tests, update the docs rows and lists, and add the changelog entry**

Changelog line: `- **NEW**: **MDX import**: \`.mdx\` files (Markdown with JSX, as written for Docusaurus, Nextra, Mintlify and Fumadocs) import through upload, URL import and every data-source sync. docreader strips imports, exports and expressions, unwraps components keeping their text and titles, and converts admonitions before the Markdown pipeline runs.`

- [ ] **Step 4: Run the tests and the gates**

Run: `go test ./internal/utils/ ./internal/infrastructure/docparser/ ./internal/application/service/ ./internal/datasource/connector/...`, `cd frontend && npx tsx --test src/utils/fileTypeVerification.test.ts`, then `scripts/validate.sh`
Expected: all PASS; validate reports every area it lists as passed.

- [ ] **Step 5: Commit**

```bash
git add internal/utils/import_filetypes.go internal/utils/import_filetypes_test.go internal/infrastructure/docparser/engines.go internal/application/service/knowledge_util_filetype_test.go internal/datasource/connector/repofile/repofile_test.go internal/datasource/connector/gitlab/client_test.go internal/datasource/connector/seafile/sync_test.go internal/datasource/connector/seafile/connector_test.go frontend/src/utils/fileTypeVerification.ts frontend/src/utils/fileTypeVerification.test.ts website-docs docreader/README.md CHANGELOG.md
git commit -m "feat(import): accept .mdx files"
```

### Task 4: Live verification

**Files:** none changed.

- [ ] **Step 1: Bring up the dev stack** per the project memory (`SERVER_PORT=8081`, MinIO profile, an embedding model before a knowledge base): `make dev-start && make dev-app && make dev-frontend`.

- [ ] **Step 2: Upload `docreader/tests/fixtures/mdx/docusaurus.mdx`** through the knowledge base upload form and open its chunks.
Expected: no `import`, `<Tabs`, `</TabItem>` or `:::` text in any chunk; the tab label and admonition title appear as bold lines.

- [ ] **Step 3: Import the same file by URL** (serve the fixture from a local HTTP server on a whitelisted loopback host, as the connector memory notes describe) and open its chunks.
Expected: the same chunk content as the upload; this is the path whose type is auto-detected from the download.

- [ ] **Step 4: Sync a GitHub data source** whose selected directory holds a `.mdx` file (the GitHub PAT from the earlier live check).
Expected: the sync log counts it as created, not failed, and `GET /api/v1/datasource?kb_id=...` shows no `ingest_failed` sample for it.
