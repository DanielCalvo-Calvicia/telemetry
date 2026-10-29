package fakes

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// fake_spool_entry is a stored record and whether it was acknowledged.
type fake_spool_entry struct {
	record       dtos.SpoolRecord
	size         int64
	acknowledged bool
}

// FakeLocalSpool is an in-memory fake of outbound.LocalSpoolPort. Append stores events with the ids record-1,
// record-2 and so on; ReadBatch returns the ones not acknowledged; Acknowledge marks them; Cleanup removes the
// acknowledged ones. FailWith makes the methods that return an error return it.
type FakeLocalSpool struct {
	mutex         sync.Mutex
	entries       []fake_spool_entry
	next_id       int
	quota_bytes   int64
	failure       error
	append_result dtos.SendResult
	append_set    bool
}

// SetQuotaBytes sets the quota that Usage reports.
func (fake_spool *FakeLocalSpool) SetQuotaBytes(quota_bytes int64) {
	fake_spool.mutex.Lock()
	defer fake_spool.mutex.Unlock()
	fake_spool.quota_bytes = quota_bytes
}

// FailWith makes ReadBatch, Acknowledge, Usage and Cleanup return the error. A nil error clears it.
func (fake_spool *FakeLocalSpool) FailWith(failure error) {
	fake_spool.mutex.Lock()
	defer fake_spool.mutex.Unlock()
	fake_spool.failure = failure
}

// SetAppendResult makes Append return the result and store nothing.
func (fake_spool *FakeLocalSpool) SetAppendResult(result dtos.SendResult) {
	fake_spool.mutex.Lock()
	defer fake_spool.mutex.Unlock()
	fake_spool.append_result = result
	fake_spool.append_set = true
}

// Append stores the events and reports them all accepted, unless SetAppendResult was called.
func (fake_spool *FakeLocalSpool) Append(_ context.Context, events []value_objects.Envelope) dtos.SendResult {
	fake_spool.mutex.Lock()
	defer fake_spool.mutex.Unlock()
	if fake_spool.append_set {
		return fake_spool.append_result
	}
	for _, event := range events {
		fake_spool.next_id++
		encoded_event, _ := json.Marshal(event)
		fake_spool.entries = append(fake_spool.entries, fake_spool_entry{
			record: dtos.SpoolRecord{ID: fmt.Sprintf("record-%d", fake_spool.next_id), Event: event},
			size:   int64(len(encoded_event)),
		})
	}
	return dtos.SendResult{Accepted: len(events)}
}

// ReadBatch returns up to maximum_events records that are not acknowledged, oldest first.
func (fake_spool *FakeLocalSpool) ReadBatch(_ context.Context, maximum_events int) ([]dtos.SpoolRecord, error) {
	fake_spool.mutex.Lock()
	defer fake_spool.mutex.Unlock()
	if fake_spool.failure != nil {
		return nil, fake_spool.failure
	}
	records := []dtos.SpoolRecord{}
	for _, entry := range fake_spool.entries {
		if len(records) >= maximum_events {
			break
		}
		if !entry.acknowledged {
			records = append(records, entry.record)
		}
	}
	return records, nil
}

// Acknowledge marks the records with the ids as acknowledged. An unknown id is ignored.
func (fake_spool *FakeLocalSpool) Acknowledge(_ context.Context, record_ids []string) error {
	fake_spool.mutex.Lock()
	defer fake_spool.mutex.Unlock()
	if fake_spool.failure != nil {
		return fake_spool.failure
	}
	for position := range fake_spool.entries {
		if slices.Contains(record_ids, fake_spool.entries[position].record.ID) {
			fake_spool.entries[position].acknowledged = true
		}
	}
	return nil
}

// Usage reports the events and bytes that are stored, acknowledged or not, and the quota.
func (fake_spool *FakeLocalSpool) Usage(_ context.Context) (dtos.SpoolUsage, error) {
	fake_spool.mutex.Lock()
	defer fake_spool.mutex.Unlock()
	if fake_spool.failure != nil {
		return dtos.SpoolUsage{}, fake_spool.failure
	}
	usage := dtos.SpoolUsage{Events: len(fake_spool.entries), QuotaBytes: fake_spool.quota_bytes}
	for _, entry := range fake_spool.entries {
		usage.Bytes += entry.size
	}
	return usage, nil
}

// Cleanup removes the acknowledged records and reports what it removed.
func (fake_spool *FakeLocalSpool) Cleanup(_ context.Context) (dtos.SpoolCleanup, error) {
	fake_spool.mutex.Lock()
	defer fake_spool.mutex.Unlock()
	if fake_spool.failure != nil {
		return dtos.SpoolCleanup{}, fake_spool.failure
	}
	cleanup := dtos.SpoolCleanup{}
	kept_entries := fake_spool.entries[:0]
	for _, entry := range fake_spool.entries {
		if entry.acknowledged {
			cleanup.RemovedEvents++
			cleanup.FreedBytes += entry.size
			continue
		}
		kept_entries = append(kept_entries, entry)
	}
	fake_spool.entries = kept_entries
	return cleanup, nil
}
