// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsemfexporter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.NotNil(t, cfg.(*Config).logger)
	require.NotNil(t, cfg.(*Config).Timeout)
	assert.Equal(t, 5*time.Second, *cfg.(*Config).Timeout)
}

func TestCreateTraces(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "1").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	ctx := t.Context()
	exporter, err := factory.CreateTraces(ctx, exportertest.NewNopSettings(metadata.Type), cfg)
	assert.Error(t, err)
	assert.Nil(t, exporter)
}

func TestCreateMetrics(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "1").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	ctx := t.Context()
	exporter, err := factory.CreateMetrics(ctx, exportertest.NewNopSettings(metadata.Type), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exporter)
}

func TestMetricsExportTimeout(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	for _, tt := range []struct {
		name          string
		timeout       string
		delay         time.Duration
		wantDeadline  bool
		direct        bool
		parentTimeout time.Duration
	}{
		{name: "longer than default", timeout: "15s", delay: 6 * time.Second},
		{name: "configured deadline", timeout: "500ms", delay: 2 * time.Second, wantDeadline: true},
		{name: "direct config omitted timeout", delay: 6 * time.Second, wantDeadline: true, direct: true},
		{name: "direct config zero timeout", timeout: "0s", delay: 6 * time.Second, direct: true},
		{name: "YAML zero timeout", timeout: "0s", delay: 6 * time.Second},
		{name: "zero respects caller deadline", timeout: "0s", delay: 2 * time.Second, wantDeadline: true, parentTimeout: 500 * time.Millisecond},
	} {
		t.Run(tt.name, func(t *testing.T) {
			received := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					LogEvents []struct {
						Message string `json:"message"`
					} `json:"logEvents"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.LogEvents) != 1 {
					t.Error("expected one EMF log event")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				received <- request.LogEvents[0].Message
				select {
				case <-time.After(tt.delay):
					w.Header().Set("Content-Type", "application/x-amz-json-1.1")
					_, _ = w.Write([]byte(`{"nextSequenceToken":"next"}`))
				case <-r.Context().Done():
				}
			}))
			defer server.Close()

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			if tt.direct {
				cfg = &Config{
					AWSSessionSettings: awsutil.AWSSessionSettings{
						Endpoint: server.URL, Region: "us-east-1", RequestTimeoutSeconds: 30,
					},
					LogGroupName: "test-group", LogStreamName: "test-stream",
					DimensionRollupOption: "NoDimensionRollup", OutputDestination: "cloudwatch",
				}
				if tt.timeout != "" {
					timeout, err := time.ParseDuration(tt.timeout)
					require.NoError(t, err)
					cfg.Timeout = &timeout
				}
			} else {
				require.NoError(t, confmap.NewFromStringMap(map[string]any{
					"timeout": tt.timeout, "endpoint": server.URL, "region": "us-east-1",
					"log_group_name": "test-group", "log_stream_name": "test-stream",
					"dimension_rollup_option": "NoDimensionRollup", "max_retries": 0,
				}).Unmarshal(cfg))
			}
			require.NoError(t, confmap.Validate(cfg))
			exp, err := factory.CreateMetrics(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
			require.NoError(t, err)
			require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))
			defer func() { assert.NoError(t, exp.Shutdown(t.Context())) }()

			metrics := pmetric.NewMetrics()
			metric := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
			metric.SetName("test_gauge")
			point := metric.SetEmptyGauge().DataPoints().AppendEmpty()
			point.SetIntValue(7)
			point.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			ctx := t.Context()
			if tt.parentTimeout != 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tt.parentTimeout)
				defer cancel()
			}
			err = exp.ConsumeMetrics(ctx, metrics)
			if tt.wantDeadline {
				require.ErrorIs(t, err, context.DeadlineExceeded)
			} else {
				require.NoError(t, err)
			}
			select {
			case message := <-received:
				var event map[string]any
				require.NoError(t, json.Unmarshal([]byte(message), &event))
				assert.Equal(t, float64(7), event["test_gauge"])
			default:
				t.Fatal("no metric reached the CloudWatch endpoint")
			}
		})
	}
}
