// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestConsumeLogsManyLogsShouldSucceed(t *testing.T) {
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

		assert.NoError(t, err, "Error reading request: %v", err)

		for _, ev := range cer.Events {
			processedEvents.Add(1)
			key, found := ev.Attrs["body.str"]
			assert.True(t, found)
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
		assert.NoError(t, err)
		l, err := w.Write(payload)
		assert.Greater(t, l, 1)
		assert.NoError(t, err)
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
	if assert.NoError(t, err) {
		err = logs.Start(context.Background(), componenttest.NewNopHost())
		assert.NoError(t, err)

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
			assert.Nil(t, err)
			time.Sleep(time.Duration(float64(maxDelay.Nanoseconds()) * 0.7))
		}

		assert.NotNil(t, logs)

		time.Sleep(time.Second)
		err = logs.Shutdown(context.Background())
		assert.Nil(t, err)
		lastProcessed := uint64(0)
		sameNumber := 0
		for {
			t.Logf("Processed events: %d / %d", processedEvents.Load(), expectedLogs)
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

	assert.True(t, wasSuccessful.Load())

	assert.Equal(t, seenKeys, expectedKeys)
	assert.Equal(t, expectedLogs, processedEvents.Load(), "processed items")
	assert.Equal(t, expectedLogs, uint64(len(seenKeys)), "unique items")
}
