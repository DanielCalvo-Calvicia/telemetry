"""The gzip of a body with a deterministic header."""

import gzip


def compress_gzip(body: bytes) -> bytes:
    """Return the body gzipped with no name and a zero modification time, so the same body gives the same bytes."""
    return gzip.compress(body, mtime=0)
