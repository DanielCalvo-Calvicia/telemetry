"""The plan of the quota: which rotated files to delete, oldest first, so the total fits the quota.

A pure function shared with Go (integration-tests/fixtures/file/quota_plan.json). The current file is never deleted.
"""

from typing import Mapping


def plan_quota(base: str, max_files: int, sizes: Mapping[str, int], quota_bytes: int) -> list[str]:
    """Return the names to delete, in order, while the sum of ``sizes`` is above ``quota_bytes``.

    It walks ``base.(max_files - 1)`` down to ``base.1`` and takes each one that exists; it stops when the total fits
    or no rotated file is left.
    """
    total = sum(sizes.values())
    names: list[str] = []
    for number in range(max_files - 1, 0, -1):
        if total <= quota_bytes:
            break
        name = f"{base}.{number}"
        if name in sizes:
            names.append(name)
            total -= sizes[name]
    return names
