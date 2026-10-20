package helpers

import (
	"encoding/json"

	dtos "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/application/dtos"
)

// addLogRequest is the wire format of the add_log endpoint of aws_microservice. The timestamp is in milliseconds
// since 1970-01-01 UTC.
type addLogRequest struct {
	LogGroupName  string `json:"log_group_name"`
	LogStreamName string `json:"log_stream_name"`
	Message       string `json:"message"`
	Timestamp     int64  `json:"timestamp"`
}

// BuildAddLogRequest adapts an entry to the JSON body of the add_log endpoint. The message of the CloudWatch event is
// the whole entry encoded as the flat JSON payload of the logger, so its level, category, fields and identifiers
// are kept; the timestamp is the one of the entry in milliseconds.
func BuildAddLogRequest(log_entry dtos.LogEntryOutboundDTO, log_group_name string, log_stream_name string) ([]byte, error) {
	encoded_entry, err := json.Marshal(log_entry)
	if err != nil {
		return nil, err
	}

	return json.Marshal(addLogRequest{
		LogGroupName:  log_group_name,
		LogStreamName: log_stream_name,
		Message:       string(encoded_entry),
		Timestamp:     log_entry.Timestamp.UnixMilli(),
	})
}
