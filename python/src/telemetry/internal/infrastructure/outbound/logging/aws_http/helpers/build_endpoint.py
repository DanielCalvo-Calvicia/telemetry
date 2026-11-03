"""The URL of the add_log endpoint (pure)."""


def build_endpoint(host: str, port: str, path: str) -> str:
    """Return ``host + ":" + port + path``; the host already holds its scheme (ADR-008)."""
    return f"{host}:{port}{path}"
