"""The layout of a UUIDv7 (RFC 9562): a pure function of the time and ten random bytes."""

RANDOM_BYTE_COUNT = 10
MAX_MILLISECONDS = (1 << 48) - 1
_VERSION_7 = 0x70
_VARIANT = 0x80


def uuid_v7_text(milliseconds: int, random_bytes: bytes) -> str:
    """Return the lowercase hyphenated UUIDv7 of a unix time in milliseconds and ten random bytes.

    The layout is: 48 bits of the time, big-endian; the version 7 in the high nibble of byte 6, then the low nibble of
    ``random_bytes[0]`` and ``random_bytes[1]`` (12 random bits); the variant ``10`` in the two high bits of byte 8,
    then the low six bits of ``random_bytes[2]`` and ``random_bytes[3:]`` (62 random bits). It raises ValueError when
    the time is outside 0 to 2^48 - 1 or the bytes are not ten.
    """
    if isinstance(milliseconds, bool) or not isinstance(milliseconds, int) or not 0 <= milliseconds <= MAX_MILLISECONDS:
        raise ValueError("the time must be an integer of 48 bits")
    if not isinstance(random_bytes, (bytes, bytearray)) or len(random_bytes) != RANDOM_BYTE_COUNT:
        raise ValueError("ten random bytes are needed")
    raw = bytearray(milliseconds.to_bytes(6, "big"))
    raw.append(_VERSION_7 | (random_bytes[0] & 0x0F))
    raw.append(random_bytes[1])
    raw.append(_VARIANT | (random_bytes[2] & 0x3F))
    raw.extend(random_bytes[3:])
    hexadecimal = raw.hex()
    return f"{hexadecimal[:8]}-{hexadecimal[8:12]}-{hexadecimal[12:16]}-{hexadecimal[16:20]}-{hexadecimal[20:]}"
