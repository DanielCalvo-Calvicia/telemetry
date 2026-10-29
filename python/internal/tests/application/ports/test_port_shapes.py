"""The fakes satisfy their ports, a plain object does not, and each port has the methods of the Go table."""

import unittest

from internal.tests.fakes import (
    FakeAiTelemetrySink,
    FakeClock,
    FakeConfiguration,
    FakeDeviceTelemetrySink,
    FakeEventSink,
    FakeIdGenerator,
    FakeLocalSpool,
    FakeLogSink,
    FakeMetricsSink,
    FakeTraceSink,
)
from telemetry.internal.application.ports.outbound.ai_telemetry_sink_port import AITelemetrySinkPort
from telemetry.internal.application.ports.outbound.clock_port import ClockPort
from telemetry.internal.application.ports.outbound.configuration_port import ConfigurationPort
from telemetry.internal.application.ports.outbound.device_telemetry_sink_port import DeviceTelemetrySinkPort
from telemetry.internal.application.ports.outbound.event_sink_port import EventSinkPort
from telemetry.internal.application.ports.outbound.id_generator_port import IDGeneratorPort
from telemetry.internal.application.ports.outbound.local_spool_port import LocalSpoolPort
from telemetry.internal.application.ports.outbound.log_sink_port import LogSinkPort
from telemetry.internal.application.ports.outbound.metrics_sink_port import MetricsSinkPort
from telemetry.internal.application.ports.outbound.trace_sink_port import TraceSinkPort

# Port, its fake and its methods. The Go half lists the same methods in PascalCase (Now, MonotonicNanoseconds, ...).
PORT_TABLE = [
    (ClockPort, FakeClock, {"now", "monotonic_nanoseconds", "clock_info"}),
    (IDGeneratorPort, FakeIdGenerator, {"new_event_id", "new_instance_id"}),
    (ConfigurationPort, FakeConfiguration, {"configuration"}),
    (LogSinkPort, FakeLogSink, {"send"}),
    (EventSinkPort, FakeEventSink, {"send"}),
    (MetricsSinkPort, FakeMetricsSink, {"send"}),
    (TraceSinkPort, FakeTraceSink, {"send"}),
    (DeviceTelemetrySinkPort, FakeDeviceTelemetrySink, {"send"}),
    (AITelemetrySinkPort, FakeAiTelemetrySink, {"send"}),
    (LocalSpoolPort, FakeLocalSpool, {"append", "read_batch", "acknowledge", "usage", "cleanup"}),
]


def public_methods(port: type) -> set:
    """The names the Protocol declares, without dunder and private names."""
    return {name for name, value in vars(port).items() if callable(value) and not name.startswith("_")}


class PortShapeTest(unittest.TestCase):
    """Each fake satisfies its port; each port has exactly the methods of the table."""

    def test_every_fake_satisfies_its_port(self) -> None:
        for port, fake, _ in PORT_TABLE:
            with self.subTest(port=port.__name__):
                self.assertIsInstance(fake(), port)

    def test_an_object_without_the_methods_does_not_satisfy_any_port(self) -> None:
        for port, _, _ in PORT_TABLE:
            with self.subTest(port=port.__name__):
                self.assertNotIsInstance(object(), port)

    def test_an_object_missing_one_method_does_not_satisfy_the_port(self) -> None:
        for port, _, methods in PORT_TABLE:
            for missing in methods:
                with self.subTest(port=port.__name__, missing=missing):
                    attributes = {name: (lambda self, *arguments: None) for name in methods if name != missing}
                    incomplete = type("Incomplete", (), attributes)()
                    self.assertNotIsInstance(incomplete, port)

    def test_the_methods_of_every_port_match_the_table(self) -> None:
        for port, _, methods in PORT_TABLE:
            with self.subTest(port=port.__name__):
                self.assertEqual(methods, public_methods(port))

    def test_every_port_and_method_is_documented(self) -> None:
        for port, _, methods in PORT_TABLE:
            with self.subTest(port=port.__name__):
                self.assertTrue((port.__doc__ or "").startswith(port.__name__))
                for name in methods:
                    self.assertTrue(getattr(port, name).__doc__)

    def test_the_six_sink_ports_share_one_method_shape(self) -> None:
        sinks = (
            LogSinkPort,
            EventSinkPort,
            MetricsSinkPort,
            TraceSinkPort,
            DeviceTelemetrySinkPort,
            AITelemetrySinkPort,
        )
        for port in sinks:
            with self.subTest(port=port.__name__):
                self.assertEqual({"send"}, public_methods(port))
                self.assertEqual(
                    LogSinkPort.send.__annotations__.keys(),
                    port.send.__annotations__.keys(),
                )


if __name__ == "__main__":
    unittest.main()
