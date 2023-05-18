// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filereceiver

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestFileReader_Readline(t *testing.T) {
	tc := testConsumer{}
	cons := consumerType{
		metricsConsumer: &tc,
	}
	f, err := os.Open(filepath.Join("testdata", "metrics.json"))
	require.NoError(t, err)
	fr := newFileReader(cons, f, newReplayTimer(0), "json", "none")
	err = fr.readMetricLine(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, len(tc.consumed))
	metrics := tc.consumed[0]
	assert.Equal(t, 26, metrics.MetricCount())
	byName := metricsByName(metrics)
	rcpMetric := byName["redis.commands.processed"]
	v := rcpMetric.Sum().DataPoints().At(0).IntValue()
	const testdataValue = 2076
	assert.EqualValues(t, testdataValue, v)
}

func TestFileReader_ReadChunk(t *testing.T) {
	tc := testConsumer{}
	cons := consumerType{
		metricsConsumer: &tc,
	}
	f, err := os.Open(filepath.Join("testdata", "metrics.proto"))
	require.NoError(t, err)
	fr := newFileReader(cons, f, newReplayTimer(0), "proto", "")
	err = fr.readMetricChunk(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, len(tc.consumed))
	metrics := tc.consumed[0]
	assert.Equal(t, 26, metrics.MetricCount())
	byName := metricsByName(metrics)
	rcpMetric := byName["redis.commands.processed"]
	v := rcpMetric.Sum().DataPoints().At(0).IntValue()
	const testdataValue = 2076
	assert.EqualValues(t, testdataValue, v)
}

func TestFileReader_Cancellation(t *testing.T) {
	fr := fileReader{
		consumer: consumerType{
			metricsConsumer: consumertest.NewNop(),
		},
		stringReader: blockingStringReader{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	errs := make(chan error)
	go func() {
		errs <- fr.readAllLines(ctx)
	}()
	cancel()
	require.Equal(t, 0, len(errs))
}

func TestFileReader_ReadAll(t *testing.T) {
	tc := testConsumer{}
	cons := consumerType{
		metricsConsumer: &tc,
	}
	f, err := os.Open(filepath.Join("testdata", "metrics.json"))
	require.NoError(t, err)
	sleeper := &fakeSleeper{}
	rt := &replayTimer{
		throttle:  2,
		sleepFunc: sleeper.fakeSleep,
	}
	fr := newFileReader(cons, f, rt, "json", "")
	err = fr.readAllLines(context.Background())
	require.NoError(t, err)
	const expectedSleeps = 10
	assert.Len(t, sleeper.durations, expectedSleeps)
	assert.EqualValues(t, 0, sleeper.durations[0])
	for i := 1; i < expectedSleeps; i++ {
		expected := time.Second * 4
		actual := sleeper.durations[i]
		delta := time.Millisecond * 10
		assert.InDelta(t, float64(expected), float64(actual), float64(delta))
	}
}

func TestFileReader_ReadAllChunks(t *testing.T) {
	tc := testConsumer{}
	cons := consumerType{
		metricsConsumer: &tc,
	}
	f, err := os.Open(filepath.Join("testdata", "metrics.proto"))
	require.NoError(t, err)
	sleeper := &fakeSleeper{}
	rt := &replayTimer{
		throttle:  2,
		sleepFunc: sleeper.fakeSleep,
	}
	fr := newFileReader(cons, f, rt, "proto", "")
	err = fr.readAllChunks(context.Background())
	require.NoError(t, err)
	const expectedSleeps = 10
	assert.Len(t, sleeper.durations, expectedSleeps)
	assert.EqualValues(t, 0, sleeper.durations[0])
	for i := 1; i < expectedSleeps; i++ {
		expected := time.Second * 4
		actual := sleeper.durations[i]
		delta := time.Millisecond * 10
		assert.InDelta(t, float64(expected), float64(actual), float64(delta))
	}
}

type blockingStringReader struct {
}

func (sr blockingStringReader) ReadString(byte) (string, error) {
	select {}
}

func (sr blockingStringReader) Read([]byte) (int, error) {
	return 0, nil
}

func metricsByName(pm pmetric.Metrics) map[string]pmetric.Metric {
	out := map[string]pmetric.Metric{}
	for i := 0; i < pm.ResourceMetrics().Len(); i++ {
		sms := pm.ResourceMetrics().At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				metric := ms.At(k)
				out[metric.Name()] = metric
			}
		}
	}
	return out
}