// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package carbonreceiver

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/internal/metadata"
)

func TestReporterObservability(t *testing.T) {
	receiverID := component.NewIDWithName(metadata.Type, "fake_receiver")
	tt := componenttest.NewTelemetry()
	defer func() {
		require.NoError(t, tt.Shutdown(t.Context()))
	}()

	reporter, err := newReporter(receiver.Settings{ID: receiverID, TelemetrySettings: tt.NewTelemetrySettings(), BuildInfo: component.NewDefaultBuildInfo()})
	require.NoError(t, err)

	ctx := reporter.OnDataReceived(t.Context())

	reporter.OnMetricsProcessed(ctx, 17, nil)

	assertReceiverMetrics(t, tt, receiverID, 17, 0)

	// Below just exercise the error paths.
	err = errors.New("fake error for tests")
	reporter.OnTranslationError(ctx, err)
	reporter.OnMetricsProcessed(ctx, 10, err)

	assertReceiverMetrics(t, tt, receiverID, 17, 10)
}

func assertReceiverMetrics(t *testing.T, tt *componenttest.Telemetry, id component.ID, accepted, refused int64) {
	got, err := tt.GetMetric("otelcol_receiver_accepted_metric_points")
	assert.NoError(t, err)
	metricdatatest.AssertEqual(t,
		metricdata.Metrics{
			Name:        "otelcol_receiver_accepted_metric_points",
			Description: "Number of metric points successfully pushed into the pipeline. [Alpha]",
			Unit:        "{datapoints}",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Attributes: attribute.NewSet(
							attribute.String("receiver", id.String()),
							attribute.String("transport", "tcp")),
						Value: accepted,
					},
				},
			},
		}, got, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())

	got, err = tt.GetMetric("otelcol_receiver_refused_metric_points")
	assert.NoError(t, err)
	metricdatatest.AssertEqual(t,
		metricdata.Metrics{
			Name:        "otelcol_receiver_refused_metric_points",
			Description: "Number of metric points that could not be pushed into the pipeline. [Alpha]",
			Unit:        "{datapoints}",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Attributes: attribute.NewSet(
							attribute.String("receiver", id.String()),
							attribute.String("transport", "tcp")),
						Value: refused,
					},
				},
			},
		}, got, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())
}
