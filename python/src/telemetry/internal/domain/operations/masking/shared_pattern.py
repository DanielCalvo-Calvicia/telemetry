r"""Compile a pattern written for Go (RE2) so that Python's ``re`` reads it the same way.

The detector patterns are a contract shared with Go (domain/rules.py keeps the texts of the old rules.go, byte for
byte). The two engines agree on the subset the patterns use, except on a few details that this module makes equal. It
is the ONE place that does it:

* ``re.ASCII`` is always on: ``\b``, ``\w`` and ``\d`` are ASCII only, like in Go (a non-ASCII letter next to a key
  is a word boundary).
* Go's ``\s`` is ``[\t\n\f\r ]`` only (no vertical tab, no Unicode space) but Python's ``\s`` also matches the
  vertical tab. Every ``\s`` of the text, inside and outside a character class, is written out as those characters,
  and ``\S`` outside a class as ``[^\t\n\f\r ]`` (``\S`` inside a class is not translated).
* A leading ``(?i)``, ``(?s)`` or ``(?m)`` (alone or combined) is removed from the text and becomes the flag
  ``re.IGNORECASE``, ``re.DOTALL`` or ``re.MULTILINE``, because Python wants global inline flags at the start only.
* Without ``(?m)``, Go's ``$`` and ``\z`` match only at the very end of the text; Python's ``$`` also matches before
  a final newline. Both are written as ``\Z``, which is the very end in Python.
* Case folding. With ``(?i)`` Go uses Unicode simple case folding, so ``k`` also matches the Kelvin sign (U+212A) and
  ``s`` also matches the long s (U+017F); Python with ``re.ASCII`` folds ASCII letters only, which would let a secret
  written with those characters through. When the pattern is case-insensitive, every literal ``s``/``S`` outside a
  class becomes ``[sSſ]`` and every literal ``k``/``K`` becomes ``[kKK]``; a character class that covers
  ``s`` or ``k`` (as a letter or inside a range, in either case, negated or not) gets the extra character too.
  ``\b`` stays ASCII. The names of groups and the letters of the flags of a group are not letters to fold. A scoped
  flag such as ``(?i:...)`` in the middle of a pattern is not folded this way (the detectors do not use one).
* Lazy quantifiers and non-capturing groups are the same in both engines. A Go replacement template (``${1}``) is
  replaced by a replacement function in text_detector.py.

Syntax that only Go has (``\pL``, ``\Q...\E``, POSIX classes) is not translated and is rejected by Python, which makes
the pattern invalid.
"""

import re
from typing import Optional

_WHITESPACE = r"\t\n\f\r "  # the exact Go set, written out
_LEADING_FLAGS = re.compile(r"\(\?([ims]+)\)")
_FLAG_VALUES = {"i": re.IGNORECASE, "s": re.DOTALL, "m": re.MULTILINE}
# The head of a group that has no letter to fold: (?P<name>, (?P=name), (?:, (?i:, (?i) ...
_GROUP_HEAD = re.compile(r"\(\?(?:P<[A-Za-z0-9_]*>|P=[A-Za-z0-9_]*\)|[A-Za-z]*(?:-[A-Za-z]*)?[:)])")

_LONG_S = "ſ"
_KELVIN_SIGN = "K"
_FOLDED_LETTER = {
    "s": "[sS" + _LONG_S + "]",
    "S": "[sS" + _LONG_S + "]",
    "k": "[kK" + _KELVIN_SIGN + "]",
    "K": "[kK" + _KELVIN_SIGN + "]",
}


def translate_pattern(pattern_text: str) -> tuple[str, int]:
    """Return the Python text and the flags (always including re.ASCII) that read ``pattern_text`` like Go does."""
    flags = re.ASCII
    position = 0
    while True:
        leading = _LEADING_FLAGS.match(pattern_text, position)
        if leading is None:
            break
        for letter in leading.group(1):
            flags |= _FLAG_VALUES[letter]
        position = leading.end()
    body = pattern_text[position:]
    translated_text = _translate_body(body, multiline=bool(flags & re.MULTILINE), fold=bool(flags & re.IGNORECASE))
    return translated_text, flags


def _translate_body(body: str, multiline: bool, fold: bool) -> str:
    pieces: list[str] = []
    index = 0
    while index < len(body):
        character = body[index]
        if character == "\\" and index + 1 < len(body):
            escaped = body[index + 1]
            index += 2
            if escaped == "s":
                pieces.append("[" + _WHITESPACE + "]")
            elif escaped == "S":
                pieces.append("[^" + _WHITESPACE + "]")
            elif escaped == "z":
                pieces.append(r"\Z")
            else:
                pieces.append("\\" + escaped)
        elif character == "[":
            class_text, index = _translate_class(body, index, fold)
            pieces.append(class_text)
        elif character == "(":
            group_head = _GROUP_HEAD.match(body, index)
            head_text = group_head.group(0) if group_head is not None else "("
            pieces.append(head_text)
            index += len(head_text)
        elif character == "$" and not multiline:
            pieces.append(r"\Z")
            index += 1
        elif fold and character in _FOLDED_LETTER:
            pieces.append(_FOLDED_LETTER[character])
            index += 1
        else:
            pieces.append(character)
            index += 1
    return "".join(pieces)


def _translate_class(body: str, start: int, fold: bool) -> tuple[str, int]:
    """Translate the character class that opens at ``start``; return its text and the index after its end."""
    index = start + 1
    negation = ""
    if body.startswith("^", index):
        negation = "^"
        index += 1
    members: list[str] = []
    tokens: list[Optional[str]] = []  # the plain characters of the class; None stands for an escape
    if body.startswith("]", index):  # a "]" right after the opening is a literal member of the class
        members.append("\\]")
        tokens.append(None)
        index += 1
    closed = False
    while index < len(body):
        character = body[index]
        if character == "\\" and index + 1 < len(body):
            escaped = body[index + 1]
            members.append(_WHITESPACE if escaped == "s" else "\\" + escaped)
            tokens.append(None)
            index += 2
            continue
        index += 1
        if character == "]":
            closed = True
            break
        members.append(character)
        tokens.append(character)
    extra = _folded_members(tokens) if fold and closed else ""
    class_text = "[" + negation + extra + "".join(members) + ("]" if closed else "")
    return class_text, index


def _folded_members(tokens: list[Optional[str]]) -> str:
    """The non-ASCII characters that Go's case folding adds to a class that covers ``s`` and ``k``."""
    singles: set[str] = set()
    ranges: list[tuple[str, str]] = []
    position = 0
    while position < len(tokens):
        character = tokens[position]
        high = tokens[position + 2] if position + 2 < len(tokens) else None
        if character is not None and high is not None and tokens[position + 1] == "-":
            ranges.append((character, high))
            position += 3
        else:
            if character is not None:
                singles.add(character)
            position += 1

    def covers(letter: str) -> bool:
        for candidate in (letter.lower(), letter.upper()):
            if candidate in singles or any(low <= candidate <= high for low, high in ranges):
                return True
        return False

    extra = ""
    if covers("s"):
        extra += _LONG_S
    if covers("k"):
        extra += _KELVIN_SIGN
    return extra


def compile_shared_pattern(pattern_text: str) -> "re.Pattern[str]":
    """Compile a pattern written for Go; it raises ``re.error`` when the pattern is not valid."""
    translated_text, flags = translate_pattern(pattern_text)
    return re.compile(translated_text, flags)
