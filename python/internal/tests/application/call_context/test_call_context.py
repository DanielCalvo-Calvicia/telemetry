"""The call context: the six tests of the old call_depth package, adapted, and the rules of the new fields."""

import dataclasses
import unittest

from internal.tests.fakes import FakeIdGenerator
from telemetry.internal.application.call_context.call_context import (
    STARTING_INDENTATION,
    CallContext,
    current_call_context,
    deeper_indentation,
    use_call_context,
)
from telemetry.internal.application.call_context.correlation import to_correlation_context
from telemetry.internal.application.call_context.new_call_context import new_call_context
from telemetry.internal.application.call_context.request_context import start_request
from telemetry.internal.application.errors import (
    CallContextFieldInvalidError,
    CallContextIdAllZerosError,
    CallContextIdFailedError,
)
from telemetry.internal.domain.errors import error_code_of

SECRET = "tok-SECRET-9f8e7d"


class OldCallDepthTest(unittest.TestCase):
    def test_a_program_without_call_context_has_none(self):
        self.assertIsNone(current_call_context())

    def test_use_call_context_stores_and_reads_back_every_value(self):
        stored = CallContext(application="app", instance_id="instance", execution_id="run", indentation=2)
        with use_call_context(stored):
            self.assertEqual(current_call_context(), stored)
        self.assertIsNone(current_call_context())

    def test_the_context_is_restored_when_the_block_raises(self):
        with self.assertRaises(RuntimeError):
            with use_call_context(CallContext(application="app")):
                raise RuntimeError("boom")
        self.assertIsNone(current_call_context())

    def test_start_request_starts_a_request(self):
        generator = FakeIdGenerator()
        result = start_request(generator, "app", "instance")
        self.assertIsNone(result.error)
        expected_id = "01900000-0000-7000-8000-000000000001"
        self.assertEqual(
            result.call_context,
            CallContext(application="app", instance_id="instance", correlation_id=expected_id, indentation=1),
        )
        self.assertEqual(STARTING_INDENTATION, 1)

    def test_start_request_when_the_id_cannot_be_generated_keeps_the_root_cause(self):
        root_cause = OSError("random source unavailable")
        generator = FakeIdGenerator()
        generator.fail_with(root_cause)
        result = start_request(generator, "app", "instance")
        self.assertIsNone(result.call_context)
        self.assertIsInstance(result.error, CallContextIdFailedError)
        self.assertIs(result.error.__cause__, root_cause)
        self.assertEqual(error_code_of(result.error), "event_build_failed")
        self.assertIsNone(current_call_context())

    def test_start_request_validates_the_application_and_the_instance(self):
        for application_name, instance_id, field_name in (
            ("  ", "instance", "application"),
            ("app", "bad\x07id", "instance_id"),
            (f"{SECRET}\n", "instance", "application"),
        ):
            with self.subTest(application=field_name):
                result = start_request(FakeIdGenerator(), application_name, instance_id)
                self.assertIsNone(result.call_context)
                self.assertIsInstance(result.error, CallContextFieldInvalidError)
                self.assertEqual(error_code_of(result.error), "event_schema_invalid")
                self.assertIn(field_name, str(result.error))
                self.assertNotIn(SECRET, str(result.error))
                self.assertIsNone(current_call_context())

    def test_deeper_indentation_goes_one_layer_deeper_without_touching_the_original(self):
        original = CallContext(correlation_id="request", indentation=1)
        with use_call_context(original):
            with deeper_indentation() as went_deeper:
                self.assertTrue(went_deeper)
                deeper = current_call_context()
                with deeper_indentation():
                    deepest = current_call_context()
                self.assertEqual(current_call_context(), deeper)
            self.assertIs(current_call_context(), original)
        self.assertEqual((original.indentation, deeper.indentation, deepest.indentation), (1, 2, 3))
        self.assertEqual(deepest.correlation_id, "request")

    def test_deeper_indentation_restores_the_context_when_the_block_raises(self):
        original = CallContext(indentation=1)
        with use_call_context(original):
            with self.assertRaises(RuntimeError):
                with deeper_indentation():
                    raise RuntimeError("boom")
            self.assertIs(current_call_context(), original)

    def test_deeper_indentation_without_call_context_does_nothing(self):
        with deeper_indentation() as went_deeper:
            self.assertFalse(went_deeper)
            self.assertIsNone(current_call_context())

    def test_use_call_context_with_none_hides_the_outer_context(self):
        with use_call_context(CallContext(application="app")):
            with use_call_context(None):
                self.assertIsNone(current_call_context())
            self.assertEqual(current_call_context().application, "app")


class NewCallContextTest(unittest.TestCase):
    def test_a_context_with_none_gives_empty_values_not_errors(self):
        result = new_call_context()
        self.assertIsNone(result.error)
        self.assertEqual(result.call_context, CallContext())
        self.assertEqual(result.call_context.execution_id, "")

    def test_the_given_values_come_back(self):
        result = new_call_context(
            application="app",
            instance_id="i-1",
            execution_id="run-7",
            correlation_id="order-9",
            trace_id="0af7651916cd43dd8448eb211c80319c",
            span_id="b7ad6b7169203331",
        )
        self.assertIsNone(result.error)
        self.assertEqual(result.call_context.indentation, STARTING_INDENTATION)
        self.assertEqual(result.call_context.span_id, "b7ad6b7169203331")

    def test_the_values_are_never_trimmed(self):
        self.assertEqual(new_call_context(application=" app ").call_context.application, " app ")

    def test_all_the_problems_come_together_in_one_error_in_order(self):
        result = new_call_context(
            application=" ",
            instance_id="a\nb",
            execution_id="",
            correlation_id="\t",
            trace_id="00000000000000000000000000000000",
            span_id="XYZ",
        )
        self.assertIsNone(result.call_context)
        self.assertIsInstance(result.error, CallContextFieldInvalidError)
        self.assertEqual(error_code_of(result.error), "event_schema_invalid")
        message = str(result.error)
        positions = [
            message.index(name + ":")
            for name in ("application", "instance_id", "correlation_id", "trace_id", "span_id")
        ]
        self.assertEqual(positions, sorted(positions))
        self.assertNotIn("execution_id", message)
        self.assertIn("all zeros", message)

    def test_no_value_appears_in_a_message(self):
        for field_name in ("application", "instance_id", "execution_id", "correlation_id", "trace_id", "span_id"):
            with self.subTest(field=field_name):
                result = new_call_context(**{field_name: f"{SECRET}\n"})
                self.assertIsNotNone(result.error)
                self.assertNotIn(SECRET, str(result.error))
                self.assertIn(field_name, str(result.error))

    def test_all_zeros_error_is_classified(self):
        self.assertEqual(error_code_of(CallContextIdAllZerosError("x")), "event_schema_invalid")

    def test_the_context_is_immutable(self):
        call_context = new_call_context(application="app").call_context
        with self.assertRaises(dataclasses.FrozenInstanceError):
            call_context.application = "other"


class CorrelationMappingTest(unittest.TestCase):
    def test_a_context_without_ids_gives_no_correlation_context(self):
        self.assertEqual(to_correlation_context(CallContext(application="app", instance_id="i")), (None, None))

    def test_only_the_set_ids_are_mapped(self):
        context, error = to_correlation_context(CallContext(execution_id="run", indentation=4))
        self.assertIsNone(error)
        self.assertEqual(
            (context.execution_id, context.trace_id, context.span_id, context.correlation_id), ("run", None, None, None)
        )

    def test_a_field_that_was_not_validated_is_returned_as_an_error_not_raised(self):
        context, error = to_correlation_context(CallContext(trace_id="not-hex"))
        self.assertIsNone(context)
        self.assertEqual(error_code_of(error), "event_schema_invalid")
        self.assertNotIn("not-hex", str(error))


if __name__ == "__main__":
    unittest.main()
