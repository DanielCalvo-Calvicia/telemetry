"""Helpers for the tests of an asynchronous delivery: wait for a fact, stop the workers, shorten the flush interval."""

import dataclasses
import time
import unittest
from typing import Any, Callable

import telemetry

SHORT_FLUSH_INTERVAL_MS = 5
POLL_SECONDS = 0.002


def eventually(timeout: float, condition: Callable[[], bool], message: str = "the condition") -> None:
    """Poll the condition with a short sleep until it is true; fail with a clear message after ``timeout`` seconds."""
    deadline = time.monotonic() + timeout
    while True:
        if condition():
            return
        if time.monotonic() >= deadline:
            raise AssertionError(f"{message} did not become true within {timeout} s")
        time.sleep(POLL_SECONDS)


def stop_workers_on_cleanup(test: unittest.TestCase, core: Any) -> Any:
    """Stop the delivery workers of the core when the test ends, so no thread outlives it; return the core."""
    test.addCleanup(core._delivery.stop)
    return core


def with_short_flush(config: telemetry.Config, flush_interval_ms: int = SHORT_FLUSH_INTERVAL_MS) -> telemetry.Config:
    """The configuration with a short flush interval, so a batch that is not full leaves almost at once."""
    delivery = dataclasses.replace(config.delivery, flush_interval_ms=flush_interval_ms)
    return dataclasses.replace(config, delivery=delivery)
