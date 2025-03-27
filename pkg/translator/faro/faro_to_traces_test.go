// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faro // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"

import (
	"context"
	"path/filepath"
	"testing"

	faroTypes "github.com/grafana/faro/pkg/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
)

func TestTranslateToTraces(t *testing.T) {
	testcases := []struct {
		name               string
		faroPayload        faroTypes.Payload
		expectedTracesFile string
		wantErr            assert.ErrorAssertionFunc
	}{
		{
			name:               "Standard payload",
			faroPayload:        PayloadFromFile(t, "standard-payload/payload.json"),
			expectedTracesFile: filepath.Join("testdata", "standard-payload", "ptraces.yaml"),
			wantErr:            assert.NoError,
		},
		{
			name:               "Empty payload",
			faroPayload:        faroTypes.Payload{},
			expectedTracesFile: filepath.Join("testdata", "empty-payload", "ptraces.yaml"),
			wantErr:            assert.NoError,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			actualTraces, err := TranslateToTraces(context.TODO(), tt.faroPayload)
			if !tt.wantErr(t, err) {
				return
			}
			expectedTraces, err := golden.ReadTraces(tt.expectedTracesFile)
			require.NoError(t, err)
			require.NoError(t, ptracetest.CompareTraces(expectedTraces, actualTraces))
		})
	}
}
