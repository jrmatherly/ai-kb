import re
from typing import Callable, Dict, List, Match, Pattern, Union

from pydantic import BaseModel, Field


class HeaderTrackerHook(BaseModel):
    """Configuration for the table header tracking hook."""

    start_pattern: Pattern[str] = Field(
        description="Header start match (regex or string)"
    )
    end_pattern: Pattern[str] = Field(description="Header end match (regex or string)")
    extract_header_fn: Callable[[Match[str]], str] = Field(
        default=lambda m: m.group(0),
        description="Function that extracts the header from the start match (default: the whole match)",
    )
    priority: int = Field(default=0, description="Priority (with several configs, higher priority matches first)")
    case_sensitive: bool = Field(
        default=True, description="Case sensitive (applies only to string patterns)"
    )

    def __init__(
        self,
        start_pattern: Union[str, Pattern[str]],
        end_pattern: Union[str, Pattern[str]],
        **kwargs,
    ):
        flags = 0 if kwargs.get("case_sensitive", True) else re.IGNORECASE
        if isinstance(start_pattern, str):
            start_pattern = re.compile(start_pattern, flags | re.DOTALL)
        if isinstance(end_pattern, str):
            end_pattern = re.compile(end_pattern, flags | re.DOTALL)
        super().__init__(
            start_pattern=start_pattern,
            end_pattern=end_pattern,
            **kwargs,
        )


# Default header hook configs: Markdown tables (code blocks disabled)
DEFAULT_CONFIGS = [
    # Code block (opens and closes with ```)
    # HeaderTrackerHook(
    #     # Code block start (with optional language)
    #     start_pattern=r"^\s*```(\w+).*(?!```$)",
    #     # Code block end
    #     end_pattern=r"^\s*```.*$",
    #     extract_header_fn=lambda m: f"```{m.group(1)}" if m.group(1) else "```",
    #     priority=20,  # code blocks take priority over tables
    #     case_sensitive=True,
    # ),
    # Markdown table (header row plus separator row)
    HeaderTrackerHook(
        # Header row + separator row
        start_pattern=r"^\s*(?:\|[^|\n]*)+[\r\n]+\s*(?:\|\s*:?-{3,}:?\s*)+\|?[\r\n]+$",
        # Blank line or non-table content
        end_pattern=r"^\s*$|^\s*[^|\s].*$",
        priority=15,
        case_sensitive=False,
    ),
]
DEFAULT_CONFIGS.sort(key=lambda x: -x.priority)

_TABLE_ROW_PATTERN = re.compile(r"^\s*(?:\|[^|\n]*)+\|\s*$", re.MULTILINE)
_MARKDOWN_TABLE_PRIORITY = 15


def _is_empty_table_header_row(header: str) -> bool:
    """True when the column-name line is only pipes/whitespace (MarkItDown quirk)."""
    newline = header.find("\n")
    if newline < 0:
        return False
    row = header[:newline].strip()
    return bool(row) and all(ch in "| \t" for ch in row)


def _extract_separator_line(header: str) -> str:
    for line in header.split("\n"):
        if "---" in line:
            return line + "\n"
    return ""


def _table_row_column_count(line: str) -> int:
    line = line.strip()
    if not line.startswith("|"):
        return 0
    parts = line.split("|")
    if parts and parts[0].strip() == "":
        parts = parts[1:]
    if parts and parts[-1].strip() == "":
        parts = parts[:-1]
    return len(parts)


def _first_table_row_column_count(text: str) -> int:
    for line in text.split("\n"):
        line = line.strip()
        if line and _TABLE_ROW_PATTERN.match(line):
            return _table_row_column_count(line)
    return 0


def _header_table_column_count(header: str) -> int:
    for line in header.split("\n"):
        line = line.strip()
        if not line or "---" in line:
            continue
        count = _table_row_column_count(line)
        if count > 0:
            return count
    return 0


def _split_ends_with_paragraph_break(split: str) -> bool:
    trimmed = split.rstrip(" \t\r")
    return trimmed.endswith("\n\n") or trimmed.endswith("\r\n\r\n")


def header_column_mismatch(headers: str, next_unit: str) -> bool:
    header_cols = _header_table_column_count(headers)
    row_cols = _first_table_row_column_count(next_unit)
    return header_cols > 0 and row_cols > 0 and header_cols != row_cols


class HeaderTracker(BaseModel):
    """State of the table header tracking hook."""

    header_hook_configs: List[HeaderTrackerHook] = Field(default=DEFAULT_CONFIGS)
    active_headers: Dict[int, str] = Field(default_factory=dict)
    ended_headers: set[int] = Field(default_factory=set)
    pending_extend: Dict[int, bool] = Field(default_factory=dict)
    pending_table_break: bool = Field(default=False)
    header_ended_this_unit: bool = Field(default=False)

    def _clear_table_header(self) -> None:
        self.ended_headers.add(_MARKDOWN_TABLE_PRIORITY)
        self.active_headers.pop(_MARKDOWN_TABLE_PRIORITY, None)
        self.pending_extend.pop(_MARKDOWN_TABLE_PRIORITY, None)

    def update(self, split: str) -> Dict[int, str]:
        """Detect header starts and ends in this split and update the hook state."""
        new_headers: Dict[int, str] = {}
        self.header_ended_this_unit = False

        if self.pending_table_break:
            self.pending_table_break = False
            if _MARKDOWN_TABLE_PRIORITY in self.active_headers:
                if _first_table_row_column_count(split) > 0:
                    self._clear_table_header()
                    self.header_ended_this_unit = True
                else:
                    self._clear_table_header()

        # 1. Check for header end markers
        for config in self.header_hook_configs:
            if config.priority in self.active_headers and config.end_pattern.search(
                split
            ):
                self.ended_headers.add(config.priority)
                del self.active_headers[config.priority]
                self.pending_extend.pop(config.priority, None)

        # 1b. \n\n splitting swallows the blank line between tables, so end header
        # tracking at a trailing \n\n or a column count change
        if (
            _MARKDOWN_TABLE_PRIORITY in self.active_headers
            and not self.pending_extend.get(_MARKDOWN_TABLE_PRIORITY)
        ):
            if _split_ends_with_paragraph_break(split):
                self.pending_table_break = True
            else:
                header = self.active_headers[_MARKDOWN_TABLE_PRIORITY]
                row_cols = _first_table_row_column_count(split)
                header_cols = _header_table_column_count(header)
                if row_cols > 0 and header_cols > 0 and row_cols != header_cols:
                    self._clear_table_header()
                    self.header_ended_this_unit = True

        # 2. Empty header row: fill column names from the first data row (matches Go header_tracker)
        for priority in list(self.pending_extend.keys()):
            if priority in self.active_headers and _TABLE_ROW_PATTERN.search(split):
                sep = _extract_separator_line(self.active_headers[priority])
                self.active_headers[priority] = split + sep
            self.pending_extend.pop(priority, None)

        # 3. Check for new header starts (only configs neither active nor ended)
        for config in self.header_hook_configs:
            if (
                config.priority not in self.active_headers
                and config.priority not in self.ended_headers
            ):
                match = config.start_pattern.search(split)
                if match:
                    header = config.extract_header_fn(match)
                    self.active_headers[config.priority] = header
                    new_headers[config.priority] = header
                    if _is_empty_table_header_row(header):
                        self.pending_extend[config.priority] = True

        # 4. When no header is active, clear the ended markers
        if not self.active_headers:
            self.ended_headers.clear()

        return new_headers

    def get_headers(self) -> str:
        """Join all active headers, highest priority first."""
        sorted_headers = sorted(self.active_headers.items(), key=lambda x: -x[0])
        return (
            "\n".join([header for _, header in sorted_headers])
            if sorted_headers
            else ""
        )
