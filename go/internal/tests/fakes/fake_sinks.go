package fakes

// FakeLogSink is a fake of outbound.LogSinkPort. It records the batches, returns the result that was set and can
// be told to delay (Send, SetResult, SetDelay, Batches and Calls).
type FakeLogSink struct{ sink_recorder }

// FakeEventSink is a fake of outbound.EventSinkPort, with the behavior of FakeLogSink.
type FakeEventSink struct{ sink_recorder }

// FakeMetricsSink is a fake of outbound.MetricsSinkPort, with the behavior of FakeLogSink.
type FakeMetricsSink struct{ sink_recorder }

// FakeTraceSink is a fake of outbound.TraceSinkPort, with the behavior of FakeLogSink.
type FakeTraceSink struct{ sink_recorder }

// FakeDeviceTelemetrySink is a fake of outbound.DeviceTelemetrySinkPort, with the behavior of FakeLogSink.
type FakeDeviceTelemetrySink struct{ sink_recorder }

// FakeAITelemetrySink is a fake of outbound.AITelemetrySinkPort, with the behavior of FakeLogSink.
type FakeAITelemetrySink struct{ sink_recorder }
