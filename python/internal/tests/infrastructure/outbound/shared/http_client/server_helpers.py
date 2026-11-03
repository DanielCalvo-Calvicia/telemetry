"""Local test servers for the HTTP client tests: http.server on 127.0.0.1, a free port, in a thread."""

import http.server
import socket
import ssl
import subprocess
import tempfile
import threading
import time
from dataclasses import dataclass, field
from typing import Callable, Optional

SECRET = "s3cr3t-Zx9QkW7"


@dataclass
class Reply:
    """What the server answers: the status, extra headers, a body and a delay before answering."""

    status: int = 200
    headers: dict[str, str] = field(default_factory=dict)
    body: bytes = b""
    delay_seconds: float = 0.0
    raw: Optional[bytes] = None  # when set, these exact bytes are written and the connection is closed


@dataclass(frozen=True)
class Received:
    """One request as the server saw it."""

    method: str
    path: str
    headers: dict[str, str]
    body: bytes


def free_port() -> int:
    """Return a port of 127.0.0.1 that nothing listens on."""
    with socket.socket() as probe:
        probe.bind(("127.0.0.1", 0))
        return probe.getsockname()[1]


def make_certificate(folder: str) -> tuple[str, str]:
    """Generate a throwaway self-signed certificate for localhost and 127.0.0.1 with openssl; never committed."""
    certificate = f"{folder}/certificate.pem"
    key = f"{folder}/key.pem"
    subprocess.run(
        [
            "openssl",
            "req",
            "-x509",
            "-newkey",
            "rsa:2048",
            "-nodes",
            "-keyout",
            key,
            "-out",
            certificate,
            "-days",
            "1",
            "-subj",
            "/CN=localhost",
            "-addext",
            "subjectAltName=DNS:localhost,IP:127.0.0.1",
        ],
        check=True,
        capture_output=True,
    )
    return certificate, key


class StubServer:
    """A server that records every request and answers with what ``respond`` returns; a context manager."""

    def __init__(self, respond: Callable[[Received], Reply] = lambda received: Reply(), tls: bool = False) -> None:
        self.requests: list[Received] = []
        self._lock = threading.Lock()
        self._respond = respond
        self._tls = tls
        self._folder: Optional[tempfile.TemporaryDirectory] = None
        stub = self

        class Handler(http.server.BaseHTTPRequestHandler):
            protocol_version = "HTTP/1.1"

            def handle_any(self) -> None:
                length = int(self.headers.get("Content-Length", "0"))
                received = Received(
                    self.command,
                    self.path,
                    {name.lower(): value for name, value in self.headers.items()},
                    self.rfile.read(length),
                )
                with stub._lock:
                    stub.requests.append(received)
                reply = stub._respond(received)
                if reply.delay_seconds:
                    time.sleep(reply.delay_seconds)
                if reply.raw is not None:
                    self.wfile.write(reply.raw)
                    self.close_connection = True
                    return
                self.send_response(reply.status)
                for name, value in reply.headers.items():
                    self.send_header(name, value)
                self.send_header("Content-Length", str(len(reply.body)))
                self.send_header("Connection", "close")
                self.end_headers()
                self.wfile.write(reply.body)

            do_POST = do_PUT = do_PATCH = handle_any  # noqa: N815  the names http.server looks for

            def log_message(self, *arguments) -> None:
                pass

        class QuietServer(http.server.ThreadingHTTPServer):
            request_queue_size = (
                128  # the default backlog of 5 refuses some of 16 clients that connect at once, on a loaded machine
            )

            def handle_error(self, request, client_address) -> None:
                pass  # a client that gave up (a timeout test) is not an error of the test

        self._server = QuietServer(("127.0.0.1", 0), Handler)
        self._server.daemon_threads = True
        self._thread = threading.Thread(target=self._server.serve_forever, daemon=True)

    @property
    def port(self) -> int:
        """The port the server listens on."""
        return self._server.server_address[1]

    def url(self, path: str = "/ingest") -> str:
        """The URL of the server: http, or https when it has a certificate."""
        return f"{'https' if self._tls else 'http'}://127.0.0.1:{self.port}{path}"

    def __enter__(self) -> "StubServer":
        if self._tls:
            self._folder = tempfile.TemporaryDirectory()
            certificate, key = make_certificate(self._folder.name)
            context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
            context.load_cert_chain(certificate, key)
            self._server.socket = context.wrap_socket(self._server.socket, server_side=True)
        self._thread.start()
        return self

    def __exit__(self, *arguments) -> None:
        self._server.shutdown()
        self._server.server_close()
        if self._folder is not None:
            self._folder.cleanup()
