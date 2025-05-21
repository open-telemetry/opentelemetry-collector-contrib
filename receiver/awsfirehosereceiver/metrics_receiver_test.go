// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsfirehosereceiver

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwmetricstream"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/unmarshalertest"
)

type metricsRecordConsumer struct {
	results []pmetric.Metrics
}

var _ consumer.Metrics = (*metricsRecordConsumer)(nil)

func (rc *metricsRecordConsumer) ConsumeMetrics(_ context.Context, metrics pmetric.Metrics) error {
	rc.results = append(rc.results, metrics)
	return nil
}

func (rc *metricsRecordConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func TestMetricsReceiver_Start(t *testing.T) {
	testCases := map[string]struct {
		encoding            string
		recordType          string
		wantUnmarshalerType pmetric.Unmarshaler
		wantErr             string
	}{
		"WithDefaultEncoding": {
			wantUnmarshalerType: &cwmetricstream.Unmarshaler{},
		},
		"WithOTLP_v1Encoding": {
			recordType: "otlp_v1",
			wantUnmarshalerType: func() pmetric.Unmarshaler {
				c := metricsConsumer{}
				u, err := c.newUnmarshalerFromEncoding(context.Background(), "otlp_v1", "opentelemetry1.0")
				require.NoError(t, err)
				return u
			}(),
		},
		"WithBuiltinEncoding": {
			encoding:            "cwmetrics",
			wantUnmarshalerType: &cwmetricstream.Unmarshaler{},
		},
		"WithExtensionEncoding": {
			encoding:            "otlp_metrics",
			wantUnmarshalerType: pmetricUnmarshalerExtension{},
		},
		"WithExtensionEncodingNamed": {
			encoding:            "otlp_metrics/name",
			wantUnmarshalerType: pmetricUnmarshalerExtension{},
		},
		"WithDeprecatedRecordType": {
			recordType:          "otlp_metrics",
			wantUnmarshalerType: pmetricUnmarshalerExtension{},
		},
		"WithUnknownEncoding": {
			encoding: "invalid",
			wantErr:  `failed to start consumer: failed to load encoding extension: unknown encoding extension "invalid"`,
		},
		"WithNonLogUnmarshalerExtension": {
			encoding: "otlp_logs",
			wantErr:  `failed to start consumer: failed to load encoding extension: extension "otlp_logs" is not a metrics unmarshaler`,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Encoding = testCase.encoding
			cfg.RecordType = testCase.recordType
			got, err := newMetricsReceiver(
				cfg,
				receivertest.NewNopSettings(metadata.Type),
				consumertest.NewNop(),
			)
			require.NoError(t, err)
			require.NotNil(t, got)
			t.Cleanup(func() {
				require.NoError(t, got.Shutdown(context.Background()))
			})

			host := hostWithExtensions{
				extensions: map[component.ID]component.Component{
					component.MustNewID("otlp_logs"):                    plogUnmarshalerExtension{},
					component.MustNewID("otlp_metrics"):                 pmetricUnmarshalerExtension{},
					component.MustNewIDWithName("otlp_metrics", "name"): pmetricUnmarshalerExtension{},
				},
			}

			err = got.Start(context.Background(), host)
			if testCase.wantErr != "" {
				require.EqualError(t, err, testCase.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.IsType(t,
				testCase.wantUnmarshalerType,
				got.(*firehoseReceiver).consumer.(*metricsConsumer).unmarshaler,
			)
		})
	}
}

func TestMetricsConsumer_Errors(t *testing.T) {
	testErr := errors.New("test error")
	testCases := map[string]struct {
		unmarshalerErr error
		consumerErr    error
		wantStatus     int
		wantErr        error
	}{
		"WithUnmarshalerError": {
			unmarshalerErr: testErr,
			wantStatus:     http.StatusBadRequest,
			wantErr:        testErr,
		},
		"WithConsumerErrorPermanent": {
			consumerErr: consumererror.NewPermanent(testErr),
			wantStatus:  http.StatusBadRequest,
			wantErr:     consumererror.NewPermanent(testErr),
		},
		"WithConsumerError": {
			consumerErr: testErr,
			wantStatus:  http.StatusServiceUnavailable,
			wantErr:     testErr,
		},
		"WithNoError": {
			wantStatus: http.StatusOK,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			mc := &metricsConsumer{
				unmarshaler: unmarshalertest.NewErrMetrics(testCase.unmarshalerErr),
				consumer:    consumertest.NewErr(testCase.consumerErr),
			}
			gotStatus, gotErr := mc.Consume(
				context.Background(),
				newNextRecordFunc([][]byte{{}}),
				nil,
			)
			require.Equal(t, testCase.wantStatus, gotStatus)
			require.Equal(t, testCase.wantErr, gotErr)
		})
	}
}

func TestMetricsConsumer(t *testing.T) {
	t.Run("WithCommonAttributes", func(t *testing.T) {
		base := pmetric.NewMetrics()
		base.ResourceMetrics().AppendEmpty()
		rc := metricsRecordConsumer{}
		mc := &metricsConsumer{
			unmarshaler: unmarshalertest.NewWithMetrics(base),
			consumer:    &rc,
		}
		gotStatus, gotErr := mc.Consume(
			context.Background(),
			newNextRecordFunc([][]byte{{}}),
			map[string]string{
				"CommonAttributes": "Test",
			},
		)
		require.Equal(t, http.StatusOK, gotStatus)
		require.NoError(t, gotErr)
		require.Len(t, rc.results, 1)
		gotRms := rc.results[0].ResourceMetrics()
		require.Equal(t, 1, gotRms.Len())
		gotRm := gotRms.At(0)
		require.Equal(t, 1, gotRm.Resource().Attributes().Len())
	})
	t.Run("WithMultipleRecords", func(t *testing.T) {
		metrics0, metricSlice0 := newMetrics("service0", "scope0")
		sum0unit0 := addDeltaSumMetric(metricSlice0, "sum0", "unit0")
		addSumDataPoint(sum0unit0, 1)

		metrics1, metricSlice1 := newMetrics("service0", "scope0")
		sum0unit0 = addDeltaSumMetric(metricSlice1, "sum0", "unit0")
		addSumDataPoint(sum0unit0, 2)
		sum0unit1 := addDeltaSumMetric(metricSlice1, "sum0", "unit1")
		addSumDataPoint(sum0unit1, 3)
		scopeMetrics1 := metrics1.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty()
		newScope("scope1").MoveTo(scopeMetrics1.Scope())
		sum0unit0 = addDeltaSumMetric(scopeMetrics1.Metrics(), "sum1", "unit0")
		addSumDataPoint(sum0unit0, 4)

		metricsRemaining := []pmetric.Metrics{metrics0, metrics1}
		var unmarshaler unmarshalMetricsFunc = func([]byte) (pmetric.Metrics, error) {
			metrics := metricsRemaining[0]
			metricsRemaining = metricsRemaining[1:]
			return metrics, nil
		}

		rc := metricsRecordConsumer{}
		lc := &metricsConsumer{unmarshaler: unmarshaler, consumer: &rc}
		nextRecord := newNextRecordFunc(make([][]byte, len(metricsRemaining)))
		gotStatus, gotErr := lc.Consume(context.Background(), nextRecord, nil)
		require.Equal(t, http.StatusOK, gotStatus)
		require.NoError(t, gotErr)
		require.Len(t, rc.results, 2)
		assert.NoError(t, pmetrictest.CompareMetrics(metrics0, rc.results[0]))
		assert.NoError(t, pmetrictest.CompareMetrics(metrics1, rc.results[1]))
	})
}

func newMetrics(serviceName, scopeName string) (pmetric.Metrics, pmetric.MetricSlice) {
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	newResource(serviceName).MoveTo(resourceMetrics.Resource())
	newScope(scopeName).MoveTo(scopeMetrics.Scope())
	return metrics, scopeMetrics.Metrics()
}

func addDeltaSumMetric(metrics pmetric.MetricSlice, name, unit string) pmetric.Sum {
	m := metrics.AppendEmpty()
	m.SetName(name)
	m.SetUnit(unit)
	sum := m.SetEmptySum()
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	return sum
}

func addSumDataPoint(sum pmetric.Sum, value int64) {
	dp := sum.DataPoints().AppendEmpty()
	dp.SetIntValue(value)
}

type unmarshalMetricsFunc func([]byte) (pmetric.Metrics, error)

func (f unmarshalMetricsFunc) UnmarshalMetrics(data []byte) (pmetric.Metrics, error) {
	return f(data)
}
