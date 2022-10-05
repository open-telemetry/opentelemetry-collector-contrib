// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testutils // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutils"

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDatadogLogsServer(t *testing.T) {
	server := DatadogLogServerMock()
	values := []map[string]interface{}{
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
	resp, err := http.Post(server.URL, "application/json", buf)
	if err != nil {
		t.Fatal(err)
		return
	}
	assert.Equal(t, 202, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
		return
	}
	assert.Equal(t, []byte(`{"status":"ok"}`), body)
	assert.Equal(t, values, server.LogsData)

}
