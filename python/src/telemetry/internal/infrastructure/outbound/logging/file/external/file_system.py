"""The file system: the only module of the file destination that touches ``os``.

No handle is kept: a file is opened for each line and closed at once, so a rotation can rename on every platform and
there is nothing to close.
"""

import os
from typing import Optional

_APPEND_FLAGS = os.O_APPEND | os.O_CREAT | os.O_WRONLY | getattr(os, "O_BINARY", 0)


def join_path(directory: str, name: str) -> str:
    """Join a directory and a file name."""
    return os.path.join(directory, name)


def ensure_directory(path: str) -> None:
    """Create the directory, and its parents, owner only (0700) when it does not exist."""
    os.makedirs(path, mode=0o700, exist_ok=True)


def file_size(path: str) -> Optional[int]:
    """Return the size in bytes of the file, or None when it does not exist."""
    try:
        return os.stat(path).st_size
    except FileNotFoundError:
        return None


def list_names(directory: str) -> list[str]:
    """Return the names of the entries of the directory."""
    return os.listdir(directory)


def append_line(path: str, line: bytes) -> None:
    """Append the bytes with ONE write (a loop only for a short write); the file is created owner only (0600)."""
    descriptor = os.open(path, _APPEND_FLAGS, 0o600)
    try:
        pending = memoryview(line)
        while len(pending) > 0:
            written = os.write(descriptor, pending)
            pending = pending[written:]
    finally:
        os.close(descriptor)


def touch_append(path: str) -> None:
    """Open the file for append, creating it owner only (0600) when needed, and close it without writing."""
    os.close(os.open(path, _APPEND_FLAGS, 0o600))


def rename(from_path: str, to_path: str) -> None:
    """Rename the file, replacing the target when it is a file."""
    os.replace(from_path, to_path)


def remove(path: str) -> None:
    """Remove the file."""
    os.remove(path)
