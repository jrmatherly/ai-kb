"""Generate the real DOCX regression fixture for GitHub issue #2634."""

from pathlib import Path

from docx import Document
from docx.enum.table import WD_CELL_VERTICAL_ALIGNMENT

OUTPUT = Path(__file__).with_name("issue_2634_vertical_merge.docx")


def main() -> None:
    # The Chinese text stays as escapes so a regenerated fixture matches the committed
    # DOCX that test_docx_merge.py reads: a genetic test catalog whose test-method
    # column is vertically merged, then a control table with equal adjacent cells.
    document = Document()
    document.add_heading("\u9057\u4f20\u75c5\u68c0\u6d4b\u9879\u76ee\u8868", level=1)
    document.add_paragraph(
        "\u56de\u5f52\u573a\u666f\uff1a\u68c0\u6d4b\u65b9\u6cd5\u7eb5\u5411\u5408\u5e76\uff0c\u4f46\u6bcf\u6761\u68c0\u6d4b\u9879\u76ee\u5728\u8f6c\u6362\u540e\u90fd\u5fc5\u987b\u4fdd\u7559\u8be5\u65b9\u6cd5\u3002"
    )

    rows = [
        ("Q0101", "\u9057\u4f20\u6027\u4e73\u817a\u764c", "BRCA1", "15\u4e2a\u5de5\u4f5c\u65e5"),
        ("Q0102", "\u9057\u4f20\u6027\u5375\u5de2\u764c", "BRCA2", "15\u4e2a\u5de5\u4f5c\u65e5"),
        ("Q0103", "\u6797\u5947\u7efc\u5408\u5f81", "MLH1", "15\u4e2a\u5de5\u4f5c\u65e5"),
        ("Q0104", "\u5bb6\u65cf\u6027\u817a\u7624\u6027\u606f\u8089\u75c5", "APC", "15\u4e2a\u5de5\u4f5c\u65e5"),
    ]
    method = (
        "\u68c0\u6d4b\u65b9\u6cd5\uff1a\u63d0\u53d6\u5916\u5468\u8840\u57fa\u56e0\u7ec4 DNA\uff0c\u91c7\u7528\u9ad8\u901a\u91cf\u6d4b\u5e8f\uff0c\u5e76\u5bf9\u5019\u9009\u4f4d\u70b9\u8fdb\u884c"
        "Sanger \u6d4b\u5e8f\u9a8c\u8bc1\uff1b\u68c0\u6d4b\u7ed3\u679c\u9700\u7ed3\u5408\u4e34\u5e8a\u8868\u73b0\u7efc\u5408\u5224\u65ad\u3002"
    )

    table = document.add_table(rows=len(rows) + 1, cols=5)
    table.style = "Table Grid"
    headers = ("\u9879\u76ee\u7f16\u53f7", "\u68c0\u6d4b\u9879\u76ee", "\u76f8\u5173\u57fa\u56e0", "\u5468\u671f", "\u68c0\u6d4b\u65b9\u6cd5")
    for column, value in enumerate(headers):
        table.cell(0, column).text = value

    for row_index, row in enumerate(rows, start=1):
        for column, value in enumerate(row):
            table.cell(row_index, column).text = value

    merged_method = table.cell(1, 4).merge(table.cell(len(rows), 4))
    merged_method.text = method
    merged_method.vertical_alignment = WD_CELL_VERTICAL_ALIGNMENT.CENTER

    document.add_paragraph("\u975e\u5408\u5e76\u5bf9\u7167\u8868\uff1a\u76f8\u90bb\u5355\u5143\u683c\u5185\u5bb9\u5141\u8bb8\u76f8\u540c\uff0c\u4f46\u4e0d\u5e94\u88ab\u5f53\u4f5c\u5408\u5e76\u3002")
    control = document.add_table(rows=2, cols=3)
    control.style = "Table Grid"
    for column, value in enumerate(("\u5217A", "\u5217B", "\u5217C")):
        control.cell(0, column).text = value
    for column, value in enumerate(("\u76f8\u540c\u503c", "\u76f8\u540c\u503c", "\u72ec\u7acb\u503c")):
        control.cell(1, column).text = value

    document.save(OUTPUT)
    print(f"generated={OUTPUT}")
    print(f"size={OUTPUT.stat().st_size}")


if __name__ == "__main__":
    main()
