"""The spool settings of the configuration."""

from dataclasses import dataclass

from telemetry.internal.domain.rules import DEFAULT_SPOOL_QUOTA_BYTES


@dataclass(frozen=True, slots=True)
class SpoolSettings:
    """The local spool of the durable mode: its directory (empty when not set) and its disk quota in bytes."""

    directory: str = ""
    quota_bytes: int = DEFAULT_SPOOL_QUOTA_BYTES
