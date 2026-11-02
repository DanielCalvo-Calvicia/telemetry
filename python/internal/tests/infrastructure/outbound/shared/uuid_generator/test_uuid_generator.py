"""Black-box tests of UuidGenerator."""

import itertools
import re
import unittest

from telemetry.internal.application.errors import IdGenerationFailedError
from telemetry.internal.application.ports.outbound.id_generator_port import IDGeneratorPort
from telemetry.internal.domain.base_value_objects.identifier import Identifier
from telemetry.internal.domain.errors import error_code_of
from telemetry.internal.domain.value_objects.error_codes_generated import ERROR_CODE_EVENT_BUILD_FAILED
from telemetry.internal.domain.value_objects.event_id import EventId
from telemetry.internal.infrastructure.outbound.shared.uuid_generator.uuid_generator import UuidGenerator

SHAPE = re.compile(r"[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}")


class UuidGeneratorTest(unittest.TestCase):
    def test_it_is_an_id_generator_port(self):
        self.assertIsInstance(UuidGenerator(), IDGeneratorPort)

    def test_an_event_id_is_a_lowercase_uuid_of_version_7_and_variant_10(self):
        generator = UuidGenerator()
        for _ in range(200):
            event_id = generator.new_event_id()
            self.assertIsInstance(event_id, EventId)
            self.assertRegex(event_id.value, SHAPE)
            self.assertEqual(event_id.value, event_id.value.lower())

    def test_an_instance_id_is_the_text_of_a_uuid_v7(self):
        instance_id = UuidGenerator().new_instance_id()
        self.assertIsInstance(instance_id, Identifier)
        self.assertRegex(instance_id.value, SHAPE)

    def test_the_time_prefix_never_goes_back_and_20000_ids_are_unique(self):
        milliseconds = itertools.count(1_700_000_000_000)
        generator = UuidGenerator(read_milliseconds=lambda: next(milliseconds) // 3)
        ids = [generator.new_event_id().value for _ in range(20_000)]
        self.assertEqual(len(set(ids)), 20_000)
        prefixes = [text[:13] for text in ids]
        self.assertEqual(prefixes, sorted(prefixes))

    def test_the_real_sources_give_unique_ids(self):
        generator = UuidGenerator()
        ids = {generator.new_event_id().value for _ in range(20_000)}
        self.assertEqual(len(ids), 20_000)

    def test_the_injected_sources_are_used(self):
        asked = []

        def read_random(byte_count: int) -> bytes:
            asked.append(byte_count)
            return b"\xff" * byte_count

        generator = UuidGenerator(read_milliseconds=lambda: 0x0123456789AB, read_random=read_random)
        self.assertEqual(generator.new_event_id().value, "01234567-89ab-7fff-bfff-ffffffffffff")
        self.assertEqual(asked, [10])

    def test_a_failure_of_the_random_source_is_an_id_generation_error_with_the_cause(self):
        cause = OSError("no entropy: secret-detail")

        def read_random(byte_count: int) -> bytes:
            raise cause

        generator = UuidGenerator(read_random=read_random)
        for call in (generator.new_event_id, generator.new_instance_id):
            with self.subTest(call=call.__name__):
                with self.assertRaises(IdGenerationFailedError) as raised:
                    call()
                self.assertIs(raised.exception.__cause__, cause)
                self.assertEqual(error_code_of(raised.exception), ERROR_CODE_EVENT_BUILD_FAILED)
                self.assertNotIn("secret-detail", str(raised.exception))

    def test_a_failure_of_the_clock_is_also_translated(self):
        def read_milliseconds() -> int:
            raise RuntimeError("clock")

        with self.assertRaises(IdGenerationFailedError) as raised:
            UuidGenerator(read_milliseconds=read_milliseconds).new_event_id()
        self.assertIsInstance(raised.exception.__cause__, RuntimeError)

    def test_a_source_that_gives_too_few_bytes_is_a_failure_not_a_weaker_id(self):
        with self.assertRaises(IdGenerationFailedError) as raised:
            UuidGenerator(read_random=lambda byte_count: b"\x01").new_event_id()
        self.assertIsInstance(raised.exception.__cause__, ValueError)


if __name__ == "__main__":
    unittest.main()
