package logtospanexporter

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	otlp "go.opentelemetry.io/collector/exporter/otlpexporter"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[configmodels.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, 2, len(cfg.Exporters))

	exporter := cfg.Exporters["logtospan/allsettings"]
	actualCfg := exporter.(*Config)
	expectedCfg := &Config{
		Config: otlp.Config{
			ExporterSettings: configmodels.ExporterSettings{
				NameVal: "logtospan/allsettings",
				TypeVal: "logtospan",
			},
			RetrySettings: exporterhelper.RetrySettings{
				Enabled:         true,
				InitialInterval: 10 * time.Second,
				MaxInterval:     1 * time.Minute,
				MaxElapsedTime:  10 * time.Minute,
			},
			QueueSettings: exporterhelper.QueueSettings{
				Enabled:      true,
				NumConsumers: 2,
				QueueSize:    10,
			},
			GRPCClientSettings: configgrpc.GRPCClientSettings{
				Endpoint:        "otelcol:4317",
				ReadBufferSize:  123,
				WriteBufferSize: 345,
				Headers: map[string]string{
					"User-Agent": "otel-collector-contrib {{version}}",
				},
			},
			TimeoutSettings: exporterhelper.TimeoutSettings{Timeout: 5000000000},
		},
		TraceType: W3CTraceType,
		FieldMap: FieldMap{
			W3CTraceContextFields: W3CTraceContextFields{
				Traceparent: "traceparent",
				Tracestate:  "tracestate",
			},
			SpanName:      "span_name",
			SpanStartTime: "req_start_time",
			SpanEndTime:   "res_start_time",
			Ignored:       []string{"fluent.tag"},
		},
	}
	assert.Equal(t, expectedCfg, actualCfg)
}

func TestConfig_sanitize(t *testing.T) {
	type fields struct {
		ExporterSettings configmodels.ExporterSettings
		Endpoint         string
		TraceType        TraceType
		FieldMap         FieldMap
	}
	tests := []struct {
		name         string
		fields       fields
		errorMessage string
		shouldError  bool
	}{
		{
			name: "Test missing endpoint",
			fields: fields{
				Endpoint: "",
			},
			errorMessage: "\"endpoint\" must be set",
			shouldError:  true,
		},
		{
			name: "Test invalid trace_type",
			fields: fields{
				TraceType: "invalid",
			},
			errorMessage: "\"trace_type\" of 'invalid' is not supported",
			shouldError:  true,
		},
		{
			name: "Test missing field_map.w3c.traceparent",
			fields: fields{
				TraceType: W3CTraceType,
				FieldMap: FieldMap{
					W3CTraceContextFields: W3CTraceContextFields{
						Traceparent: "",
					},
				},
			},
			errorMessage: "\"field_map.w3c.traceparent\" must be set",
			shouldError:  true,
		},
		{
			name: "Test missing field_map.w3c.tracestate",
			fields: fields{
				TraceType: W3CTraceType,
				FieldMap: FieldMap{
					W3CTraceContextFields: W3CTraceContextFields{
						Tracestate: "",
					},
				},
			},
			errorMessage: "\"field_map.w3c.tracestate\" must be set",
			shouldError:  true,
		},
		{
			name: "Test missing field_map.span_name",
			fields: fields{
				FieldMap: FieldMap{
					SpanName: "",
				},
			},
			errorMessage: "\"field_map.span_name\" must be set",
			shouldError:  true,
		},
		{
			name: "Test missing field_map.span_start_time",
			fields: fields{
				FieldMap: FieldMap{
					SpanStartTime: "",
				},
			},
			errorMessage: "\"field_map.span_start_time\" must be set",
			shouldError:  true,
		},
		{
			name: "Test missing field_map.span_end_time",
			fields: fields{
				FieldMap: FieldMap{
					SpanEndTime: "",
				},
			},
			errorMessage: "\"field_map.span_end_time\" must be set",
			shouldError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.ExporterSettings = tt.fields.ExporterSettings
			cfg.Endpoint = tt.fields.Endpoint
			cfg.TraceType = tt.fields.TraceType
			cfg.FieldMap = tt.fields.FieldMap

			err := cfg.sanitize()
			if (err != nil) != tt.shouldError {
				t.Errorf("sanitize() error = %v, shouldError %v", err, tt.shouldError)
				return
			}

			if tt.shouldError {
				assert.Error(t, err)
				if len(tt.errorMessage) != 0 {
					assert.Contains(t, err.Error(), tt.errorMessage)
				}
			}
		})
	}
}
