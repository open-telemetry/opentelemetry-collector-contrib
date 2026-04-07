// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eventhub

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/trigger"
)

type fakeUnmarshaler struct {
	logs plog.Logs
	err  error
}

func (f *fakeUnmarshaler) UnmarshalLogs(_ []byte) (plog.Logs, error) {
	if f.err != nil {
		return plog.Logs{}, f.err
	}
	return f.logs, nil
}

// makeLogs creates plog.Logs with one resource and one log record.
func makeLogs() plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	sl.LogRecords().AppendEmpty()
	return logs
}

func TestLogsConsumer_ConsumeEvents(t *testing.T) {
	tests := map[string]struct {
		content        [][]byte
		metadata       map[string]string
		unmarshaler    plog.Unmarshaler
		wantErr        string
		wantLogBatches int
		wantLogRecords int
		wantMetadata   map[string]string
	}{
		"success_single_message": {
			content:        [][]byte{[]byte("msg1")},
			unmarshaler:    &fakeUnmarshaler{logs: makeLogs()},
			wantLogBatches: 1,
			wantLogRecords: 1,
		},
		"success_multiple_messages_merged": {
			content:        [][]byte{[]byte("a"), []byte("b")},
			unmarshaler:    &fakeUnmarshaler{logs: makeLogs()},
			wantLogBatches: 1,
			wantLogRecords: 2,
		},
		"metadata_applied_to_resource_attributes": {
			content: [][]byte{[]byte("msg")},
			metadata: map[string]string{
				AttrEventHubName:          "myhub",
				AttrEventHubPartitionID:   "1",
				AttrEventHubNamespace:     "test-namespace.servicebus.windows.net",
				AttrEventHubConsumerGroup: "test",
			},
			unmarshaler:    &fakeUnmarshaler{logs: makeLogs()},
			wantLogBatches: 1,
			wantLogRecords: 1,
			wantMetadata: map[string]string{
				AttrEventHubName:          "myhub",
				AttrEventHubPartitionID:   "1",
				AttrEventHubNamespace:     "test-namespace.servicebus.windows.net",
				AttrEventHubConsumerGroup: "test",
			},
		},
		"unmarshal_error": {
			content:        [][]byte{[]byte("bad")},
			unmarshaler:    &fakeUnmarshaler{err: errors.New("unmarshal failed")},
			wantErr:        "unmarshal message 0",
			wantLogBatches: 0,
		},
		"no_logs_to_consume_empty_content": {
			content:        [][]byte{},
			unmarshaler:    &fakeUnmarshaler{logs: makeLogs()},
			wantErr:        "no logs to consume",
			wantLogBatches: 0,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			sink := new(consumertest.LogsSink)
			consumer := NewLogsConsumer(tt.unmarshaler, sink)

			req := trigger.ParsedRequest{
				Content:  tt.content,
				Metadata: tt.metadata,
			}
			err := consumer.ConsumeEvents(t.Context(), req)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Len(t, sink.AllLogs(), tt.wantLogBatches)
				return
			}
			require.NoError(t, err)
			all := sink.AllLogs()
			require.Len(t, all, tt.wantLogBatches, "log batches")
			if tt.wantLogBatches > 0 {
				assert.Equal(t, tt.wantLogRecords, all[0].LogRecordCount(), "log records in first batch")
			}
			if len(tt.wantMetadata) > 0 && tt.wantLogBatches > 0 {
				resource := all[0].ResourceLogs().At(0).Resource()
				for key, wantVal := range tt.wantMetadata {
					attr, ok := resource.Attributes().Get(key)
					require.True(t, ok, "attribute %q", key)
					assert.Equal(t, wantVal, attr.Str(), "attribute %q", key)
				}
			}
		})
	}
}
