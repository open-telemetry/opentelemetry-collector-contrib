package linuxfiledescriptorreceiver

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
)

type testMetricsConsumer struct {
	metrics pmetric.Metrics
}

func (c *testMetricsConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (c *testMetricsConsumer) ConsumeMetrics(_ context.Context, metrics pmetric.Metrics) error {
	c.metrics = metrics
	return nil
}

func TestParseFileNr(t *testing.T) {
	allocated, unused, maximum, err := parseFileNr("13223\t0\t9223372036854775807\n")

	require.NoError(t, err)
	require.Equal(t, uint64(13223), allocated)
	require.Equal(t, uint64(0), unused)
	require.Equal(t, uint64(9223372036854775807), maximum)
}

func TestParseFileNrInvalidFieldCount(t *testing.T) {
	_, _, _, err := parseFileNr("13223\t0\n")

	require.Error(t, err)
}

func TestParseFileNrInvalidValue(t *testing.T) {
	_, _, _, err := parseFileNr("invalid\t0\t100\n")

	require.Error(t, err)
}

func TestParseFileNrZeroMaximum(t *testing.T) {
	_, _, _, err := parseFileNr("13223\t0\t0\n")

	require.Error(t, err)
}

func TestParseFileNrAllocatedExceedsMaximum(t *testing.T) {
	_, _, _, err := parseFileNr("200\t0\t100\n")

	require.Error(t, err)
}

func TestReceiverScrape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file-nr")

	require.NoError(t, os.WriteFile(
		path,
		[]byte("100\t25\t1000\n"),
		0600,
	))

	cfg := &Config{Path: path, Interval: time.Second}

	consumer := &testMetricsConsumer{}

	r, err := newReceiver(
		receiver.Settings{},
		cfg,
		consumer,
	)

	require.NoError(t, err)

	receiverImpl := r.(*receiverImpl)

	metrics, err := receiverImpl.scrape()
	require.NoError(t, err)

	require.Equal(t, 1, metrics.ResourceMetrics().Len())

	scopeMetrics := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)
	require.Equal(t, 4, scopeMetrics.Metrics().Len())

	assertMetricValue(t, scopeMetrics.Metrics().At(0), "linux.file_descriptors.allocated", 100)
	assertMetricValue(t, scopeMetrics.Metrics().At(1), "linux.file_descriptors.unused", 25)
	assertMetricValue(t, scopeMetrics.Metrics().At(2), "linux.file_descriptors.maximum", 1000)
	assertMetricValue(t, scopeMetrics.Metrics().At(3), "linux.file_descriptors.used_percent", 10)
}

func assertMetricValue(t *testing.T, metric pmetric.Metric, name string, expected float64) {
	t.Helper()

	require.Equal(t, name, metric.Name())
	require.Equal(t, pmetric.MetricTypeGauge, metric.Type())

	points := metric.Gauge().DataPoints()
	require.Equal(t, 1, points.Len())
	require.InDelta(t, expected, points.At(0).DoubleValue(), 0.000001)
}
