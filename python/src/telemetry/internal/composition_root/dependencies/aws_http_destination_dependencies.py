"""The dependencies of the AWS HTTP destination."""

from telemetry.internal.domain.value_objects.limits import Limits
from telemetry.internal.infrastructure.outbound.logging.aws_http.aws_http_destination import AwsHttpDestination
from telemetry.internal.infrastructure.outbound.logging.aws_http.settings import Settings


def new_aws_http_destination(settings: Settings, limits: Limits = Limits()) -> AwsHttpDestination:
    """Create the AWS HTTP destination over the shared client; invalid settings raise ConfigurationError."""
    return AwsHttpDestination(settings, limits)
