"""A fake receiver for the tests of delivery (T-04.8).

It plays the other side of the HTTP contract of T-04.4: it accepts batches ``{"schema_version", "events"}``, answers
with the status codes a test scripts, and lets the test assert what arrived. It is not a receiver design: it checks
the contract of the client and nothing more. It listens on 127.0.0.1 only, in a daemon thread, and lives with the tests.
"""

import copy
import http.server
import json
import threading
import time
import zlib
from dataclasses import dataclass, field
from typing import Any, Optional
from urllib.parse import urlsplit

MAX_BODY_BYTES = 8 * 1024 * 1024
"""The largest body the fake reads; a larger one is answered 413."""

SCHEMA_VERSION = "0.1"
HOST = "127.0.0.1"
_TRUNCATED_ANNOUNCED = 1000
_TRUNCATED_WRITTEN = 10
_DRAIN_CHUNK = 65536
_DRAIN_EXTRA_BYTES = 64 * 1024 * 1024
_TOO_LARGE = "the body is larger than 8 MiB"
_BAD_GZIP = "the body is not valid gzip"


@dataclass(frozen=True)
class Config:
    """What the test expects of the client: the authentication and the status used when the script is empty."""

    expected_bearer: str = ""
    expected_api_key_header: str = ""
    expected_api_key_value: str = ""
    default_status: int = 0  # 0 means 200


@dataclass(frozen=True)
class Answer:
    """One scripted answer. ``close`` answers nothing; ``truncate`` announces a body and never completes it."""

    status: int = 0  # 0 means 200
    retry_after: str = ""
    location: str = ""
    delay_seconds: float = 0.0
    close: bool = False
    truncate: bool = False


@dataclass(frozen=True)
class Request:
    """One request as the fake saw it, with the verdict it gave."""

    method: str
    path: str
    query: str
    headers: dict  # lower-cased names to values
    raw_body: bytes
    body: bytes  # decompressed when Content-Encoding is gzip
    gzipped: bool
    events: Optional[list]  # the events of a valid batch, else None
    valid: bool
    invalid_reason: str
    authenticated: bool
    answered_status: int  # 0 when the connection was closed without an answer
    received_at: float = field(default=0.0)


def validate_batch(body: bytes) -> tuple[Optional[list], str]:
    """Check the body is ``{"schema_version": "0.1", "events": [objects]}``; return the events or the reason."""
    try:
        document = json.loads(body.decode("utf-8"))
    except (ValueError, UnicodeDecodeError):
        return None, "the body is not valid JSON"
    if not isinstance(document, dict):
        return None, "the body is not a JSON object"
    if "schema_version" not in document:
        return None, "schema_version is missing"
    if "events" not in document:
        return None, "events is missing"
    for name in document:
        if name not in ("schema_version", "events"):
            return None, f"an unknown key: {name}"
    if not isinstance(document["schema_version"], str) or document["schema_version"] != SCHEMA_VERSION:
        return None, f"schema_version must be {SCHEMA_VERSION}"
    events = document["events"]
    if not isinstance(events, list):
        return None, "events must be a list"
    if not all(isinstance(item, dict) for item in events):
        return None, "every event must be an object"
    return events, ""


class _Server(http.server.ThreadingHTTPServer):
    daemon_threads = True
    request_queue_size = 128

    def __init__(self, receiver: "Receiver") -> None:
        self.receiver = receiver
        super().__init__((HOST, 0), _Handler)

    def handle_error(self, request: Any, client_address: Any) -> None:  # noqa: D401 - never print to stderr
        """A broken connection is a test situation, not a log line."""


class _Handler(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.0"  # one request per connection: close and truncate behave the same everywhere

    def log_message(self, format: str, *args: Any) -> None:  # noqa: A002
        """Never print a log line."""

    def handle_any(self) -> None:
        """Serve one request with the receiver that owns the server."""
        receiver: Receiver = self.server.receiver  # type: ignore[attr-defined]
        receiver._serve(self)

    do_GET = do_POST = do_PUT = do_PATCH = do_DELETE = do_HEAD = do_OPTIONS = handle_any  # noqa: N815


class Receiver:
    """A fake receiver on 127.0.0.1 and a free port. Thread safe; use it as a context manager or call ``close``."""

    def __init__(self, config: Config = Config()) -> None:
        self._config = config
        self._lock = threading.Lock()
        self._condition = threading.Condition(self._lock)
        self._script: list = []
        self._requests: list = []
        self._event_count = 0
        self._closed = threading.Event()
        self._server = _Server(self)
        self._port = self._server.server_address[1]
        self._thread = threading.Thread(target=self._server.serve_forever, name="fake-receiver", daemon=True)
        self._thread.start()

    def url(self) -> str:
        """The base URL of the fake, ``http://127.0.0.1:<port>``."""
        return f"http://{HOST}:{self._port}"

    def script(self, *answers: Answer) -> None:
        """Append answers; valid, authenticated requests consume them in order, then the default status applies."""
        with self._lock:
            self._script.extend(answers)

    def reset(self) -> None:
        """Clear the script and the recorded requests."""
        with self._condition:
            self._script.clear()
            self._requests.clear()
            self._event_count = 0
            self._condition.notify_all()

    def requests(self) -> list:
        """A deep copy of the recorded requests, in arrival order."""
        with self._lock:
            return copy.deepcopy(self._requests)

    def events(self) -> list:
        """The events of the valid batches, in arrival order."""
        with self._lock:
            return [event for item in self._requests if item.events for event in copy.deepcopy(item.events)]

    def wait_for_events(self, count: int, timeout_seconds: float) -> bool:
        """Block until at least ``count`` events arrived in valid batches; False when the timeout passes first."""
        with self._condition:
            return self._condition.wait_for(lambda: self._event_count >= count, timeout=timeout_seconds)

    def close(self) -> None:
        """Stop the server, release a request that waits on a delay and join the thread; safe to call twice."""
        with self._lock:
            if self._closed.is_set():
                return
            self._closed.set()
        self._server.shutdown()
        self._server.server_close()
        self._thread.join(timeout=10)

    def __enter__(self) -> "Receiver":
        return self

    def __exit__(self, *exc_info: Any) -> None:
        self.close()

    # ---- one request -------------------------------------------------------------------------------------------

    def _authenticated(self, headers: dict) -> bool:
        config = self._config
        if config.expected_bearer and headers.get("authorization", "") != f"Bearer {config.expected_bearer}":
            return False
        if config.expected_api_key_header:
            if headers.get(config.expected_api_key_header.lower()) != config.expected_api_key_value:
                return False
        return True

    def _read_body(self, handler: http.server.BaseHTTPRequestHandler) -> tuple[Optional[bytes], bool]:
        """The body and whether it is too large; the body is None when the client left before it ended."""
        try:
            length = int(handler.headers.get("Content-Length", "0"))
        except ValueError:
            length = 0
        length = max(length, 0)
        if length > MAX_BODY_BYTES:
            remaining = min(length, MAX_BODY_BYTES + _DRAIN_EXTRA_BYTES)
            while remaining > 0:  # drain, so that the client can read the answer
                chunk = handler.rfile.read(min(_DRAIN_CHUNK, remaining))
                if not chunk:
                    break
                remaining -= len(chunk)
            return b"", True
        body = handler.rfile.read(length) if length else b""
        if len(body) < length:
            return None, False
        return body, False

    @staticmethod
    def _gunzip(raw_body: bytes) -> tuple[bytes, str]:
        """The decompressed body and a reason: 'too large' when it passes the limit, 'bad gzip' when it is not gzip."""
        try:
            decoder = zlib.decompressobj(wbits=31)
            body = decoder.decompress(raw_body, MAX_BODY_BYTES + 1)
        except zlib.error:
            return b"", _BAD_GZIP
        if len(body) > MAX_BODY_BYTES:
            return b"", _TOO_LARGE
        if not decoder.eof:
            return b"", _BAD_GZIP
        return body, ""

    def _serve(self, handler: http.server.BaseHTTPRequestHandler) -> None:
        received_at = time.time()
        parts = urlsplit(handler.path)
        headers = {name.lower(): value for name, value in handler.headers.items()}
        authenticated = self._authenticated(headers)
        gzipped = headers.get("content-encoding", "").strip().lower() == "gzip"
        raw_body, too_large = self._read_body(handler)
        if raw_body is None:
            return  # the client disconnected in the middle of the request: nothing to record or to answer
        body = raw_body
        reason = _TOO_LARGE if too_large else ""
        if gzipped and not too_large:
            body, reason = self._gunzip(raw_body)
        events: Optional[list] = None
        status_override = 0
        if not authenticated:
            reason, status_override = "", 401
        elif reason == _TOO_LARGE:
            raw_body, body, status_override = b"", b"", 413
        elif reason:
            status_override = 400
        else:
            events, reason = validate_batch(body)
            if reason:
                status_override = 400
        self._finish(
            handler,
            parts,
            headers,
            raw_body,
            body,
            gzipped,
            events,
            reason,
            authenticated,
            status_override,
            received_at,
        )

    def _finish(  # noqa: PLR0913 - one record, one place
        self,
        handler: http.server.BaseHTTPRequestHandler,
        parts: Any,
        headers: dict,
        raw_body: bytes,
        body: bytes,
        gzipped: bool,
        events: Optional[list],
        reason: str,
        authenticated: bool,
        forced_status: int,
        received_at: float,
    ) -> None:
        """Choose the answer, record the request in one step (so the order is the order of arrival) and answer."""
        valid = events is not None
        with self._condition:
            answer = Answer()
            if valid:
                answer = self._script.pop(0) if self._script else Answer()
            if forced_status:
                answered = forced_status
            elif answer.close:
                answered = 0
            else:
                answered = answer.status or self._config.default_status or 200
            self._requests.append(
                Request(
                    method=handler.command,
                    path=parts.path,
                    query=parts.query,
                    headers=headers,
                    raw_body=raw_body,
                    body=body,
                    gzipped=gzipped,
                    events=events,
                    valid=valid,
                    invalid_reason=reason,
                    authenticated=authenticated,
                    answered_status=answered,
                    received_at=received_at,
                )
            )
            if valid:
                self._event_count += len(events or [])
            self._condition.notify_all()
        if forced_status:
            self._write(handler, forced_status, Answer(), reason.encode("utf-8"), text=True)
            return
        if answer.delay_seconds > 0 and self._closed.wait(answer.delay_seconds):
            handler.close_connection = True
            return
        if answer.close:
            handler.close_connection = True
            return
        self._write(handler, answered, answer, b"", text=False)

    @staticmethod
    def _write(
        handler: http.server.BaseHTTPRequestHandler, status: int, answer: Answer, body: bytes, text: bool
    ) -> None:
        """Write the status line, the headers and the body; a truncated answer promises more than it writes."""
        try:
            handler.send_response(status)
            if text:
                handler.send_header("Content-Type", "text/plain; charset=utf-8")
            if answer.retry_after:
                handler.send_header("Retry-After", answer.retry_after)
            if answer.location:
                handler.send_header("Location", answer.location)
            handler.send_header("Connection", "close")
            if answer.truncate:
                handler.send_header("Content-Length", str(_TRUNCATED_ANNOUNCED))
                handler.end_headers()
                handler.wfile.write(b"x" * _TRUNCATED_WRITTEN)
            else:
                handler.send_header("Content-Length", str(len(body)))
                handler.end_headers()
                handler.wfile.write(body)
            handler.wfile.flush()
        except OSError:
            pass  # the client left: nothing to do
        handler.close_connection = True
