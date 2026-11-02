"""The harness shared by the cases of the core and of the logger: the options and the fake sinks of a case."""

import datetime
from typing import Any

import telemetry
from internal.tests.fakes import (
    FakeAiTelemetrySink,
    FakeClock,
    FakeDeviceTelemetrySink,
    FakeEventSink,
    FakeIdGenerator,
    FakeLocalSpool,
    FakeLogSink,
    FakeMetricsSink,
    FakeTraceSink,
)
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.domain.value_objects.event_id import EventId

# The key of a fixture, the fake of its port and the option that adds it, in one place.
SINK_KINDS = {
    "log": (FakeLogSink, telemetry.with_log_sink),
    "event": (FakeEventSink, telemetry.with_event_sink),
    "metrics": (FakeMetricsSink, telemetry.with_metrics_sink),
    "trace": (FakeTraceSink, telemetry.with_trace_sink),
    "device_telemetry": (FakeDeviceTelemetrySink, telemetry.with_device_telemetry_sink),
    "ai_telemetry": (FakeAiTelemetrySink, telemetry.with_ai_telemetry_sink),
}


def parse_time(text: str) -> datetime.datetime:
    """Parse an ISO time with an offset and return it in UTC."""
    return datetime.datetime.fromisoformat(text).astimezone(datetime.timezone.utc)


def build_options(case: dict[str, Any]) -> tuple[list, dict[str, list]]:
    """Return the options of the case and the fake sinks it builds, by kind."""
    clock = FakeClock(wall_time=parse_time(case["clock"]["wall_time"]))
    event_ids = [EventId(text) for text in case["event_ids"]]
    options = [telemetry.with_clock(clock), telemetry.with_id_generator(FakeIdGenerator(event_ids))]
    sinks: dict[str, list] = {}
    for kind, entries in case["sinks"].items():
        fake_class, with_sink = SINK_KINDS[kind]
        sinks[kind] = []
        for entry in entries:
            sink = fake_class()
            result = entry.get("result")
            if result is not None:
                sink.set_result(telemetry.SendResult(**result))
            sinks[kind].append(sink)
            options.append(with_sink(sink))
    if case.get("spool"):
        options.append(telemetry.with_local_spool(FakeLocalSpool()))
    return options, sinks


def delivered_texts(sinks: list) -> list[list[str]]:
    """The canonical JSON text of every envelope each sink received, in order."""
    return [
        [canonical_json(envelope).decode("utf-8") for batch in sink.batches for envelope in batch] for sink in sinks
    ]
