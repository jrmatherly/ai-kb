"""Read the parts of an EPUB container the parser needs: Dublin Core metadata,
the manifest and the spine. Pure standard library (zipfile plus ElementTree).
"""

from dataclasses import dataclass
from io import BytesIO
import posixpath
from urllib.parse import unquote
import xml.etree.ElementTree as ET
import zipfile

CONTAINER_NS = "{urn:oasis:names:tc:opendocument:xmlns:container}"
OPF_NS = "{http://www.idpf.org/2007/opf}"
DC_NS = "{http://purl.org/dc/elements/1.1/}"
DOCUMENT_MEDIA_TYPES = ("application/xhtml+xml", "text/html")


def normalize_epub_path(path: str) -> str:
    """Normalize a decoded archive path without interpreting it as a URI."""
    normalized = posixpath.normpath(path.replace("\\", "/"))
    return "" if normalized == "." else normalized.lstrip("/")


# Items compare by identity: the spine lookups below test membership, and two
# manifest entries are never the same item just because their fields match.
@dataclass(frozen=True, eq=False)
class EpubItem:
    id: str
    # The manifest href, URI-decoded, relative to the OPF directory. Chapter
    # image references resolve against this name, so it keeps literal
    # characters such as "#" exactly as the archive spells them.
    name: str
    media_type: str
    properties: frozenset[str]

    @property
    def is_document(self) -> bool:
        return self.media_type in DOCUMENT_MEDIA_TYPES and "nav" not in self.properties

    @property
    def is_image(self) -> bool:
        return self.media_type.startswith("image/")


class Epub:
    def __init__(self, content: bytes):
        self._zip = zipfile.ZipFile(BytesIO(content))
        container = ET.fromstring(self._zip.read("META-INF/container.xml"))
        rootfile = container.find(f".//{CONTAINER_NS}rootfile")
        if rootfile is None or not rootfile.get("full-path"):
            raise ValueError("container.xml names no OPF rootfile")
        opf_path = rootfile.get("full-path")
        self._opf_dir = posixpath.dirname(opf_path)
        package = ET.fromstring(self._zip.read(opf_path))

        self.metadata: dict[str, list[str]] = {}
        for element in package.iter():
            if element.tag.startswith(DC_NS) and element.text and element.text.strip():
                self.metadata.setdefault(element.tag[len(DC_NS):], []).append(
                    element.text.strip()
                )
        self.items: list[EpubItem] = [
            EpubItem(
                id=item.get("id", ""),
                name=unquote(item.get("href", "")),
                media_type=item.get("media-type", ""),
                properties=frozenset(item.get("properties", "").split()),
            )
            for item in package.iter(f"{OPF_NS}item")
        ]
        by_id = {item.id: item for item in self.items}
        # The reading order; idrefs the manifest does not define are dropped.
        self.spine: list[EpubItem] = [
            by_id[ref.get("idref", "")]
            for ref in package.iter(f"{OPF_NS}itemref")
            if ref.get("idref", "") in by_id
        ]

    def read(self, item: EpubItem) -> bytes:
        return self._zip.read(
            normalize_epub_path(posixpath.join(self._opf_dir, item.name))
        )

    def __enter__(self) -> "Epub":
        return self

    def __exit__(self, *exc) -> None:
        self._zip.close()
