"""Black-box tests of the file destination: construction, failures, rotation, quota, concurrency, ports and the core.

Every test works in a temporary directory that is removed at the end.
"""

import dataclasses
import json
import os
import pathlib
import tempfile
import threading
import unittest
from typing import Any
from unittest import mock

import telemetry
from internal.tests.application.services.masking_helpers import envelope_from_json
from internal.tests.eventually import eventually, stop_workers_on_cleanup
from internal.tests.fakes import FakeClock, FakeIdGenerator
from internal.tests.infrastructure.config.helpers import merged_config
from telemetry.internal.application.ports.outbound.event_sink_port import EventSinkPort
from telemetry.internal.application.ports.outbound.log_sink_port import LogSinkPort
from telemetry.internal.domain.errors import ClassifiedError
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.domain.value_objects.event_id import EventId
from telemetry.internal.infrastructure.config.errors import ConfigurationError
from telemetry.internal.infrastructure.outbound.logging.file.error.file_errors import (
    FileRotationFailedError,
    FileSerializationFailedError,
    FileWriteFailedError,
)
from telemetry.internal.infrastructure.outbound.logging.file.external import file_system
from telemetry.internal.infrastructure.outbound.logging.file.file_destination import FileDestination
from telemetry.internal.infrastructure.outbound.logging.file.helpers.encode_json_line import encode_json_line
from telemetry.internal.infrastructure.outbound.logging.file.settings import Settings
from telemetry.logging import file as public_file

BASE = "telemetry.jsonl"


def event(severity: str = "INFO", message: str = "started", attributes: Any = None):
    """A log entry envelope."""
    fields: dict[str, Any] = {
        "schema_version": "0.1",
        "event_id": "01900000-0000-7000-8000-000000000001",
        "event_type": "log.entry.recorded",
        "occurred_at": "2026-10-28T10:00:00.123456Z",
        "recorded_at": "2026-10-28T10:00:00.123456Z",
        "severity": severity,
        "resource": {"service_name": "sensor-hub", "instance_id": "hub-01"},
        "data": {"category": "http", "message": message},
    }
    if attributes is not None:
        fields["attributes"] = attributes
    return envelope_from_json(fields)


def read_lines(path: pathlib.Path) -> list[str]:
    """The lines of a file, each without its line feed."""
    text = path.read_bytes().decode("utf-8")
    return text.split("\n")[:-1] if text else []


class TemporaryDirectoryTestCase(unittest.TestCase):
    def setUp(self) -> None:
        holder = tempfile.TemporaryDirectory()
        self.addCleanup(holder.cleanup)
        self.root = pathlib.Path(holder.name)

    def build(self, **overrides: Any) -> FileDestination:
        settings = {"directory": str(self.root / "logs"), **overrides}
        return FileDestination(Settings(**settings))

    @property
    def logs(self) -> pathlib.Path:
        return self.root / "logs"


class ConstructionTest(TemporaryDirectoryTestCase):
    def test_invalid_settings_raise_a_configuration_error_with_every_problem(self):
        with self.assertRaises(ConfigurationError) as raised:
            FileDestination(Settings(directory="", file_name="a/b", max_files=0))
        self.assertEqual(
            [problem.field for problem in raised.exception.problems], ["directory", "file_name", "max_files"]
        )
        self.assertNotIn("a/b", str(raised.exception))

    def test_a_value_of_another_type_is_a_problem_and_a_reason_never_holds_a_value(self):
        wrong = [
            {"directory": None},
            {"directory": 3},
            {"directory": "bad\x00name"},
            {"file_name": None},
            {"file_name": "bad\x7fname"},
            {"file_name": "."},
            {"minimum_severity": 3},
            {"minimum_severity": "Info"},
            {"max_file_bytes": True},
            {"max_file_bytes": "10"},
            {"max_files": 1.5},
            {"quota_bytes": None},
        ]
        for overrides in wrong:
            with self.subTest(overrides=overrides):
                settings = Settings(**{"directory": str(self.logs), **overrides})  # type: ignore[arg-type]
                with self.assertRaises(ConfigurationError) as raised:
                    FileDestination(settings)
                for problem in raised.exception.problems:
                    for value in overrides.values():
                        if isinstance(value, str) and len(value) > 3:
                            self.assertNotIn(value, problem.reason)
        self.assertFalse(self.logs.exists())

    def test_a_quota_below_the_file_size_is_a_problem_on_the_quota(self):
        with self.assertRaises(ConfigurationError) as raised:
            self.build(max_file_bytes=100, quota_bytes=99)
        self.assertEqual([problem.field for problem in raised.exception.problems], ["quota_bytes"])

    def test_a_directory_that_is_an_existing_file_gives_one_problem_without_the_path(self):
        blocker = self.root / "blocker-file"
        blocker.write_bytes(b"x")
        with self.assertRaises(ConfigurationError) as raised:
            FileDestination(Settings(directory=str(blocker)))
        problems = raised.exception.problems
        self.assertEqual([problem.field for problem in problems], ["directory"])
        self.assertEqual(problems[0].reason, "the directory cannot be written")
        self.assertNotIn("blocker-file", str(raised.exception))
        self.assertNotIn(str(self.root), str(raised.exception))

    def test_a_base_name_that_is_a_directory_is_a_directory_problem(self):
        (self.logs / BASE).mkdir(parents=True)
        with self.assertRaises(ConfigurationError) as raised:
            self.build()
        self.assertEqual([problem.field for problem in raised.exception.problems], ["directory"])

    def test_a_missing_directory_is_created_with_the_current_file_and_owner_only(self):
        destination = self.build(directory=str(self.root / "a" / "b"))
        self.assertIsInstance(destination, FileDestination)
        base = self.root / "a" / "b" / BASE
        self.assertTrue(base.is_file())
        self.assertEqual(base.read_bytes(), b"")
        if os.name != "nt":
            self.assertEqual(base.stat().st_mode & 0o777, 0o600)
            self.assertEqual((self.root / "a" / "b").stat().st_mode & 0o777, 0o700)

    def test_a_file_created_by_a_write_is_owner_only(self):
        destination = self.build(file_name="other.jsonl")
        destination.send([event()])
        if os.name != "nt":
            self.assertEqual((self.logs / "other.jsonl").stat().st_mode & 0o777, 0o600)

    def test_an_existing_directory_and_file_are_kept_and_appended_to(self):
        self.logs.mkdir()
        (self.logs / BASE).write_bytes(b"old line\n")
        self.build().send([event()])
        lines = read_lines(self.logs / BASE)
        self.assertEqual(lines[0], "old line")
        self.assertEqual(len(lines), 2)

    def test_it_serves_both_ports(self):
        destination = self.build()
        self.assertIsInstance(destination, LogSinkPort)
        self.assertIsInstance(destination, EventSinkPort)

    def test_the_counters_start_at_zero_and_cannot_be_changed(self):
        counters = self.build().counters
        self.assertEqual(dataclasses.astuple(counters), (0, 0, 0, 0, 0))
        with self.assertRaises(dataclasses.FrozenInstanceError):
            counters.files_rotated = 1  # type: ignore[misc]


class SendTest(TemporaryDirectoryTestCase):
    def test_an_event_below_the_minimum_is_not_written_and_counts_as_accepted(self):
        result = self.build(minimum_severity="WARN").send(
            [event("DEBUG"), event("INFO"), event("WARN"), event("CRITICAL")]
        )
        self.assertEqual((result.accepted, result.dropped, result.failed, result.reason), (4, 0, 0, ""))
        self.assertEqual(len(read_lines(self.logs / BASE)), 2)

    def test_an_empty_batch_writes_nothing(self):
        result = self.build().send([])
        self.assertEqual((result.accepted, result.dropped, result.failed, result.reason), (0, 0, 0, ""))
        self.assertEqual((self.logs / BASE).read_bytes(), b"")

    def test_each_line_is_the_canonical_json_of_the_event(self):
        item = event(message="héllo ✓", attributes={"order": 7})
        self.build().send([item])
        self.assertEqual((self.logs / BASE).read_bytes(), encode_json_line(item))
        self.assertTrue(encode_json_line(item).endswith(b"\n"))

    def test_an_event_that_cannot_be_encoded_fails_and_the_others_are_written(self):
        events = [event(message="before"), event(attributes={"ratio": float("nan")}), event(message="after")]
        result = self.build().send(events)
        self.assertEqual((result.accepted, result.dropped, result.failed), (2, 0, 1))
        self.assertEqual(result.reason, "serialization_failed")
        text = (self.logs / BASE).read_text(encoding="utf-8")
        self.assertIn("before", text)
        self.assertIn("after", text)

    def test_the_directory_of_the_files_can_hold_other_files_that_are_left_alone(self):
        self.logs.mkdir()
        (self.logs / "notes.txt").write_bytes(b"keep")
        (self.logs / "telemetry.jsonl.x").write_bytes(b"keep")
        self.build(max_file_bytes=10, quota_bytes=10, max_files=2).send([event(), event(), event()])
        self.assertEqual((self.logs / "notes.txt").read_bytes(), b"keep")
        self.assertEqual((self.logs / "telemetry.jsonl.x").read_bytes(), b"keep")

    def test_something_unexpected_fails_the_events_not_yet_counted(self):
        destination = self.build()
        result = destination.send([event(), None, event()])  # type: ignore[list-item]
        self.assertEqual((result.accepted, result.dropped, result.failed), (1, 0, 2))
        self.assertEqual(result.reason, "destination_failed")
        self.assertEqual(len(read_lines(self.logs / BASE)), 1)
        self.assertEqual(destination.send([event()]).accepted, 1)  # the lock was released

    def test_a_batch_that_cannot_be_read_does_not_raise(self):
        def broken_batch():
            raise RuntimeError("no batch")
            yield  # pragma: no cover

        result = self.build().send(broken_batch())  # type: ignore[arg-type]
        self.assertEqual((result.accepted, result.dropped, result.failed), (0, 0, 0))
        self.assertEqual(result.reason, "destination_failed")

    def test_a_keyboard_interrupt_is_not_swallowed_and_the_lock_is_released(self):
        destination = self.build()
        with mock.patch.object(file_system, "append_line", side_effect=KeyboardInterrupt()):
            with self.assertRaises(KeyboardInterrupt):
                destination.send([event()])
        self.assertEqual(destination.send([event()]).accepted, 1)

    def test_the_send_result_is_a_send_result(self):
        self.assertIsInstance(self.build().send([event()]), telemetry.SendResult)


class FailureTest(TemporaryDirectoryTestCase):
    def replace_base_with_a_directory(self) -> None:
        (self.logs / BASE).unlink()
        (self.logs / BASE).mkdir()

    def test_a_failed_write_fails_that_event_counts_the_error_and_raises_nothing(self):
        destination = self.build()
        self.replace_base_with_a_directory()
        result = destination.send([event(), event(), event(severity="DEBUG")])
        self.assertEqual((result.accepted, result.dropped, result.failed), (1, 0, 2))
        self.assertEqual(result.reason, "write_failed")
        self.assertEqual(destination.counters.write_errors, 2)

    def test_a_failure_to_inspect_the_file_is_a_failed_write(self):
        destination = self.build()
        with mock.patch.object(file_system, "file_size", side_effect=PermissionError("secret path")):
            result = destination.send([event()])
        self.assertEqual((result.accepted, result.failed, result.reason), (0, 1, "write_failed"))
        self.assertEqual(destination.counters.write_errors, 1)
        self.assertNotIn("secret path", result.reason)

    def test_a_failed_rotation_fails_the_event_and_the_next_one_tries_again(self):
        destination = self.build(max_file_bytes=10, quota_bytes=1000, max_files=2)
        self.assertEqual(destination.send([event(message="first")]).accepted, 1)
        target = self.logs / f"{BASE}.1"
        target.mkdir()
        (target / "keep").write_bytes(b"x")  # a non-empty directory cannot be removed
        result = destination.send([event(message="second"), event(message="third")])
        self.assertEqual((result.accepted, result.failed, result.reason), (0, 2, "rotation_failed"))
        counters = destination.counters
        self.assertEqual((counters.files_rotated, counters.rotation_errors), (0, 2))
        self.assertEqual(len(read_lines(self.logs / BASE)), 1)  # the new events were not written
        (target / "keep").unlink()
        target.rmdir()
        result = destination.send([event(message="fourth")])
        self.assertEqual((result.accepted, result.failed), (1, 0))
        self.assertEqual(destination.counters.files_rotated, 1)

    def test_a_failed_quota_deletion_is_counted_and_does_not_fail_the_event(self):
        destination = self.build(max_file_bytes=10, quota_bytes=10, max_files=3)
        target = self.logs / f"{BASE}.1"
        target.mkdir()
        (target / "keep").write_bytes(b"x")
        result = destination.send([event()])
        self.assertEqual((result.accepted, result.failed, result.reason), (1, 0, ""))
        counters = destination.counters
        self.assertEqual((counters.files_deleted_for_quota, counters.quota_errors), (0, 1))

    def test_a_failure_to_measure_the_files_for_the_quota_is_counted(self):
        destination = self.build()
        real = file_system.file_size
        calls = {"count": 0}

        def failing_after_the_first(path: str):
            calls["count"] += 1
            if calls["count"] > 1:
                raise PermissionError("secret path")
            return real(path)

        with mock.patch.object(file_system, "file_size", side_effect=failing_after_the_first):
            result = destination.send([event()])
        self.assertEqual((result.accepted, result.failed), (1, 0))
        self.assertEqual(destination.counters.quota_errors, 1)

    def test_the_errors_are_classified_and_chained_from_their_cause(self):
        for error_class in (FileWriteFailedError, FileRotationFailedError, FileSerializationFailedError):
            with self.subTest(error=error_class.__name__):
                try:
                    try:
                        raise OSError("disk full")
                    except OSError as cause:
                        raise error_class("something failed") from cause
                except ClassifiedError as error:
                    self.assertIsInstance(error.__cause__, OSError)
                    self.assertNotIn("disk full", str(error))


class RotationAndQuotaTest(TemporaryDirectoryTestCase):
    def test_a_line_larger_than_the_file_is_written_to_an_empty_file(self):
        destination = self.build(max_file_bytes=1, quota_bytes=1, max_files=3)
        destination.send([event()])
        self.assertEqual(len(read_lines(self.logs / BASE)), 1)
        self.assertEqual(destination.counters.files_rotated, 0)

    def test_the_files_never_hold_more_than_max_files_and_the_quota_removes_the_oldest(self):
        line_size = len(encode_json_line(event()))
        destination = self.build(max_file_bytes=line_size, quota_bytes=2 * line_size, max_files=5)
        destination.send([event(message="a") for _ in range(6)])
        names = sorted(path.name for path in self.logs.iterdir())
        self.assertEqual(names, [BASE, f"{BASE}.1"])
        counters = destination.counters
        self.assertGreater(counters.files_rotated, 0)
        self.assertGreater(counters.files_deleted_for_quota, 0)
        self.assertEqual((counters.write_errors, counters.rotation_errors, counters.quota_errors), (0, 0, 0))


class ConcurrencyTest(TemporaryDirectoryTestCase):
    def test_the_lines_of_sixteen_threads_never_interleave(self):
        destination = self.build()
        per_thread = 12
        results: list[Any] = []

        def work(number: int) -> None:
            batch = [event(message=f"thread {number} line {line} " + "x" * 200) for line in range(per_thread)]
            results.append(destination.send(batch))

        threads = [threading.Thread(target=work, args=(number,)) for number in range(16)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join()
        lines = read_lines(self.logs / BASE)
        self.assertEqual(len(lines), 16 * per_thread)
        messages = set()
        for line in lines:
            messages.add(json.loads(line)["data"]["message"])
        self.assertEqual(len(messages), 16 * per_thread)
        self.assertEqual(sum(result.accepted for result in results), 16 * per_thread)

    def test_the_counters_are_consistent_under_concurrency(self):
        line_size = len(encode_json_line(event()))
        destination = self.build(max_file_bytes=3 * line_size, quota_bytes=6 * line_size, max_files=4)
        done = threading.Event()
        seen: list[Any] = []

        def reader() -> None:
            while not done.wait(0.001):
                seen.append(destination.counters)

        def work() -> None:
            destination.send([event() for _ in range(10)])

        reader_thread = threading.Thread(target=reader)
        reader_thread.start()
        threads = [threading.Thread(target=work) for _ in range(16)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join()
        done.set()
        reader_thread.join()
        counters = destination.counters
        # 160 lines, 3 per file: a rotation before every fourth line (the first line goes to an empty file).
        self.assertEqual(counters.files_rotated, (16 * 10 - 1) // 3)
        self.assertGreater(counters.files_deleted_for_quota, 0)
        self.assertEqual((counters.write_errors, counters.rotation_errors, counters.quota_errors), (0, 0, 0))
        self.assertTrue(all(item.files_rotated <= counters.files_rotated for item in seen))
        for line in (line for path in self.logs.iterdir() for line in read_lines(path)):
            json.loads(line)


class ExternalTest(TemporaryDirectoryTestCase):
    def test_the_size_of_a_missing_file_is_none_and_the_names_are_listed(self):
        self.assertIsNone(file_system.file_size(str(self.root / "missing")))
        (self.root / "one").write_bytes(b"abc")
        self.assertEqual(file_system.file_size(str(self.root / "one")), 3)
        self.assertEqual(file_system.list_names(str(self.root)), ["one"])
        self.assertEqual(file_system.join_path("a", "b"), os.path.join("a", "b"))

    def test_the_other_errors_of_the_size_are_raised(self):
        with mock.patch.object(file_system.os, "stat", side_effect=PermissionError("no")):
            with self.assertRaises(OSError):
                file_system.file_size("anything")

    def test_a_short_write_is_completed_with_more_writes(self):
        path = str(self.root / "short")
        real_write = os.write

        def two_bytes_at_a_time(descriptor: int, data: Any) -> int:
            return real_write(descriptor, bytes(data[:2]))

        with mock.patch.object(file_system.os, "write", side_effect=two_bytes_at_a_time):
            file_system.append_line(path, b"hello\n")
        self.assertEqual(pathlib.Path(path).read_bytes(), b"hello\n")

    def test_append_adds_to_the_end_and_rename_and_remove_work(self):
        path = str(self.root / "one")
        file_system.append_line(path, b"a\n")
        file_system.append_line(path, b"b\n")
        file_system.touch_append(path)
        self.assertEqual(pathlib.Path(path).read_bytes(), b"a\nb\n")
        file_system.rename(path, str(self.root / "two"))
        self.assertFalse(pathlib.Path(path).exists())
        file_system.remove(str(self.root / "two"))
        self.assertEqual(list(self.root.iterdir()), [])

    def test_the_lines_keep_a_bare_line_feed_on_every_platform(self):
        path = str(self.root / "lf")
        file_system.append_line(path, b"a\n")
        self.assertEqual(pathlib.Path(path).read_bytes(), b"a\n")


class PublicPackageTest(TemporaryDirectoryTestCase):
    def test_the_defaults(self):
        self.assertEqual(
            dataclasses.asdict(public_file.default_settings()),
            {
                "directory": "",
                "file_name": "telemetry.jsonl",
                "minimum_severity": "INFO",
                "max_file_bytes": 10485760,
                "max_files": 5,
                "quota_bytes": 52428800,
            },
        )

    def test_a_destination_with_invalid_settings_raises(self):
        with self.assertRaises(ConfigurationError):
            public_file.new_destination(public_file.default_settings())

    def test_the_standard_library_logging_still_imports(self):
        import logging

        self.assertTrue(hasattr(logging, "getLogger"))
        self.assertEqual(public_file.__name__, "telemetry.logging.file")
        self.assertIs(public_file.Destination, FileDestination)

    def test_the_whole_core_writes_the_canonical_json_of_the_masked_event(self):
        sink = public_file.new_destination(public_file.Settings(directory=str(self.logs)))
        options = [
            telemetry.with_log_sink(sink),
            telemetry.with_clock(FakeClock()),
            telemetry.with_id_generator(FakeIdGenerator([EventId("01900000-0000-7000-8000-000000000001")])),
        ]
        core = stop_workers_on_cleanup(
            self,
            telemetry.new(
                merged_config(
                    "microservice", {"resource": {"service_name": "sensor-hub"}, "delivery": {"flush_interval_ms": 5}}
                ),
                *options,
            ),
        )
        core.logger.info(
            telemetry.LogEntry(
                category="http", message="login password=placeholder-secret-not-real", fields={"order": 7}
            )
        )
        eventually(5.0, lambda: (self.logs / BASE).exists() and read_lines(self.logs / BASE), "the line of the worker")
        core.shutdown()
        lines = read_lines(self.logs / BASE)
        self.assertEqual(len(lines), 1)
        self.assertIn("login password=*****MASKED*****", lines[0])
        self.assertNotIn("placeholder-secret-not-real", lines[0])
        self.assertIn('"event_type":"log.entry.recorded"', lines[0])
        self.assertEqual(lines[0], canonical_json(json.loads(lines[0])).decode("utf-8"))
