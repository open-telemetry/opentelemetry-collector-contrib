// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faro // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	faroTypes "github.com/grafana/faro/pkg/go"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranslateToLogs(t *testing.T) {
	testcases := []struct {
		name             string
		faroPayload      faroTypes.Payload
		expectedLogsFile string
		wantErr          assert.ErrorAssertionFunc
	}{
		{
			name:             "Empty payload",
			faroPayload:      faroTypes.Payload{},
			expectedLogsFile: filepath.Join("testdata", "empty-payload", "plogs.yaml"),
			wantErr:          assert.NoError,
		},
		{
			name:             "Standard payload",
			faroPayload:      PayloadFromFile(t, "standard-payload/payload.json"),
			expectedLogsFile: filepath.Join("testdata", "standard-payload", "plogs.yaml"),
			wantErr:          assert.NoError,
		},
		{
			name:             "Payload with browser brands as slice",
			faroPayload:      PayloadFromFile(t, "browser-brand-slice-payload/payload.json"),
			expectedLogsFile: filepath.Join("testdata", "browser-brand-slice-payload", "plogs.yaml"),
			wantErr:          assert.NoError,
		},
		{
			name:             "Payload with browser brands as string",
			faroPayload:      PayloadFromFile(t, "browser-brand-string-payload/payload.json"),
			expectedLogsFile: filepath.Join("testdata", "browser-brand-string-payload", "plogs.yaml"),
			wantErr:          assert.NoError,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			actualLogs, err := TranslateToLogs(context.TODO(), tt.faroPayload)
			if !tt.wantErr(t, err) {
				return
			}
			expectedLogs, err := golden.ReadLogs(tt.expectedLogsFile)
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expectedLogs, actualLogs))
		})
	}
}

func PayloadFromFile(t *testing.T, filename string) faroTypes.Payload {
	t.Helper()

	f, err := os.Open(filepath.Join("testdata", filename))
	if err != nil {
		t.Fatal(err)
	}

	defer f.Close()

	var p faroTypes.Payload
	if err := json.NewDecoder(f).Decode(&p); err != nil {
		t.Fatal(err)
	}

	return p
}
