// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/metadata"
)

// TestLogConventionsFeatureGates verifies which semconv error attributes the
// messaging unmarshaler emits for each combination of the DontEmitV0/EmitV1 log
// convention feature gates. The v0 attribute is error.message; the v1 attribute
// is exception.message.
func TestLogConventionsFeatureGates(t *testing.T) {
	// not parallel: mutates the global featuregate registry
	data, err := os.ReadFile(filepath.Join(testFilesDirectory, "messaging", "diagnostic-error-log.json"))
	require.NoError(t, err)

	const wantMessage = "Entity not found"

	tests := []struct {
		name       string
		emitV1     bool
		dontEmitV0 bool
		wantV0     bool // error.message
		wantV1     bool // exception.message
	}{
		{
			name:       "default (beta): only v1",
			emitV1:     true,
			dontEmitV0: true,
			wantV0:     false,
			wantV1:     true,
		},
		{
			name:       "legacy: only v0",
			emitV1:     false,
			dontEmitV0: false,
			wantV0:     true,
			wantV1:     false,
		},
		{
			name:       "migration: both v0 and v1",
			emitV1:     true,
			dontEmitV0: false,
			wantV0:     true,
			wantV1:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := featuregate.GlobalRegistry()
			require.NoError(t, registry.Set(metadata.ExtensionAzureencodingEmitV1LogConventionsFeatureGate.ID(), tt.emitV1))
			require.NoError(t, registry.Set(metadata.ExtensionAzureencodingDontEmitV0LogConventionsFeatureGate.ID(), tt.dontEmitV0))
			t.Cleanup(func() {
				require.NoError(t, registry.Set(metadata.ExtensionAzureencodingEmitV1LogConventionsFeatureGate.ID(), true))
				require.NoError(t, registry.Set(metadata.ExtensionAzureencodingDontEmitV0LogConventionsFeatureGate.ID(), true))
			})

			unmarshaler := NewAzureResourceLogsUnmarshaler(testBuildInfo, zap.NewNop(), LogsConfig{
				TimeFormats: []string{
					"01/02/2006 15:04:05",
					"2006-01-02T15:04:05Z",
					"1/2/2006 3:04:05.000 PM -07:00",
					"1/2/2006 3:04:05 PM -07:00",
				},
			})

			logs, err := unmarshaler.UnmarshalLogs(data)
			require.NoError(t, err)

			v0, hasV0 := findLogAttr(logs, "error.message")
			v1, hasV1 := findLogAttr(logs, "exception.message")

			require.Equal(t, tt.wantV0, hasV0, "error.message presence")
			require.Equal(t, tt.wantV1, hasV1, "exception.message presence")
			if tt.wantV0 {
				require.Equal(t, wantMessage, v0.Str())
			}
			if tt.wantV1 {
				require.Equal(t, wantMessage, v1.Str())
			}
		})
	}
}

// findLogAttr returns the first log record attribute matching key across all
// resource logs, scope logs, and log records.
func findLogAttr(logs plog.Logs, key string) (pcommon.Value, bool) {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		scopeLogs := logs.ResourceLogs().At(i).ScopeLogs()
		for j := 0; j < scopeLogs.Len(); j++ {
			records := scopeLogs.At(j).LogRecords()
			for k := 0; k < records.Len(); k++ {
				if v, ok := records.At(k).Attributes().Get(key); ok {
					return v, true
				}
			}
		}
	}
	return pcommon.Value{}, false
}
