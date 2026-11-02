"""The names under which a consumer can load each setting of the file destination from its own source.

The keys are the property names of contracts/config/file.schema.json, in its order, and the values are its ``x-env``
names. There are NO default values here on purpose (the defaults live in settings.py) and the table is read only. This
package never reads them (ADR-004).
"""

from types import MappingProxyType
from typing import Mapping

FILE_ENVIRONMENT_VARIABLES: Mapping[str, str] = MappingProxyType(
    {
        "directory": "TELEMETRY_FILE_DIRECTORY",
        "file_name": "TELEMETRY_FILE_NAME",
        "minimum_severity": "TELEMETRY_FILE_MINIMUM_SEVERITY",
        "max_file_bytes": "TELEMETRY_FILE_MAX_FILE_BYTES",
        "max_files": "TELEMETRY_FILE_MAX_FILES",
        "quota_bytes": "TELEMETRY_FILE_QUOTA_BYTES",
    }
)
