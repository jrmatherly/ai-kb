"""EPUB parser.

Parses EPUB files into markdown text and optional embedded images.
"""

import base64
import logging
import os
import posixpath
from urllib.parse import unquote
import uuid
from typing import Any, Dict

from bs4 import BeautifulSoup

from docreader.models.document import Document
from docreader.parser.base_parser import BaseParser
from docreader.parser.epub_container import Epub, EpubItem, normalize_epub_path

logger = logging.getLogger(__name__)


class EPUBParser(BaseParser):
    """Parser for EPUB e-book files."""

    def __init__(self, *args, extract_images: bool = True, **kwargs):
        super().__init__(*args, **kwargs)
        self.extract_images = extract_images

    def parse_into_text(self, content: bytes) -> Document:
        logger.info(
            "Parsing EPUB file: %s, size: %d bytes", self.file_name, len(content)
        )
        try:
            with Epub(content) as book:
                metadata = self._extract_metadata(book)
                markdown_content, images = self._extract_content(book)
            metadata["source_format"] = "epub"
            metadata["file_size"] = len(content)
            metadata["chapter_count"] = len(
                [part for part in markdown_content.split("\n## ") if part.strip()]
            )
            metadata["image_count"] = len(images)
            return Document(
                content=markdown_content, images=images, metadata=metadata
            )
        except Exception as e:
            logger.warning(
                "EPUB container unreadable: %s, trying ZIP fallback", str(e)
            )
            return self._parse_epub_fallback(content)

    def _parse_epub_fallback(self, content: bytes) -> Document:
        """Parse EPUB directly as a ZIP when the container or OPF is unreadable."""
        import re
        import zipfile
        from io import BytesIO

        metadata = {"source_format": "epub", "file_size": len(content)}
        images: Dict[str, str] = {}
        image_aliases: Dict[str, str] = {}
        markdown_parts = []

        with zipfile.ZipFile(BytesIO(content), "r") as epub_zip:
            html_files = [
                f
                for f in epub_zip.namelist()
                if f.endswith((".html", ".xhtml", ".htm"))
            ]

            def chapter_num(filename: str) -> int:
                match = re.search(r"chapter(\d+)", filename, re.IGNORECASE)
                return int(match.group(1)) if match else 999999

            html_files.sort(key=chapter_num)

            if self.extract_images:
                img_exts = (".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg")
                for img_file in epub_zip.namelist():
                    if img_file.lower().endswith(img_exts):
                        try:
                            self._store_image(
                                images, image_aliases, img_file, epub_zip.read(img_file)
                            )
                        except Exception as e:
                            logger.warning(
                                "Failed to extract image %s: %s", img_file, e
                            )

            for html_file in html_files:
                try:
                    html_content = epub_zip.read(html_file).decode(
                        "utf-8", errors="ignore"
                    )
                    chapter_md = self._html_to_markdown(
                        html_content,
                        image_aliases=image_aliases,
                        base_path=posixpath.dirname(html_file),
                    )
                    base = os.path.basename(html_file)
                    title = base.replace(".html", "").replace(".xhtml", "")
                    title = re.sub(
                        r"chapter[_-]?", "Chapter ", title, flags=re.IGNORECASE
                    )
                    title = title.replace("_", " ").replace("-", " ").title()
                    if chapter_md.strip():
                        markdown_parts.append(f"## {title}\n\n{chapter_md}")
                except Exception as e:
                    logger.warning("Failed to process %s: %s", html_file, e)

        metadata["chapter_count"] = len(markdown_parts)
        metadata["image_count"] = len(images)
        return Document(
            content="\n\n".join(markdown_parts), images=images, metadata=metadata
        )

    def _extract_metadata(self, book: Epub) -> Dict[str, Any]:
        metadata: Dict[str, Any] = {}
        mapping = {
            "title": "title",
            "creator": "author",
            "publisher": "publisher",
            "language": "language",
            "description": "description",
            "date": "date",
            "identifier": "isbn",
        }
        for dc_key, out_key in mapping.items():
            values = book.metadata.get(dc_key)
            if values:
                metadata[out_key] = ", ".join(values) if out_key == "author" else values[0]
        return metadata

    def _extract_content(self, book: Epub) -> tuple[str, Dict[str, str]]:
        markdown_parts = []
        images: Dict[str, str] = {}
        image_aliases: Dict[str, str] = {}

        if self.extract_images:
            for item in book.items:
                if not item.is_image:
                    continue
                try:
                    img_data = book.read(item)
                except KeyError:
                    logger.warning("EPUB manifest image %s is not in the archive", item.name)
                    continue
                self._store_image(images, image_aliases, item.name, img_data)

        for item in self._documents_in_reading_order(book):
            markdown_parts.append(
                self._process_chapter(
                    book,
                    item,
                    toc_index=len(markdown_parts),
                    image_aliases=image_aliases,
                )
            )

        return "\n\n".join(part for part in markdown_parts if part.strip()), images

    @staticmethod
    def _documents_in_reading_order(book: Epub) -> list[EpubItem]:
        """Return the book's documents in spine order.

        The spine is the reading order. The manifest can list files in any
        order: Standard Ebooks puts the title page after the last chapter
        there. Documents the spine leaves out follow, in manifest order.
        """
        documents = [item for item in book.items if item.is_document]
        ordered = []
        for item in book.spine:
            if item in documents and item not in ordered:
                ordered.append(item)
        ordered.extend(item for item in documents if item not in ordered)
        return ordered

    def _process_chapter(
        self,
        book: Epub,
        html_item: EpubItem,
        toc_index: int = 0,
        image_aliases: Dict[str, str] | None = None,
    ) -> str:
        try:
            html_content = book.read(html_item)
            soup = BeautifulSoup(html_content, "lxml")
            title_tag = soup.find(["h1", "h2"])
            if title_tag:
                chapter_title = title_tag.get_text().strip()
                title_tag.decompose()
            else:
                chapter_title = html_item.name.replace("/", "")
                chapter_title = chapter_title.replace(".xhtml", "")
                chapter_title = chapter_title.replace("-", " ").title()
            body_html = (
                str(soup.body)
                if soup.body
                else html_content.decode("utf-8", errors="ignore")
            )
            chapter_md = self._html_to_markdown(
                body_html,
                image_aliases=image_aliases,
                base_path=posixpath.dirname(html_item.name),
            )
            return f"## {chapter_title}\n\n{chapter_md}"
        except Exception as e:
            logger.error(
                "Failed to process chapter %s: %s", html_item.name, e
            )
            return f"## Chapter {toc_index + 1}\n\n[Error processing chapter: {e}]"

    def _html_to_markdown(
        self,
        html_content: str,
        image_aliases: Dict[str, str] | None = None,
        base_path: str = "",
    ) -> str:
        try:
            from bs4 import Comment
            from markdownify import markdownify as md

            soup = BeautifulSoup(html_content, "lxml")
            for element in soup(["script", "style"]):
                element.decompose()
            for comment in soup.find_all(
                string=lambda text: isinstance(text, Comment)
            ):
                comment.extract()
            self._strip_internal_links(soup)
            if image_aliases:
                self._rewrite_image_sources(soup, image_aliases, base_path)
            markdown_text = md(str(soup), heading_style="ATX")
            return "\n".join(
                line.strip() for line in markdown_text.split("\n") if line.strip()
            )
        except ImportError:
            logger.warning("markdownify not available, using HTML as-is")
            return f"```html\n{html_content}\n```"
        except Exception as e:
            logger.error("HTML to Markdown conversion failed: %s", e)
            return f"```html\n{html_content}\n```"

    @staticmethod
    def _strip_internal_links(soup: BeautifulSoup) -> None:
        """Unwrap links that don't point to an external resource.

        EPUB internal links (other chapter files, ``#fragment`` anchors, TOC
        entries) become dead links after extraction. Keep only external links
        and replace everything else with its text.
        """
        external = ("http://", "https://", "mailto:", "tel:")
        for link in soup.find_all("a"):
            href = str(link.get("href") or "").strip().lower()
            if not href or not href.startswith(external):
                link.unwrap()

    @staticmethod
    def _store_image(
        images: Dict[str, str],
        image_aliases: Dict[str, str],
        original_path: str,
        data: bytes,
    ) -> None:
        ext = os.path.splitext(original_path)[1]
        img_path = f"images/{uuid.uuid4().hex}{ext}"
        images[img_path] = base64.b64encode(data).decode("utf-8")
        EPUBParser._add_image_aliases(image_aliases, original_path, img_path)

    @staticmethod
    def _add_image_aliases(
        image_aliases: Dict[str, str],
        original_path: str,
        image_path: str,
    ) -> None:
        normalized = normalize_epub_path(original_path)
        aliases = {
            original_path,
            normalized,
        }
        for alias in aliases:
            if alias:
                image_aliases[alias] = image_path
        # A basename fallback must not overwrite a real archive-root path.
        image_aliases.setdefault(posixpath.basename(normalized), image_path)

    @staticmethod
    def _rewrite_image_sources(
        soup: BeautifulSoup,
        image_aliases: Dict[str, str],
        base_path: str = "",
    ) -> None:
        for img in soup.find_all("img"):
            src = str(img.get("src") or "").strip()
            if not src:
                continue
            # Only src is a URI reference. Strip its URI suffixes before decoding
            # once; archive paths from the manifest and ZIP already contain literal
            # characters, including #, ? and percent-encoded-looking names.
            raw_path = src.split("#", 1)[0].split("?", 1)[0].replace("\\", "/")
            src_path = unquote(raw_path).replace("\\", "/")
            normalized_src = normalize_epub_path(src_path)
            # Some EPUBs reference literal percent-looking names verbatim.
            # Try the undecoded path after the decoded path at each location.
            raw_normalized = normalize_epub_path(raw_path)
            candidates = [normalized_src, raw_normalized]
            if base_path:
                joined = normalize_epub_path(
                    posixpath.join(base_path, src_path)
                )
                raw_joined = normalize_epub_path(
                    posixpath.join(base_path, raw_path)
                )
                # Resolve relative to the chapter before trying ambiguous aliases.
                candidates[:0] = [joined, raw_joined]
            candidates.append(posixpath.basename(normalized_src))
            for candidate in candidates:
                if candidate in image_aliases:
                    img["src"] = image_aliases[candidate]
                    break

