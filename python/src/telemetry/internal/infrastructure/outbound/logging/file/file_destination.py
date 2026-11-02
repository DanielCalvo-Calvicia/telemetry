"""The file destination: events appended as JSON Lines to a local file, with rotation by size and a quota (section 7.2).

It satisfies both the log sink port and the event sink port. Each event is one canonical JSON line, written with ONE
write under a lock, so the lines of concurrent callers never interleave. The file is opened for each line and closed
at once: no handle is kept, so a rotation can rename on every platform and there is nothing to close. The files hold
only the (already masked) event line and are owner only. The destination never raises from ``send`` (ADR-005).

The files are the current one and, after a rotation, ``file_name.1`` (the newest) to ``file_name.(max_files - 1)``
(the oldest). When the total passes the quota the oldest rotated files are deleted first; the current file never is.
"""

import threading
from dataclasses import dataclass
from typing import Sequence

from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.domain.value_objects.envelope_generated import Envelope
from telemetry.internal.domain.value_objects.error_codes_generated import ERROR_CODE_CONFIG_FIELD_INVALID
from telemetry.internal.domain.value_objects.severity import parse_severity, severity_at_least
from telemetry.internal.infrastructure.config.errors import ConfigProblem, ConfigurationError
from telemetry.internal.infrastructure.outbound.logging.file.error.file_errors import (
    REASON_DESTINATION_FAILED,
    REASON_ROTATION_FAILED,
    REASON_SERIALIZATION_FAILED,
    REASON_WRITE_FAILED,
    FileRotationFailedError,
    FileSerializationFailedError,
    FileWriteFailedError,
)
from telemetry.internal.infrastructure.outbound.logging.file.external import file_system
from telemetry.internal.infrastructure.outbound.logging.file.helpers.encode_json_line import encode_json_line
from telemetry.internal.infrastructure.outbound.logging.file.helpers.plan_quota import plan_quota
from telemetry.internal.infrastructure.outbound.logging.file.helpers.plan_rotation import (
    OP_REMOVE,
    plan_rotation,
)
from telemetry.internal.infrastructure.outbound.logging.file.settings import Settings, validate_settings

_DIRECTORY_CANNOT_BE_WRITTEN = "the directory cannot be written"


@dataclass(frozen=True)
class Counters:
    """A copy of the counters of the destination, taken at one moment.

    ``files_rotated`` counts the rotations, ``files_deleted_for_quota`` the rotated files the quota removed, and the
    three error counters the failed writes, failed rotations and failed quota deletions.
    """

    files_rotated: int = 0
    files_deleted_for_quota: int = 0
    write_errors: int = 0
    rotation_errors: int = 0
    quota_errors: int = 0


class FileDestination:
    """FileDestination appends each event it accepts as one JSON line to its current file.

    An event below the minimum severity is not written and it counts as ACCEPTED: the destination took it and its own
    policy decided not to write it. A failure to write or to rotate fails that event only.
    """

    def __init__(self, settings: Settings) -> None:
        """Validate the settings, create the directory and the current file, and keep what the sends need.

        Invalid settings raise a ConfigurationError with every problem. A directory that cannot be written raises a
        ConfigurationError with one problem on ``directory`` whose text holds neither the path nor the system error.
        """
        problems = validate_settings(settings)
        if problems:
            raise ConfigurationError(problems)
        self._base_name = settings.file_name
        self._base_path = file_system.join_path(settings.directory, settings.file_name)
        self._directory = settings.directory
        self._minimum = parse_severity(settings.minimum_severity)
        self._max_file_bytes = settings.max_file_bytes
        self._max_files = settings.max_files
        self._quota_bytes = settings.quota_bytes
        try:
            file_system.ensure_directory(settings.directory)
            file_system.touch_append(self._base_path)
        except OSError:
            raise ConfigurationError(
                (ConfigProblem("directory", ERROR_CODE_CONFIG_FIELD_INVALID, _DIRECTORY_CANNOT_BE_WRITTEN),)
            ) from None
        self._lock = threading.Lock()
        self._counters_lock = threading.Lock()
        self._counts = {
            "files_rotated": 0,
            "files_deleted_for_quota": 0,
            "write_errors": 0,
            "rotation_errors": 0,
            "quota_errors": 0,
        }

    @property
    def counters(self) -> Counters:
        """A copy of the counters; it is safe to read while other threads send."""
        with self._counters_lock:
            return Counters(**self._counts)

    def send(self, events: Sequence[Envelope]) -> SendResult:
        """Append the events and say how many were accepted, dropped or failed; it never raises (ADR-005).

        Each event is counted once. A line that cannot be built fails with ``serialization_failed``; a failed rotation
        fails the event with ``rotation_failed`` and a failed write with ``write_failed``, and the next events are
        still tried. Whatever is not counted when something unexpected happens fails with ``destination_failed``. The
        reason is the first one that happened. The lock is held for the whole batch.
        """
        accepted = failed = total = 0
        reason = ""
        with self._lock:
            try:
                batch = list(events)
                total = len(batch)
                for event in batch:
                    if not severity_at_least(event.severity, self._minimum):
                        accepted += 1
                        continue
                    try:
                        self._append_event(event)
                    except FileSerializationFailedError:
                        failed += 1
                        reason = reason or REASON_SERIALIZATION_FAILED
                        continue
                    except FileRotationFailedError:
                        failed += 1
                        reason = reason or REASON_ROTATION_FAILED
                        continue
                    except FileWriteFailedError:
                        failed += 1
                        reason = reason or REASON_WRITE_FAILED
                        continue
                    accepted += 1
                    self._enforce_quota()
            except Exception:
                failed += total - accepted - failed
                reason = reason or REASON_DESTINATION_FAILED
        return SendResult(accepted=accepted, dropped=0, failed=failed, reason=reason)

    def _count(self, name: str) -> None:
        with self._counters_lock:
            self._counts[name] += 1

    def _append_event(self, event: Envelope) -> None:
        """Encode the event, rotate when the line would pass the size, and append it."""
        try:
            line = encode_json_line(event)
        except Exception as cause:
            raise FileSerializationFailedError("the event could not be turned into a line") from cause
        try:
            size = file_system.file_size(self._base_path) or 0
        except OSError as cause:
            self._count("write_errors")
            raise FileWriteFailedError("the file could not be inspected") from cause
        if size > 0 and size + len(line) > self._max_file_bytes:
            self._rotate()
        try:
            file_system.append_line(self._base_path, line)
        except OSError as cause:
            self._count("write_errors")
            raise FileWriteFailedError("the file could not be written") from cause

    def _rotate(self) -> None:
        """Run the steps of the rotation plan; a failure raises FileRotationFailedError and is counted."""
        try:
            names = file_system.list_names(self._directory)
            for step in plan_rotation(self._base_name, self._max_files, names):
                from_path = file_system.join_path(self._directory, step.from_name)
                if step.op == OP_REMOVE:
                    file_system.remove(from_path)
                else:
                    file_system.rename(from_path, file_system.join_path(self._directory, step.to_name))
        except OSError as cause:
            self._count("rotation_errors")
            raise FileRotationFailedError("the files could not be rotated") from cause
        self._count("files_rotated")

    def _enforce_quota(self) -> None:
        """Delete the oldest rotated files while the total is above the quota; a failure is counted, nothing more."""
        names = [self._base_name] + [f"{self._base_name}.{number}" for number in range(1, self._max_files)]
        sizes: dict[str, int] = {}
        try:
            for name in names:
                size = file_system.file_size(file_system.join_path(self._directory, name))
                if size is not None:
                    sizes[name] = size
        except OSError:
            self._count("quota_errors")
            return
        for name in plan_quota(self._base_name, self._max_files, sizes, self._quota_bytes):
            try:
                file_system.remove(file_system.join_path(self._directory, name))
            except OSError:
                self._count("quota_errors")
            else:
                self._count("files_deleted_for_quota")
