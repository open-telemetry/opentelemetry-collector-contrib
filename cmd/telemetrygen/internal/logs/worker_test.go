// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
)

type mockExporter struct {
	logs []plog.Logs
}

func (m *mockExporter) export(logs plog.Logs) error {
	m.logs = append(m.logs, logs)
	return nil
}

func TestFixedNumberOfLogs(t *testing.T) {
	cfg := &Config{
		Config: common.Config{
			WorkerCount: 1,
		},
		NumLogs: 5,
	}

	exp := &mockExporter{}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, Run(cfg, exp, logger))

	time.Sleep(1 * time.Second)

	// verify
	require.Len(t, exp.logs, 5)
}

func TestRateOfLogs(t *testing.T) {
	cfg := &Config{
		Config: common.Config{
			Rate:          10,
			TotalDuration: time.Second / 2,
			WorkerCount:   1,
		},
	}
	exp := &mockExporter{}

	// test
	require.NoError(t, Run(cfg, exp, zap.NewNop()))

	// verify
	// the minimum acceptable number of logs for the rate of 10/sec for half a second
	assert.True(t, len(exp.logs) >= 5, "there should have been 5 or more logs, had %d", len(exp.logs))
	// the maximum acceptable number of logs for the rate of 10/sec for half a second
	assert.True(t, len(exp.logs) <= 20, "there should have been less than 20 logs, had %d", len(exp.logs))
}

func TestUnthrottled(t *testing.T) {
	cfg := &Config{
		Config: common.Config{
			TotalDuration: 1 * time.Second,
			WorkerCount:   1,
		},
	}
	exp := &mockExporter{}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, Run(cfg, exp, logger))

	assert.True(t, len(exp.logs) > 100, "there should have been more than 100 logs, had %d", len(exp.logs))
}

func TestRandomAttributes(t *testing.T) {
	minV := 2
	maxV := 4
	cfg := &Config{
		Config: common.Config{
			TotalDuration: 1 * time.Second,
			WorkerCount:   1,
			ResourceAttributes: common.KeyValue{
				"attrA": fmt.Sprintf("a-${random:%d-%d}", minV, maxV),
				"attrB": fmt.Sprintf("b-${random:%d-%d}-${random:%d-%d}", minV, maxV, minV, maxV),
			},
		},
	}
	exp := &mockExporter{}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, Run(cfg, exp, logger))

	assert.True(t, len(exp.logs) > 100, "there should have been more than 100 logs, had %d", len(exp.logs))

	seenA := make(map[string]bool)
	seenB := make(map[string]bool)
	for _, log := range exp.logs {
		val, ok := log.ResourceLogs().At(0).Resource().Attributes().Get("attrA")
		if ok {
			seenA[val.AsString()] = true
		}

		val, ok = log.ResourceLogs().At(0).Resource().Attributes().Get("attrB")
		if ok {
			seenB[val.AsString()] = true
		}
	}

	expSeenA := make(map[string]bool)
	expSeenB := make(map[string]bool)
	for i := minV; i < maxV; i++ {
		expSeenA[fmt.Sprintf("a-%d", i)] = true
		for j := minV; j < maxV; j++ {
			expSeenB[fmt.Sprintf("b-%d-%d", i, j)] = true
		}

	}

	assert.Equal(t, expSeenA, seenA)
	assert.Equal(t, expSeenB, seenB)
}
