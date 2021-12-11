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

package otlpjsonfilereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver"

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

var (
	tracesMarshaler  = ptrace.NewJSONMarshaler()
	logsMarshaler    = plog.NewJSONMarshaler()
	metricsMarshaler = pmetric.NewJSONMarshaler()
)

func TestWatcher(t *testing.T) {
	tempFolder, err := os.MkdirTemp("", "file")
	assert.NoError(t, err)
	logsSink := new(consumertest.LogsSink)
	metricsSink := new(consumertest.MetricsSink)
	tracesSink := new(consumertest.TracesSink)
	w := watcher{
		logger:          zap.NewNop(),
		path:            tempFolder,
		logsConsumer:    logsSink,
		metricsConsumer: metricsSink,
		tracesConsumer:  tracesSink,
	}
	err = w.start(context.Background())
	assert.NoError(t, err)
	newLog := plog.NewLogs()
	newLog.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStringVal("hello world")
	logLine, err := logsMarshaler.MarshalLogs(newLog)
	assert.NoError(t, err)
	newTrace := ptrace.NewTraces()
	newTrace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("foo")
	traceLine, err := tracesMarshaler.MarshalTraces(newTrace)
	assert.NoError(t, err)
	newMetric := pmetric.NewMetrics()
	newMetric.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("foo")
	metricLine, err := metricsMarshaler.MarshalMetrics(newMetric)
	assert.NoError(t, err)

	result := logLine
	result = append(result, '\n')
	result = append(result, traceLine...)
	result = append(result, '\n')
	result = append(result, metricLine...)
	result = append(result, '\n')

	moreSpaces := "{   "
	result = append(result, moreSpaces...)
	result = append(result, traceLine[1:]...)
	result = append(result, '\n')
	result = append(result, moreSpaces...)
	result = append(result, metricLine[1:]...)
	result = append(result, '\n')
	result = append(result, moreSpaces...)
	result = append(result, logLine[1:]...)

	f, err := ioutil.TempFile("", "temp-")
	assert.NoError(t, err)
	_, err = f.Write(result)
	assert.NoError(t, err)
	err = f.Close()
	assert.NoError(t, err)
	err = os.Rename(f.Name(), filepath.Join(tempFolder, "foo"))
	assert.NoError(t, err)

	maxWait := 10
	for {
		if len(logsSink.AllLogs()) == 2 || maxWait == 0 {
			break
		}
		maxWait--
		time.Sleep(200 * time.Millisecond)
	}
	err = w.stop()
	assert.NoError(t, err)

	assert.Len(t, logsSink.AllLogs(), 2)
	assert.Len(t, metricsSink.AllMetrics(), 2)
	assert.Len(t, tracesSink.AllTraces(), 2)

}
