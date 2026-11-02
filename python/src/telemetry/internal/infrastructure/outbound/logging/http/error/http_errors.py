"""The fixed reasons the HTTP destination itself gives (the shared client has its own: ``Outcome.reason``).

A reason is a fixed text: it never holds a configured value (a URL or a secret) or the text of an exception.
"""

REASON_DESTINATION_FAILED = "destination_failed"
