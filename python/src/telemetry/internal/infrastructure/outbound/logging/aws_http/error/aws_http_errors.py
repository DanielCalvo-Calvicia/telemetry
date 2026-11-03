"""The fixed reasons the AWS HTTP adapter itself gives (the shared client has its own: ``Outcome.reason``).

A reason is a fixed text: it never holds a configured value (a host or a secret) or the text of an exception.
"""

REASON_DESTINATION_FAILED = "destination_failed"
REASON_SERIALIZATION_FAILED = "serialization_failed"
