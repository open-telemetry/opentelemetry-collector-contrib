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

//go:build integration
// +build integration

package datasetexporter

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type SuiteLogsE2EExporter struct {
	suite.Suite
}

func (s *SuiteLogsE2EExporter) SetupTest() {
	os.Clearenv()
}

func TestSuiteLogsE2EExporterIntegration(t *testing.T) {
	suite.Run(t, new(SuiteLogsE2EExporter))
}

func (s *SuiteLogsE2EExporter) TestConsumeLogsManyLogsShouldSucceed() {
	const maxDelay = 200 * time.Millisecond
	createSettings := exportertest.NewNopCreateSettings()

	const maxBatchCount = 20
	const logsPerBatch = 10000
	const expectedLogs = uint64(maxBatchCount * logsPerBatch)

	attempt := atomic.Uint64{}
	wasSuccessful := atomic.Bool{}
	processedEvents := atomic.Uint64{}
	seenKeys := make(map[string]int64)
	expectedKeys := make(map[string]int64)
	mutex := &sync.RWMutex{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		attempt.Add(1)
		cer, err := extract(req)

		s.NoError(err, "Error reading request: %v", err)

		for _, ev := range cer.Events {
			processedEvents.Add(1)
			key, found := ev.Attrs["body.str"]
			s.True(found)
			mutex.Lock()
			sKey := key.(string)
			_, f := seenKeys[sKey]
			if !f {
				seenKeys[sKey] = 0
			}
			seenKeys[sKey]++
			mutex.Unlock()
		}

		wasSuccessful.Store(true)
		payload, err := json.Marshal(map[string]interface{}{
			"status":       "success",
			"bytesCharged": 42,
		})
		s.NoError(err)
		l, err := w.Write(payload)
		s.Greater(l, 1)
		s.NoError(err)
	}))
	defer server.Close()

	config := &Config{
		DatasetURL: server.URL,
		APIKey:     "key-lib",
		BufferSettings: BufferSettings{
			MaxLifetime: maxDelay,
			GroupBy:     []string{"attributes.container_id"},
		},
		RetrySettings:   exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
		TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
	}

	logs, err := createLogsExporter(context.Background(), createSettings, config)
	waitingTime := time.Duration(0)
	if s.NoError(err) {
		err = logs.Start(context.Background(), componenttest.NewNopHost())
		s.NoError(err)

		for bI := 0; bI < maxBatchCount; bI++ {
			batch := plog.NewLogs()
			rL := batch.ResourceLogs().AppendEmpty()
			sL := rL.ScopeLogs().AppendEmpty()
			for lI := 0; lI < logsPerBatch; lI++ {
				key := fmt.Sprintf("%04d-%06d", bI, lI)
				log := sL.LogRecords().AppendEmpty()
				log.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
				log.Body().SetStr(key)
				log.Attributes().PutStr("key", key)
				log.Attributes().PutStr("p1", strings.Repeat("A", rand.Intn(2000)))
				expectedKeys[key] = 1
			}
			err = logs.ConsumeLogs(context.Background(), batch)
			s.Nil(err)
			time.Sleep(time.Duration(float64(maxDelay.Nanoseconds()) * 0.7))
		}

		s.NotNil(logs)

		time.Sleep(time.Second)
		err = logs.Shutdown(context.Background())
		s.Nil(err)
		lastProcessed := uint64(0)
		sameNumber := 0
		for {
			s.T().Logf("Processed events: %d / %d", processedEvents.Load(), expectedLogs)
			if lastProcessed == processedEvents.Load() {
				sameNumber++
			}
			if processedEvents.Load() >= expectedLogs || sameNumber > 10 {
				break
			}
			lastProcessed = processedEvents.Load()
			time.Sleep(time.Second)
			waitingTime += time.Second
		}
	}

	time.Sleep(2 * time.Second)

	s.True(wasSuccessful.Load())

	s.Equal(seenKeys, expectedKeys)
	s.Equal(expectedLogs, processedEvents.Load(), "processed items")
	s.Equal(expectedLogs, uint64(len(seenKeys)), "unique items")
}
