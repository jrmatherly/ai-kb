"""MDX to Markdown preprocessing.

A line-based pass that drops MDX-only syntax (ESM imports and exports, JSX
expressions and tags, Docusaurus admonition fences) and keeps the text a
reader sees, so the result can go through the Markdown pipeline.
"""

import bisect
import re

from docreader.models.document import Document
from docreader.parser.base_parser import BaseParser
from docreader.parser.chain_parser import PipelineParser
from docreader.parser.markdown_parser import (
    MarkdownImageBase64,
    MarkdownTableFormatter,
)
from docreader.utils import endecode

_FENCE_OPEN = re.compile(r"^\s*(`{3,}|~{3,})")
_FENCE_CLOSE = re.compile(r"^\s*(`{3,}|~{3,})\s*$")
_ESM_START = re.compile(r"import\b|export ")
_ADMONITION_OPEN = re.compile(r"^\s*:::(\w+)(?:\[(.*)\]|\s+(.+))?\s*$")
_ADMONITION_CLOSE = re.compile(r"^\s*:::\s*$")
_FRONTMATTER_FIELD = re.compile(r"^(\w+):(.*)$")
_YAML_LINE = re.compile(r"""(\s|#|-\s|-$|[\w.-]+\s*:|"[^"]*"\s*:|'[^']*'\s*:)""")
_BACKTICKS = re.compile(r"`+")
_SPECIAL = re.compile(r"[`{<]")
_TAG_START = re.compile(r"<[A-Za-z/>]")
_TAG_HEAD = re.compile(r"<\s*(/)?\s*([A-Za-z][\w.:-]*)?")
_STRING_ATTR = re.compile(
    r"""(?<![\w:-])([A-Za-z_][\w:-]*)\s*=\s*(?:"([^"]*)"|'([^']*)')"""
)


def _find_end(text: str, start: int, kind: str) -> tuple[int, str] | None:
    """Scan from ``start`` to the end of a JSX tag, a ``{}`` expression or
    an ESM statement, skipping strings and comments.

    Returns the end offset (exclusive; for a statement, the offset of the
    newline that ends it) and, for a tag, its text with ``{}`` expressions
    removed, or None when the end never arrives.
    """
    depth, quote, flat = 0, None, []
    openers, closers = ("{[(", "}])") if kind == "statement" else ("{", "}")
    k = start
    while k < len(text):
        c = text[k]
        if quote:
            if c == "\\":
                k += 2
                continue
            if c == quote:
                quote = None
        elif (depth or kind == "statement") and text.startswith("/*", k):
            k = text.find("*/", k + 2)
            if k < 0:
                return None
            k += 2
            continue
        elif (depth or kind == "statement") and text.startswith("//", k):
            k = text.find("\n", k)
            if k < 0:
                return None
            continue
        elif c in "\"'`":
            quote = c
        elif c in openers:
            depth += 1
        elif c in closers:
            depth -= 1
            if kind == "brace" and depth == 0:
                return k + 1, ""
            k += 1
            continue
        elif kind == "tag" and c == ">" and depth == 0:
            return k + 1, "".join(flat) + c
        elif kind == "statement" and c == "\n" and depth <= 0:
            return k, ""
        if depth == 0 and kind == "tag":
            flat.append(c)
        k += 1
    if kind == "statement" and depth <= 0 and quote is None:
        return len(text), ""
    return None


def _render_tag(tag: str) -> list[str]:
    """Lines a tag contributes: a bold title and its description, an image,
    or nothing."""
    head = _TAG_HEAD.match(tag)
    if head is None or head.group(1) or head.group(2) is None:
        return []
    attrs = {
        m.group(1): m.group(2) or m.group(3) or "" for m in _STRING_ATTR.finditer(tag)
    }
    if head.group(2) in ("img", "Image"):
        src = attrs.get("src")
        return [f"![{attrs.get('alt', '')}]({src})"] if src else []
    title = next(
        (attrs[k] for k in ("title", "label", "name", "path", "value") if attrs.get(k)),
        None,
    )
    if not title:
        return []
    detail = attrs.get("description") or attrs.get("alt")
    return [f"**{title}**"] + ([detail] if detail else [])


def _line_end(text: str, pos: int) -> int:
    end = text.find("\n", pos)
    return len(text) if end < 0 else end


def _scan_line(text: str, pos: int) -> tuple[list[tuple[str, bool]], int, bool] | None:
    """Process prose from ``pos`` to the end of its line, letting tags span
    lines.

    Returns ``(pieces, end, changed)``: pieces are ``(text, generated)``
    pairs, ``end`` is the offset of the newline that ends the logical line,
    ``changed`` says whether any tag or comment was removed. None when a tag
    never closes.
    """
    pieces: list[tuple[str, bool]] = []
    buf: list[str] = []
    changed = False
    eol = _line_end(text, pos)
    while pos < len(text) and text[pos] != "\n":
        c = text[pos]
        if c == "\\" and text[pos + 1 : pos + 2] not in ("", "\n"):
            buf.append(text[pos : pos + 2])
            pos += 2
            continue
        if c == "`" and (backticks := _BACKTICKS.match(text, pos)):
            run = backticks.group(0)
            close = text.find(run, pos + len(run), eol)
            if close >= 0:
                buf.append(text[pos : close + len(run)])
                pos = close + len(run)
                continue
        elif text.startswith("{/*", pos):
            close = text.find("*/}", pos, eol)
            if close >= 0:
                pos = close + 3
                changed = True
                continue
        elif _TAG_START.match(text, pos):
            found = _find_end(text, pos, "tag")
            if found is None:
                return None
            pos, tag = found
            if pos > eol:
                eol = _line_end(text, pos)
            changed = True
            head = _TAG_HEAD.match(tag)
            if head and (head.group(2) or "").lower() == "br":
                buf.append(" ")
            generated = _render_tag(tag)
            if generated:
                pieces.append(("".join(buf), False))
                buf = []
                pieces.extend((line, True) for line in generated)
            continue
        buf.append(c)
        pos += 1
    pieces.append(("".join(buf), False))
    return pieces, pos, changed


def _frontmatter(lines: list[str]) -> tuple[int, str | None, str | None]:
    """Index of the first body line, and the frontmatter title and
    description."""
    if not lines or lines[0].rstrip() != "---":
        return 0, None, None
    for end in range(1, len(lines)):
        if lines[end].rstrip() == "---":
            break
    else:
        return 0, None, None
    if not all(_YAML_LINE.match(line) for line in lines[1:end] if line.strip()):
        return 0, None, None
    fields = {}
    for line in lines[1:end]:
        m = _FRONTMATTER_FIELD.match(line)
        if m is None:
            continue
        value = m.group(2).strip()
        if value and value[0] not in ">|":
            if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
                value = value[1:-1]
            fields[m.group(1)] = value
    return end + 1, fields.get("title"), fields.get("description")


def _apply_output_policy(out: list[tuple[str, bool]]) -> str:
    """Collapse blank runs to one and strip leading and trailing blanks;
    verbatim lines (fenced code) are kept as they are."""
    result: list[str] = []
    blank = False
    for line, verbatim in out:
        if not verbatim and not line.strip():
            blank = True
            continue
        if blank and result:
            result.append("")
        blank = False
        result.append(line)
    return "\n".join(result) + "\n" if result else ""


def mdx_to_markdown(text: str) -> str:
    """Turn MDX source into plain Markdown."""
    text = text.removeprefix("\ufeff").replace("\r\n", "\n").replace("\r", "\n")
    lines = text.split("\n")
    if lines[-1] == "":
        lines.pop()
    starts = [0]
    for line in lines[:-1]:
        starts.append(starts[-1] + len(line) + 1)

    def line_of(offset: int) -> int:
        return bisect.bisect_right(starts, offset) - 1

    body_start, title, description = _frontmatter(lines)
    out: list[tuple[str, bool]] = []
    has_h1 = False

    def emit_generated(line: str) -> None:
        out.extend([("", False), (line, False), ("", False)])

    fence: str | None = None
    i = body_start
    while i < len(lines):
        line = lines[i]
        if fence:
            out.append((line, True))
            m = _FENCE_CLOSE.match(line)
            if m and m.group(1)[0] == fence[0] and len(m.group(1)) >= len(fence):
                fence = None
            i += 1
            continue
        m = _FENCE_OPEN.match(line)
        if m:
            fence = m.group(1)
            out.append((line, True))
            i += 1
            continue

        kind = (
            "brace"
            if line.startswith("{")
            else "statement"
            if _ESM_START.match(line)
            else None
        )
        found = _find_end(text, starts[i], kind) if kind else None
        unterminated = kind is not None and found is None
        end_line = line_of(found[0] - 1) if found else i
        tail = text[found[0] : _line_end(text, found[0])].strip() if found else ""
        if found and (end_line > i or not tail):
            if tail:
                emit_generated(tail)
            i = end_line + 1
            continue
        elif m := _ADMONITION_OPEN.match(line):
            label = m.group(1).capitalize()
            heading = m.group(2) or (m.group(3) or "").strip()
            emit_generated(f"**{label}: {heading}**" if heading else f"**{label}**")
            i += 1
            continue
        elif _ADMONITION_CLOSE.match(line):
            i += 1
            continue
        elif not unterminated and not _SPECIAL.search(line):
            has_h1 = has_h1 or line.startswith("# ")
            out.append((line, False))
            i += 1
            continue
        elif not unterminated and (scanned := _scan_line(text, starts[i])):
            pieces, end, changed = scanned
            for piece, generated in pieces:
                if generated:
                    emit_generated(piece)
                elif not changed or piece.strip():
                    has_h1 = has_h1 or piece.startswith("# ")
                    out.append((piece, False))
            i = line_of(end) + 1 if end < len(text) else len(lines)
            continue
        out.extend((rest, False) for rest in lines[i:])
        break

    header: list[tuple[str, bool]] = []
    if title and not has_h1:
        header += [(f"# {title}", False), ("", False)]
    if description:
        header += [(description, False), ("", False)]
    return _apply_output_policy(header + out)


class MdxPreprocessor(BaseParser):
    """Converts MDX bytes into a Markdown Document."""

    def parse_into_text(self, content: bytes) -> Document:
        return Document(content=mdx_to_markdown(endecode.decode_bytes(content)))


class MdxParser(PipelineParser):
    _parser_cls = (MdxPreprocessor, MarkdownTableFormatter, MarkdownImageBase64)
