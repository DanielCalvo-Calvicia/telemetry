"""Black-box tests of LimitsService."""

import copy
import threading
import unittest
from unittest import mock

from internal.tests.domain.operations.limits.helpers import make_envelope
from telemetry.internal.application.errors import EnvelopeLimitsFailedError
from telemetry.internal.application.services import limits_service
from telemetry.internal.application.services.limits_service import LimitsService
from telemetry.internal.domain.errors import EventTooLargeError
from telemetry.internal.domain.operations.limits.report import LimitsReport
from telemetry.internal.domain.value_objects.limits import Limits


class LimitsServiceTest(unittest.TestCase):
    def test_a_limited_copy_comes_back_with_the_counters(self):
        envelope = make_envelope({"k": "v" * 300})
        before = copy.deepcopy(envelope)
        result = LimitsService(Limits()).apply(envelope)
        self.assertIsNone(result.error)
        self.assertEqual(len(result.envelope.attributes["k"]), 256)
        self.assertEqual(result.report.attribute_values_truncated, 1)
        self.assertEqual(envelope, before)

    def test_a_too_large_event_is_returned_not_raised(self):
        result = LimitsService(Limits(max_event_bytes=10, max_batch_bytes=10)).apply(make_envelope())
        self.assertIsNone(result.envelope)
        self.assertIsInstance(result.error, EventTooLargeError)
        self.assertEqual(result.report.events_rejected, 1)

    def test_an_unexpected_failure_is_returned_not_raised(self):
        cause = RuntimeError("the secret of the event")
        with mock.patch.object(limits_service, "apply_limits", side_effect=cause):
            result = LimitsService(Limits()).apply(make_envelope())
        self.assertIsNone(result.envelope)
        self.assertIsInstance(result.error, EnvelopeLimitsFailedError)
        self.assertIs(result.error.__cause__, cause)
        self.assertEqual(result.report, LimitsReport())
        self.assertNotIn("secret", str(result.error))

    def test_a_key_with_a_lone_surrogate_does_not_raise(self):
        result = LimitsService(Limits()).apply(make_envelope({"a\ud800": 1}))
        self.assertIsInstance(result.error, EnvelopeLimitsFailedError)

    def test_threads(self):
        service = LimitsService(Limits())
        results = []

        def work():
            for _ in range(50):
                results.append(service.apply(make_envelope({"k": "v" * 300})).report.attribute_values_truncated)

        threads = [threading.Thread(target=work) for _ in range(8)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join()
        self.assertEqual(results, [1] * 400)
