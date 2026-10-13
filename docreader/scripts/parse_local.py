"""Local parsing debug script. Calls Parser on a local file directly, without the gRPC service.

Usage (run from the repository root):
    PYTHONPATH=. docreader/.venv/bin/python docreader/scripts/parse_local.py <file path> [--engine markitdown] [--out out.md]

Examples:
    PYTHONPATH=. docreader/.venv/bin/python docreader/scripts/parse_local.py docreader/testdata/test.md
    PYTHONPATH=. docreader/.venv/bin/python docreader/scripts/parse_local.py ~/Desktop/demo.pdf --out /tmp/demo.md
"""

import argparse
import logging
import os
import sys
import time

from docreader.parser import Parser


def main() -> int:
    parser = argparse.ArgumentParser(description="Parse a local file and print markdown")
    parser.add_argument("path", help="Path of the local file to parse")
    parser.add_argument(
        "--engine",
        default="",
        help="Parser engine name (builtin / markitdown); empty uses the built-in engine",
    )
    parser.add_argument(
        "--type",
        default="",
        help="File type (such as pdf/docx/md); empty infers it from the extension",
    )
    parser.add_argument(
        "--out",
        default="",
        help="Write the full markdown to this file and export images to images/ in the same directory",
    )
    parser.add_argument(
        "--scanned",
        action="store_true",
        help="Skip markitdown text extraction and render each PDF page as an image (for scans, avoids pdfminer hanging)",
    )
    parser.add_argument(
        "--log-level",
        default="INFO",
        help="Log level (DEBUG/INFO/WARNING/ERROR)",
    )
    args = parser.parse_args()

    logging.basicConfig(
        level=getattr(logging, args.log_level.upper(), logging.INFO),
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
        stream=sys.stderr,
    )

    if not os.path.isfile(args.path):
        print(f"File not found: {args.path}", file=sys.stderr)
        return 1

    file_name = os.path.basename(args.path)
    file_type = args.type or os.path.splitext(file_name)[1].lstrip(".")
    with open(args.path, "rb") as f:
        content = f.read()

    started = time.monotonic()
    if args.scanned:
        from docreader.parser.pdf_parser import PDFScannedParser

        doc = PDFScannedParser(file_name=file_name, file_type=file_type).parse_into_text(
            content
        )
    else:
        doc = Parser().parse_file(
            file_name=file_name,
            file_type=file_type,
            content=content,
            parser_engine=args.engine or None,
        )
    elapsed = time.monotonic() - started

    print("=" * 60, file=sys.stderr)
    print(f"file       : {file_name}", file=sys.stderr)
    print(f"type       : {file_type}", file=sys.stderr)
    print(f"engine     : {args.engine or 'builtin'}", file=sys.stderr)
    print(f"scanned    : {args.scanned}", file=sys.stderr)
    print(f"content_len: {len(doc.content)}", file=sys.stderr)
    print(f"images     : {len(doc.images)}", file=sys.stderr)
    print(f"metadata   : {doc.metadata}", file=sys.stderr)
    print(f"elapsed    : {elapsed:.2f}s", file=sys.stderr)
    print("=" * 60, file=sys.stderr)

    if args.out:
        import base64

        out_dir = os.path.dirname(os.path.abspath(args.out))
        os.makedirs(out_dir, exist_ok=True)
        with open(args.out, "w", encoding="utf-8") as f:
            f.write(doc.content)
        if doc.images:
            img_root = os.path.join(out_dir, "images")
            os.makedirs(img_root, exist_ok=True)
            for ref_path, b64data in doc.images.items():
                try:
                    raw = base64.b64decode(b64data)
                except Exception:
                    raw = b64data if isinstance(b64data, bytes) else b64data.encode()
                dest = os.path.join(out_dir, ref_path)
                os.makedirs(os.path.dirname(dest), exist_ok=True)
                with open(dest, "wb") as imgf:
                    imgf.write(raw)
        print(f"Wrote: {args.out} (images exported to {out_dir}/images/)", file=sys.stderr)
    else:
        print(doc.content)

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
