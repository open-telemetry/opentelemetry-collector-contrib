// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutil"

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatadogLogsServer(t *testing.T) {
	server := DatadogLogServerMock()
	values := JSONLogs{
		{
			"company":   "datadog",
			"component": "logs",
		},
	}
	jsonBytes, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
		return
	}
	var buf = bytes.NewBuffer([]byte{})
	w := gzip.NewWriter(buf)
	_, _ = w.Write(jsonBytes)
	_ = w.Close()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL, buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer resp.Body.Close()
	assert.Equal(t, 202, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
		return
	}
	assert.Equal(t, []byte(`{"status":"ok"}`), body)
	assert.Equal(t, values, server.LogsData)

}
