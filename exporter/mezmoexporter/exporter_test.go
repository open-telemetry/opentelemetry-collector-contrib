// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mezmoexporter

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.6.1"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func createSimpleLogData(numberOfLogs int) plog.Logs {
	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty() // Add an empty ResourceLogs
	rl := logs.ResourceLogs().AppendEmpty()
	rl.ScopeLogs().AppendEmpty() // Add an empty ScopeLogs
	sl := rl.ScopeLogs().AppendEmpty()

	for i := 0; i < numberOfLogs; i++ {
		ts := pcommon.Timestamp(int64(i) * time.Millisecond.Nanoseconds())
		logRecord := sl.LogRecords().AppendEmpty()
		logRecord.Body().SetStringVal("10byteslog")
		logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
		logRecord.Attributes().InsertString("my-label", "myapp-type")
		logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
		logRecord.Attributes().InsertString("custom", "custom")
		logRecord.SetTimestamp(ts)
	}

	return logs
}

// Creates a logs set that exceeds the maximum message side we can send in one HTTP POST
func createMaxLogData() plog.Logs {
	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty() // Add an empty ResourceLogs
	rl := logs.ResourceLogs().AppendEmpty()
	rl.ScopeLogs().AppendEmpty() // Add an empty ScopeLogs
	sl := rl.ScopeLogs().AppendEmpty()

	var lineLen = maxMessageSize
	var lineCnt = (maxBodySize / lineLen) * 2

	for i := 0; i < lineCnt; i++ {
		ts := pcommon.Timestamp(int64(i) * time.Millisecond.Nanoseconds())
		logRecord := sl.LogRecords().AppendEmpty()
		logRecord.Body().SetStringVal(randString(maxMessageSize))
		logRecord.SetTimestamp(ts)
	}

	return logs
}

func createSizedPayloadLogData(payloadSize int) plog.Logs {
	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty() // Add an empty ResourceLogs
	rl := logs.ResourceLogs().AppendEmpty()
	rl.ScopeLogs().AppendEmpty() // Add an empty ScopeLogs
	sl := rl.ScopeLogs().AppendEmpty()

	maxMsg := randString(payloadSize)

	ts := pcommon.Timestamp(0)
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.Body().SetStringVal(maxMsg)
	logRecord.SetTimestamp(ts)

	return logs
}

func TestLogsExporter(t *testing.T) {
	// Spin up a HTTP server to receive the test exporters...
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}

		var logBody MezmoLogBody
		if err = json.Unmarshal(body, &logBody); err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	assert.NoError(t, err)

	config := &Config{
		IngestURL: serverURL.String(),
	}

	exp := newLogsExporter(config, componenttest.NewNopTelemetrySettings())
	require.NotNil(t, exp)

	err = exp.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	t.Run("Test simple log data", func(t *testing.T) {
		var logs = createSimpleLogData(3)
		exp.pushLogData(context.Background(), logs)
	})

	t.Run("Test max message size", func(t *testing.T) {
		var logs = createSizedPayloadLogData(maxMessageSize)
		exp.pushLogData(context.Background(), logs)
	})

	t.Run("Test max body size", func(t *testing.T) {
		var logs = createMaxLogData()
		exp.pushLogData(context.Background(), logs)
	})
}
