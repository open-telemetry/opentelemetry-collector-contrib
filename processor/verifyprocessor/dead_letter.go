// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package verifyprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/verifyprocessor"

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

const deadLetterStorageClient = "dead_letter"

var (
	errDeadLetterClient   = errors.New("dead letter storage client is not started")
	errDeadLetterTooLarge = errors.New("dead letter entry exceeds max_entry_size_bytes")
)

type deadLetterEntry struct {
	Reason        string          `json:"reason"`
	Error         string          `json:"error,omitempty"`
	FailureMode   string          `json:"failure_mode"`
	Profile       string          `json:"profile"`
	LogID         string          `json:"log_id,omitempty"`
	ResourceIndex int             `json:"resource_index"`
	ScopeIndex    int             `json:"scope_index"`
	RecordIndex   int             `json:"record_index"`
	Algorithm     string          `json:"algorithm,omitempty"`
	RecordedAt    string          `json:"recorded_at"`
	Truncated     bool            `json:"truncated,omitempty"`
	Logs          json.RawMessage `json:"logs,omitempty"`
}

func deadLetterStorageName(pipelineID pipeline.ID) string {
	if pipelineID.String() == "" {
		return deadLetterStorageClient
	}
	return deadLetterStorageClient + "-" + pipelineID.String()
}

func getStorageClient(ctx context.Context, host component.Host, storageID component.ID, settings processor.Settings) (storage.Client, error) {
	if host == nil {
		return nil, fmt.Errorf("storage extension %q not found", storageID)
	}
	ext, ok := host.GetExtensions()[storageID]
	if !ok {
		return nil, fmt.Errorf("storage extension %q not found", storageID)
	}
	storageExt, ok := ext.(storage.Extension)
	if !ok {
		return nil, fmt.Errorf("extension %q is not a storage extension", storageID)
	}
	return storageExt.GetClient(ctx, component.KindProcessor, settings.ID, deadLetterStorageName(settings.PipelineID))
}

func (dl DeadLetterConfig) includeRecord() bool {
	if dl.IncludeRecord == nil {
		return true
	}
	return *dl.IncludeRecord
}

func (dl DeadLetterConfig) includeResource() bool {
	if dl.IncludeResource == nil {
		return false
	}
	return *dl.IncludeResource
}

func (dl DeadLetterConfig) failOnStorageError() bool {
	if dl.FailOnStorageError == nil {
		return false
	}
	return *dl.FailOnStorageError
}

func (dl DeadLetterConfig) matchesFailure(mode, reason string) bool {
	if len(dl.FailureModes) > 0 && !containsString(dl.FailureModes, mode) {
		return false
	}
	if len(dl.Reasons) > 0 && !containsString(dl.Reasons, reason) {
		return false
	}
	return true
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func (p *verifyProcessor) writeDeadLetter(
	ctx context.Context,
	rl plog.ResourceLogs,
	sl plog.ScopeLogs,
	lr plog.LogRecord,
	resourceIdx, scopeIdx, recordIdx int,
	algorithm, logID string,
	fail *verifyFailure,
) error {
	dl := p.config.DeadLetter
	if !dl.Enabled || !dl.matchesFailure(p.config.FailureMode, fail.reason) {
		return nil
	}
	if p.storageClient == nil {
		return errDeadLetterClient
	}
	now := time.Now().UTC()
	payload, err := buildDeadLetterPayload(dl, p.config.FailureMode, p.config.VerificationProfile, rl, sl, lr, resourceIdx, scopeIdx, recordIdx, algorithm, logID, fail, now)
	if err != nil {
		return err
	}
	seq := p.dlqSeq.Add(1)
	key := fmt.Sprintf("%s%s-%020d-%08d", dl.KeyPrefix, p.dlqInstance, now.UnixNano(), seq)
	return p.storageClient.Set(ctx, key, payload)
}

// deadLetterWriteErrors collects dead letter write failures for one batch so
// they are logged once instead of once per record.
type deadLetterWriteErrors struct {
	count       int
	first       error
	firstReason string
	tooLarge    error
}

func (d *deadLetterWriteErrors) add(reason string, err error) {
	d.count++
	if d.first == nil {
		d.first = err
		d.firstReason = reason
	}
	if errors.Is(err, errDeadLetterTooLarge) {
		d.tooLarge = errors.Join(d.tooLarge, err)
	}
}

func (d *deadLetterWriteErrors) log(logger *zap.Logger) {
	if d.count == 0 {
		return
	}
	logger.Error("failed to write verification failures to dead letter storage",
		zap.Int("failed_entries", d.count),
		zap.String("first_reason", d.firstReason),
		zap.Error(d.first),
	)
}

func newDeadLetterInstance() (string, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func deadLetterEntryTooLarge(size, maxBytes int) error {
	return fmt.Errorf("%w: entry is %d bytes, max_entry_size_bytes is %d", errDeadLetterTooLarge, size, maxBytes)
}

func buildDeadLetterPayload(
	dl DeadLetterConfig,
	failureMode, profile string,
	rl plog.ResourceLogs,
	sl plog.ScopeLogs,
	lr plog.LogRecord,
	resourceIdx, scopeIdx, recordIdx int,
	algorithm, logID string,
	fail *verifyFailure,
	recordedAt time.Time,
) ([]byte, error) {
	maxBytes := dl.MaxEntrySizeBytes
	entry := deadLetterEntry{
		Reason:        fail.reason,
		FailureMode:   failureMode,
		Profile:       profile,
		LogID:         logID,
		ResourceIndex: resourceIdx,
		ScopeIndex:    scopeIdx,
		RecordIndex:   recordIdx,
		Algorithm:     algorithm,
		RecordedAt:    recordedAt.Format(time.RFC3339Nano),
	}
	if fail.err != nil {
		entry.Error = fail.err.Error()
	}
	includeRecord := dl.includeRecord()
	includeResource := dl.includeResource()
	if includeRecord || includeResource {
		logsJSON, err := snapshotLogs(rl, sl, lr, includeResource, includeRecord)
		if err != nil {
			return nil, fmt.Errorf("marshal dead letter logs: %w", err)
		}
		entry.Logs = logsJSON
	}
	payload, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("marshal dead letter entry: %w", err)
	}
	if len(payload) <= maxBytes {
		return payload, nil
	}
	if len(entry.Logs) == 0 {
		return nil, deadLetterEntryTooLarge(len(payload), maxBytes)
	}
	entry.Logs = nil
	entry.Truncated = true
	payload, err = json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("marshal dead letter entry: %w", err)
	}
	if len(payload) > maxBytes {
		return nil, deadLetterEntryTooLarge(len(payload), maxBytes)
	}
	return payload, nil
}

func snapshotLogs(rl plog.ResourceLogs, sl plog.ScopeLogs, lr plog.LogRecord, includeResource, includeRecord bool) ([]byte, error) {
	out := plog.NewLogs()
	dstRL := out.ResourceLogs().AppendEmpty()
	if includeResource {
		rl.Resource().CopyTo(dstRL.Resource())
		dstRL.SetSchemaUrl(rl.SchemaUrl())
	}
	if includeRecord {
		dstSL := dstRL.ScopeLogs().AppendEmpty()
		sl.Scope().CopyTo(dstSL.Scope())
		dstSL.SetSchemaUrl(sl.SchemaUrl())
		lr.CopyTo(dstSL.LogRecords().AppendEmpty())
	}
	return (&plog.JSONMarshaler{}).MarshalLogs(out)
}
