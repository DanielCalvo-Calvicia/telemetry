"""The canonical form of a number: the ECMAScript Number-to-string form that RFC 8785 requires."""

import math
from decimal import Decimal

from telemetry.internal.domain.errors import NumberOutOfRangeError
from telemetry.internal.domain.value_objects.error_codes_generated import (
    ERROR_CODE_EVENT_SCHEMA_INVALID,
    ERROR_CODE_LIMIT_EXCEEDED,
)

# The largest integer that prints exactly: 2^53 - 1. An integer beyond it, in either direction, is rejected with the
# code limit_exceeded. It is the one place to change if the maintainer decides to serialize such integers as strings.
MAX_EXACT_INTEGER = 2**53 - 1


def format_int(value: int) -> str:
    """Print an integer; reject one beyond +-MAX_EXACT_INTEGER with the code limit_exceeded."""
    if value > MAX_EXACT_INTEGER or value < -MAX_EXACT_INTEGER:
        raise NumberOutOfRangeError(f"the integer {value} is beyond +-{MAX_EXACT_INTEGER}", ERROR_CODE_LIMIT_EXCEEDED)
    return str(value)


def format_float(value: float) -> str:
    """Print a double in the ECMAScript form: shortest round-trip digits, exponent form below 1e-6 and from 1e21.

    Both zeros print as "0". NaN and the infinities are rejected with the code event_schema_invalid.
    """
    if math.isnan(value) or math.isinf(value):
        raise NumberOutOfRangeError("NaN and the infinities have no JSON form", ERROR_CODE_EVENT_SCHEMA_INVALID)
    if value == 0:
        return "0"
    sign = "-" if value < 0 else ""
    # repr gives the shortest digits that round-trip; Decimal splits them from the exponent.
    _, digit_tuple, exponent = Decimal(repr(abs(value))).as_tuple()
    digits = "".join(str(digit) for digit in digit_tuple)
    stripped = digits.rstrip("0")
    exponent += len(digits) - len(stripped)
    return sign + ecmascript_form(stripped, len(stripped) + exponent)


def ecmascript_form(digits: str, point_position: int) -> str:
    """Lay out the shortest digits of a positive number the way Number::toString of ECMAScript does.

    digits has no trailing zeros and the value is 0.DIGITS * 10^point_position (n in the specification).
    """
    digit_count = len(digits)
    if digit_count <= point_position <= 21:
        return digits + "0" * (point_position - digit_count)
    if 0 < point_position <= 21:
        return digits[:point_position] + "." + digits[point_position:]
    if -6 < point_position <= 0:
        return "0." + "0" * -point_position + digits
    exponent = point_position - 1
    exponent_sign = "+" if exponent >= 0 else "-"
    if digit_count == 1:
        return f"{digits}e{exponent_sign}{abs(exponent)}"
    return f"{digits[0]}.{digits[1:]}e{exponent_sign}{abs(exponent)}"
