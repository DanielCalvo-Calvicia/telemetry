"""The AWS HTTP destination: ``log.*`` events sent to the add_log endpoint of aws_microservice (T-04.6, ADR-008).

    import telemetry
    from telemetry.logging import aws_http

    settings = aws_http.Settings(
        host="http://localhost", port="8080", path="/api/v1/cloudwatch/logs/add",
        log_group_name="my-log-group", log_stream_name="my-log-stream",
    )
    core = telemetry.new(telemetry.microservice(), telemetry.with_log_sink(aws_http.new_destination(settings)))

The host, the port, the path, the log group and the log stream are required: without them
``telemetry.ConfigurationError`` is raised when the destination is built, and it names every field. The library reads
no environment (ADR-004): the consumer loads the values and passes them in. The destination sends ONE request per
``log.*`` event, skips (and counts as accepted) the events of any other type, does not queue and does not retry on its
own; a failure is in the ``SendResult`` and never an exception.
"""

from telemetry.internal.composition_root.dependencies.aws_http_destination_dependencies import (
    new_aws_http_destination,
)
from telemetry.internal.infrastructure.outbound.logging.aws_http.aws_http_destination import (
    AwsHttpDestination,
    Delivery,
)
from telemetry.internal.infrastructure.outbound.logging.aws_http.settings import Settings

Destination = AwsHttpDestination
"""The AWS HTTP destination: it serves the log and the event capability; ``deliver`` also gives the outcome."""

__all__ = ["Delivery", "Destination", "Settings", "default_settings", "new_destination"]


def default_settings() -> Settings:
    """Return the default settings: no host, port, path, log group or log stream (the consumer must set them)."""
    return Settings()


def new_destination(settings: Settings) -> Destination:
    """Create the AWS HTTP destination; invalid settings raise ConfigurationError with every problem."""
    return new_aws_http_destination(settings)
