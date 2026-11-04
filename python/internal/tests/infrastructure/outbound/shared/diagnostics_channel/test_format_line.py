"""Black-box tests of the line of the writer of the diagnostics channel."""

import unittest

from telemetry.internal.application.dtos.diagnostic_event import DiagnosticEvent
from telemetry.internal.domain.value_objects.enums_generated import DiagnosticCode
from telemetry.internal.infrastructure.outbound.shared.diagnostics_channel.helpers.format_line import format_line


class FormatLineTest(unittest.TestCase):
    def test_the_line_has_the_exact_format_and_ends_in_a_line_feed(self):
        event = DiagnosticEvent(DiagnosticCode.EVENTS_DROPPED, "log", 2, "queue_full_newest", 4, suppressed=7, at_ms=1)
        self.assertEqual(
            format_line(event),
            "telemetry diagnostics: code=events_dropped destination=log/2 "
            "reason=queue_full_newest count=4 suppressed=7\n",
        )


if __name__ == "__main__":
    unittest.main()
