"""The plan of a rotation: which files to remove and rename, in order, as a pure function of the names that exist.

The files are the current one (``base``) and ``base.1`` (the newest rotated) to ``base.(max_files - 1)`` (the oldest).
A rotation makes room: the oldest goes, each rotated file takes the next number and the current file becomes
``base.1``. Any other name in the directory is ignored. The plan is a CONTRACT shared with Go
(integration-tests/fixtures/file/rotation_plan.json).
"""

from dataclasses import dataclass
from typing import Sequence

OP_REMOVE = "remove"
OP_RENAME = "rename"


@dataclass(frozen=True)
class Step:
    """One step of a rotation: ``op`` is ``remove`` (of ``from_name``) or ``rename`` (``from_name`` to ``to_name``).

    The shared fixtures call the fields ``from`` and ``to``; ``from`` is a keyword of Python.
    """

    op: str
    from_name: str
    to_name: str = ""


def plan_rotation(base: str, max_files: int, existing: Sequence[str]) -> list[Step]:
    """Return the steps that rotate the files of ``base`` when ``max_files`` are kept; ``existing`` are the names."""
    names = set(existing)
    if max_files == 1:
        return [Step(OP_REMOVE, base)] if base in names else []
    steps: list[Step] = []
    oldest = f"{base}.{max_files - 1}"
    if oldest in names:
        steps.append(Step(OP_REMOVE, oldest))
    for number in range(max_files - 2, 0, -1):
        name = f"{base}.{number}"
        if name in names:
            steps.append(Step(OP_RENAME, name, f"{base}.{number + 1}"))
    if base in names:
        steps.append(Step(OP_RENAME, base, f"{base}.1"))
    return steps
