import os
import unittest

from docreader.parser.mdx_parser import MdxParser, mdx_to_markdown

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
        src = "\ufeff---\r\ntitle: T\r\n---\r\n<Note>\r\nHi\r\n</Note>\r\n"
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

    def test_export_trailing_comment_with_apostrophe_keeps_prose(self):
        src = "export const meta = { title: 'X' } // don't edit\n\nPara one.\n\nPara two isn't lost?\n\nPara three.\n"
        self.assertEqual(mdx_to_markdown(src), "Para one.\n\nPara two isn't lost?\n\nPara three.\n")

    def test_indented_fence_inside_component_is_untouched(self):
        src = "<Tabs>\n    ```jsx\n    <Tabs>\n    ```\n</Tabs>\n"
        self.assertEqual(mdx_to_markdown(src), "    ```jsx\n    <Tabs>\n    ```\n")

    def test_line_starting_with_expression_keeps_prose(self):
        src = "{props.version} is the current release.\n"
        self.assertEqual(mdx_to_markdown(src), src)

    def test_leading_thematic_break_is_not_frontmatter(self):
        out = mdx_to_markdown("---\n# Intro\nBody text\n---\nMore\n")
        self.assertIn("# Intro", out)
        self.assertIn("Body text", out)
        self.assertIn("More", out)

    def test_lone_carriage_returns(self):
        self.assertEqual(mdx_to_markdown("---\rtitle: T\r---\rHi\r"), "# T\n\nHi\n")

    def test_frontmatter_with_valid_yaml_shapes(self):
        cases = [
            "---\ntitle: T\ntags:\n- a\n- b\n---\nHi\n",
            "---\nsidebar-position: 1\ntitle: T\n---\nHi\n",
            "---\n\"x y\": 1\ntitle: T\n---\nHi\n",
        ]
        for src in cases:
            with self.subTest(src=src):
                self.assertEqual(mdx_to_markdown(src), "# T\n\nHi\n")


class MdxParserTest(unittest.TestCase):
    def test_pipeline_runs_preprocessor_then_tables(self):
        src = b"import X from 'y'\n\n|a|b|\n|-|-|\n|1|2|\n"
        doc = MdxParser(file_name="t.mdx").parse_into_text(src)
        self.assertNotIn("import", doc.content)
        self.assertIn("| a | b |", doc.content)

    def test_multiline_expression_keeps_text_after_its_closer(self):
        self.assertEqual(mdx_to_markdown("{\n  a\n} tail\nBody\n"), "tail\n\nBody\n")
