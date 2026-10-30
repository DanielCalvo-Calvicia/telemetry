"""The delivery mode: the generated enum, and the parsing of a text into it."""

from telemetry.internal.domain.errors import DeliveryModeUnknownError
from telemetry.internal.domain.value_objects.enums_generated import DeliveryMode


def parse_delivery_mode(text: str) -> DeliveryMode:
    """Return the DeliveryMode whose value is exactly ``text``; raise DeliveryModeUnknownError otherwise.

    There is no case folding. The message never holds the text that was given.
    """
    for member in DeliveryMode:
        if member.value == text:
            return member
    raise DeliveryModeUnknownError("a delivery mode must be best_effort, retry or durable")
