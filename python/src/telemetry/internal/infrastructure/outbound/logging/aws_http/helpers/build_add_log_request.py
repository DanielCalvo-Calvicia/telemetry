"""The body of one add_log request for one ``log.*`` event: the same bytes in Go and Python (pure)."""

import re

from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.domain.value_objects.envelope_generated import Envelope
from telemetry.internal.infrastructure.outbound.logging.aws_http.helpers.map_level import map_level
from telemetry.internal.infrastructure.outbound.logging.aws_http.helpers.timestamp_milliseconds import (
    timestamp_milliseconds,
)

_LONE_SURROGATE = re.compile("[\ud800-\udfff]")


def _text(value: object) -> str:
    """The JSON string of a text: UTF-8 kept, control characters as \\u00xx, no HTML escaping, a lone surrogate U+FFFD.

    A value that is not a text is an empty text (a ``data`` key that is missing or of another type).
    """
    clean = _LONE_SURROGATE.sub("�", value if isinstance(value, str) else "")
    return canonical_json(clean).decode("utf-8")


def build_add_log_message(event: Envelope) -> str:
    """Return the entry the endpoint stores: the keys of the old LogEntryOutboundDTO, in the order of that struct.

    The application comes from ``resource.service_name`` and the application id from ``resource.instance_id``; the
    process id is the correlation id or empty; category, function and message come from ``data`` (empty when missing);
    indentation and depth are 0 (an event has no call tree); the level is the severity with CRITICAL as FATAL; the
    fields are the canonical JSON of the attributes, or null when there are none; the timestamp is ``occurred_at``.
    """
    correlation_id = event.context.correlation_id if event.context is not None else None
    fields = canonical_json(event.attributes).decode("utf-8") if event.attributes else "null"
    data = event.data
    return (
        f'{{"ProducerID":{_text(event.resource.service_name)}'
        f',"ApplicationID":{_text(event.resource.instance_id)}'
        f',"ProcessID":{_text(correlation_id)}'
        f',"Application":{_text(event.resource.service_name)}'
        f',"Category":{_text(data.get("category"))}'
        f',"Function":{_text(data.get("function"))}'
        ',"Indentation":0,"Depth":0'
        f',"Level":{_text(map_level(event.severity))}'
        f',"Message":{_text(data.get("message"))}'
        f',"Fields":{fields}'
        f',"Timestamp":{_text(event.occurred_at)}}}'
    )


def build_add_log_request(event: Envelope, log_group_name: str, log_stream_name: str) -> bytes:
    """Return the JSON body of the add_log request: group, stream, the message text and the timestamp in milliseconds.

    It raises when the event has no such form (a lone surrogate or a number with no canonical form in the attributes,
    an ``occurred_at`` that is not a UTC time); the caller treats that as a permanent failure of the event.
    """
    body = (
        f'{{"log_group_name":{_text(log_group_name)}'
        f',"log_stream_name":{_text(log_stream_name)}'
        f',"message":{_text(build_add_log_message(event))}'
        f',"timestamp":{timestamp_milliseconds(event.occurred_at)}}}'
    )
    return body.encode("utf-8")
