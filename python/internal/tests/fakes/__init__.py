"""Fakes of the outbound ports: they record calls and can be told to fail or to delay. They live with the tests."""

from internal.tests.fakes.fake_clock import FakeClock
from internal.tests.fakes.fake_configuration import FakeConfiguration
from internal.tests.fakes.fake_id_generator import FakeIdGenerator
from internal.tests.fakes.fake_local_spool import FakeLocalSpool
from internal.tests.fakes.fake_sinks import (
    FakeAiTelemetrySink,
    FakeDeviceTelemetrySink,
    FakeEventSink,
    FakeLogSink,
    FakeMetricsSink,
    FakeTraceSink,
)

__all__ = [
    "FakeAiTelemetrySink",
    "FakeClock",
    "FakeConfiguration",
    "FakeDeviceTelemetrySink",
    "FakeEventSink",
    "FakeIdGenerator",
    "FakeLocalSpool",
    "FakeLogSink",
    "FakeMetricsSink",
    "FakeTraceSink",
]
