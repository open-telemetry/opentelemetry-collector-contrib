// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

func TestOpenSearchExporter(t *testing.T) {
	// Create HTTP listener
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		decoder := json.NewDecoder(r.Body)
		for decoder.More() {
			var traceData any
			err = decoder.Decode(&traceData)
			require.NoError(t, err)
			require.NotNil(t, traceData)
		}
	}))
	defer ts.Close()

	// Create exporter
	f := NewFactory()
	cfg := withDefaultConfig(func(config *Config) {
		config.Endpoint = ts.URL
	})
	exporter, err := f.CreateTracesExporter(context.Background(), exportertest.NewNopCreateSettings(), cfg)
	require.NoError(t, err)

	// Initialize the exporter
	err = exporter.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// Load sample data
	dp, err := testbed.NewFileDataProvider("testdata/traces.yaml", component.DataTypeTraces)
	require.NoError(t, err)
	dataItemsGenerated := atomic.Uint64{}
	dp.SetLoadGeneratorCounters(&dataItemsGenerated)
	traces, _ := dp.GenerateTraces()

	// Send it
	err = exporter.ConsumeTraces(context.Background(), traces)
	require.NoError(t, err)
}
