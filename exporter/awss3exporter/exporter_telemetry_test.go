// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/metadatatest"
)

// TestExporterTelemetryWiring goes through the start of the exporter, against a
// stub S3, to check that the upload telemetry reaches the upload manager.
func TestExporterTelemetryWiring(t *testing.T) {
	for name, status := range map[string]int{
		"success":       http.StatusOK,
		"access denied": http.StatusForbidden,
	} {
		t.Run(name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
				if status != http.StatusOK {
					_, _ = fmt.Fprint(w, `<Error><Code>AccessDenied</Code><Message>m</Message></Error>`)
				}
			}))
			t.Cleanup(s.Close)

			cfg := createDefaultConfig().(*Config)
			cfg.S3Uploader.Endpoint = s.URL
			cfg.S3Uploader.S3Bucket = "my-bucket"
			cfg.S3Uploader.S3ForcePathStyle = true
			cfg.S3Uploader.AccessKey = "access"
			cfg.S3Uploader.SecretKey = "secret"
			cfg.Telemetry.Enabled = true

			tt := componenttest.NewTelemetry()
			t.Cleanup(func() { assert.NoError(t, tt.Shutdown(context.WithoutCancel(t.Context()))) })
			e, err := newS3Exporter(cfg, "logs", metadatatest.NewSettings(tt))
			require.NoError(t, err)
			require.NoError(t, e.start(t.Context(), componenttest.NewNopHost()))
			t.Cleanup(func() { assert.NoError(t, e.shutdown(context.WithoutCancel(t.Context()))) })

			logs := plog.NewLogs()
			logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
			err = e.ConsumeLogs(t.Context(), logs)

			if status == http.StatusOK {
				require.NoError(t, err)
				// The exact size is checked in the upload tests, here it only has to be recorded.
				metadatatest.AssertEqualAwss3UploadedSize(t, tt, []metricdata.DataPoint[int64]{
					{Attributes: attribute.NewSet()},
				}, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue())
				// The measured durations are not deterministic, so only the series is checked.
				metadatatest.AssertEqualAwss3UploadDuration(t, tt, []metricdata.HistogramDataPoint[float64]{
					{Attributes: attribute.NewSet(attribute.String("outcome", "success"))},
				}, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue())
				return
			}

			require.Error(t, err)
			metadatatest.AssertEqualAwss3UploadDuration(t, tt, []metricdata.HistogramDataPoint[float64]{
				{Attributes: attribute.NewSet(attribute.String("outcome", "failure"), attribute.String("error.type", "AccessDenied"))},
			}, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue())
		})
	}
}

// failingMeterProvider fails to create any instrument.
type failingMeterProvider struct{ noopmetric.MeterProvider }

func (failingMeterProvider) Meter(string, ...metric.MeterOption) metric.Meter {
	return failingMeter{}
}

type failingMeter struct{ noopmetric.Meter }

func (failingMeter) Int64Counter(string, ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	return nil, errors.New("cannot create the instrument")
}

// TestTelemetryDisabledByDefault checks that with the default config the
// telemetry is not even created, so no instrument is registered.
func TestTelemetryDisabledByDefault(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	require.False(t, cfg.Telemetry.Enabled)

	settings := exportertest.NewNopSettings(metadata.Type)
	settings.MeterProvider = failingMeterProvider{}

	e, err := newS3Exporter(cfg, "logs", settings)
	require.NoError(t, err)
	assert.Nil(t, e.telemetry)
}

func TestTelemetryCreationError(t *testing.T) {
	settings := exportertest.NewNopSettings(metadata.Type)
	settings.MeterProvider = failingMeterProvider{}
	cfg := createDefaultConfig()
	cfg.(*Config).Telemetry.Enabled = true

	_, err := newS3Exporter(cfg.(*Config), "logs", settings)
	assert.ErrorContains(t, err, "cannot create the instrument")

	_, err = createLogsExporter(t.Context(), settings, cfg)
	assert.ErrorContains(t, err, "cannot create the instrument")
	_, err = createMetricsExporter(t.Context(), settings, cfg)
	assert.ErrorContains(t, err, "cannot create the instrument")
	_, err = createTracesExporter(t.Context(), settings, cfg)
	assert.ErrorContains(t, err, "cannot create the instrument")
}
