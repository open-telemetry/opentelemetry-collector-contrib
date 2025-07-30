// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func newTestExtension(t *testing.T, cfg Config) *ext {
	extension := newExtension(&cfg)
	err := extension.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	t.Cleanup(func() {
		err = extension.Shutdown(context.Background())
		require.NoError(t, err)
	})
	return extension
}

func readFile(t *testing.T, filename string) []byte {
	data, err := os.ReadFile(filename)
	require.NoError(t, err)
	return data
}

func TestUnmarshalLogs(t *testing.T) {
	tests := []struct {
		name       string
		input      []byte
		wantFile   string
		setFlag    bool // workaround for the flag
		expectsErr string
		cfg        Config
	}{
		{
			name:     "real_log",
			input:    readFile(t, "testdata/real_log.json"),
			wantFile: "testdata/real_log_expected.yaml",
			setFlag:  true,
			cfg:      *createDefaultConfig().(*Config),
		},
		{
			name: "invalid_span",
			input: func() []byte {
				data := readFile(t, "testdata/real_log.json")
				return bytes.Replace(data, []byte(`"spanId":"3e3a5741b18f0710"`), []byte(`"spanId":"13210305202245662348"`), 1)
			}(),
			expectsErr: "failed to handle log entry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			extension := newTestExtension(t, tt.cfg)

			logs, err := extension.UnmarshalLogs(tt.input)

			if tt.expectsErr != "" {
				require.ErrorContains(t, err, tt.expectsErr)
				return
			}

			require.NoError(t, err)

			want, err := golden.ReadLogs(tt.wantFile)
			require.NoError(t, err)

			var flags plog.LogRecordFlags
			want.ResourceLogs().At(0).
				ScopeLogs().At(0).
				LogRecords().At(0).
				SetFlags(flags.WithIsSampled(tt.setFlag))

			require.NoError(t,
				plogtest.CompareLogs(logs, want,
					plogtest.IgnoreObservedTimestamp(),
				),
			)
		})
	}
}
