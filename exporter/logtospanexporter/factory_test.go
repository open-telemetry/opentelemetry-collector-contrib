package logtospanexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/testutil"
	"go.uber.org/zap"
)

func TestFactory_TestType(t *testing.T) {
	f := NewFactory()
	assert.Equal(t, f.Type(), configmodels.Type(typeStr))
}

func TestFactory_CreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))
	ocfg, ok := factory.CreateDefaultConfig().(*Config)
	assert.True(t, ok)
	assert.Equal(t, ocfg.GRPCClientSettings.Endpoint, "")
	assert.Equal(t, ocfg.RetrySettings.Enabled, true, "default retry is enabled")
	assert.Equal(t, ocfg.RetrySettings.MaxElapsedTime, 300*time.Second, "default retry MaxElapsedTime")
	assert.Equal(t, ocfg.RetrySettings.InitialInterval, 5*time.Second, "default retry InitialInterval")
	assert.Equal(t, ocfg.RetrySettings.MaxInterval, 30*time.Second, "default retry MaxInterval")
	assert.Equal(t, ocfg.QueueSettings.Enabled, true, "default sending queue is enabled")
	assert.Equal(t, ocfg.TraceType, W3CTraceType)
	assert.Equal(t, ocfg.FieldMap.W3CTraceContextFields.Traceparent, "traceparent")
	assert.Equal(t, ocfg.FieldMap.W3CTraceContextFields.Tracestate, "tracestate")
	assert.Equal(t, ocfg.FieldMap.SpanName, "")
	assert.Equal(t, ocfg.FieldMap.SpanStartTime, "")
	assert.Equal(t, ocfg.FieldMap.SpanEndTime, "")
}

func TestFactory_CreateLogsExporter(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.GRPCClientSettings.Endpoint = testutil.GetAvailableLocalAddress(t)
	cfg.TraceType = W3CTraceType
	cfg.FieldMap.W3CTraceContextFields = W3CTraceContextFields{
		Traceparent: "traceparent",
		Tracestate:  "tracestate",
	}
	cfg.FieldMap.SpanName = "span_name"
	cfg.FieldMap.TimeFormat = UnixEpochMicroTimeFormat
	cfg.FieldMap.SpanStartTime = "req_start_time"
	cfg.FieldMap.SpanEndTime = "res_start_time"

	creationParams := component.ExporterCreateParams{
		Logger: zap.NewNop(),
		ApplicationStartInfo: component.ApplicationStartInfo{
			Version: "0.0.0",
		},
	}
	oexp, err := factory.CreateLogsExporter(context.Background(), creationParams, cfg)
	require.Nil(t, err)
	require.NotNil(t, oexp)

	require.Equal(t, "otel-collector-contrib 0.0.0", cfg.Headers["User-Agent"])
}

func TestFactory_CreateLogsExporterInvalidConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	creationParams := component.ExporterCreateParams{Logger: zap.NewNop()}
	oexp, err := factory.CreateLogsExporter(context.Background(), creationParams, cfg)
	require.Error(t, err)
	require.Nil(t, oexp)
}

func TestFactory_CreateMetricsExporter(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	creationParams := component.ExporterCreateParams{
		Logger: zap.NewNop(),
		ApplicationStartInfo: component.ApplicationStartInfo{
			Version: "0.0.0",
		},
	}
	oexp, err := factory.CreateMetricsExporter(context.Background(), creationParams, cfg)
	require.Error(t, err)
	require.Nil(t, oexp)
}

func TestFactory_CreateTracesExporter(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	creationParams := component.ExporterCreateParams{
		Logger: zap.NewNop(),
		ApplicationStartInfo: component.ApplicationStartInfo{
			Version: "0.0.0",
		},
	}
	oexp, err := factory.CreateTracesExporter(context.Background(), creationParams, cfg)
	require.Error(t, err)
	require.Nil(t, oexp)
}
