import contextlib
import logging
import time
import uuid
from contextvars import ContextVar
from logging import LogRecord
from typing import Optional

logger = logging.getLogger(__name__)

request_id_var: ContextVar[Optional[str]] = ContextVar("request_id", default=None)
_request_start_time_ctx: ContextVar[Optional[float]] = ContextVar(
    "request_start_time", default=None
)


def set_request_id(request_id: str) -> None:
    """Set the request ID for the current context."""
    request_id_var.set(request_id)


def get_request_id() -> Optional[str]:
    """Get the request ID for the current context."""
    return request_id_var.get()


class MillisecondFormatter(logging.Formatter):
    """Log formatter that shows millisecond timestamps (3 digits) instead of microseconds (6)."""

    def formatTime(self, record, datefmt=None):
        """Override formatTime to truncate microseconds to milliseconds."""
        result = super().formatTime(record, datefmt)

        # With a .%f format, truncate microseconds (6 digits) to milliseconds (3)
        if datefmt and ".%f" in datefmt:
            parts = result.split(".")
            if len(parts) > 1 and len(parts[1]) >= 6:
                millis = parts[1][:3]
                result = f"{parts[0]}.{millis}"

        return result


def init_logging_request_id():
    """
    Initialize logging to include request ID in log messages.
    Add the custom filter to all existing handlers
    """
    logger.info("Initializing request ID logging")
    root_logger = logging.getLogger()

    for handler in root_logger.handlers:
        handler.addFilter(RequestIdFilter())

        formatter = logging.Formatter(
            fmt="%(asctime)s.%(msecs)03d [%(request_id)s] %(levelname)-5s %(name)-20s | %(message)s",
            datefmt="%Y-%m-%d %H:%M:%S",
        )
        handler.setFormatter(formatter)

    logger.info(
        f"Updated {len(root_logger.handlers)} handlers with request ID formatting"
    )

    # No handlers: add a stream handler
    if not root_logger.handlers:
        handler = logging.StreamHandler()
        formatter = logging.Formatter(
            fmt="%(asctime)s.%(msecs)03d [%(request_id)s] %(levelname)-5s %(name)-20s | %(message)s",
            datefmt="%Y-%m-%d %H:%M:%S",
        )
        handler.setFormatter(formatter)
        handler.addFilter(RequestIdFilter())
        root_logger.addHandler(handler)
        logger.info("Added new StreamHandler with request ID formatting")


class RequestIdFilter(logging.Filter):
    """Filter that adds request ID to log messages"""

    def filter(self, record: LogRecord) -> bool:
        request_id = request_id_var.get()
        if request_id is not None:
            # Use a short request ID so log columns line up
            if len(request_id) > 8:
                short_id = request_id[:8]
                if "-" in request_id:
                    # Keep the prefix of IDs shaped like test-req-1-XXX
                    parts = request_id.split("-")
                    if len(parts) >= 3:
                        short_id = f"{parts[0]}-{parts[1]}-{parts[2]}"
                record.request_id = short_id
            else:
                record.request_id = request_id

            start_time = _request_start_time_ctx.get()
            if start_time is not None:
                elapsed_ms = int((time.time() - start_time) * 1000)
                record.elapsed_ms = elapsed_ms
                if not hasattr(record, "message_with_elapsed"):
                    record.message_with_elapsed = True
                    record.msg = f"{record.msg} (elapsed: {elapsed_ms}ms)"
        else:
            record.request_id = "no-req-id"

        return True


@contextlib.contextmanager
def request_id_context(request_id: Optional[str] = None):
    """Context manager that sets a request ID for the current context

    Args:
        request_id: request ID to use; generated when None

    Example:
        with request_id_context("req-123"):
            # Every log in this block carries request ID req-123
            logging.info("Processing request")
    """
    # Generate or use provided request ID
    req_id = request_id or str(uuid.uuid4())

    # Set start time and request ID
    start_time = time.time()
    req_token = request_id_var.set(req_id)
    time_token = _request_start_time_ctx.set(start_time)

    logger.info(f"Starting new request with ID: {req_id}")

    try:
        yield request_id_var.get()
    finally:
        # Log completion and reset context vars
        elapsed_ms = int((time.time() - start_time) * 1000)
        logger.info(f"Request {req_id} completed in {elapsed_ms}ms")
        request_id_var.reset(req_token)
        _request_start_time_ctx.reset(time_token)
