// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package netstats

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc/stats"
)

func dropView(instrument metric.Instrument) metric.View {
	return metric.NewView(
		instrument,
		metric.Stream{
			Aggregation: metric.AggregationDrop{},
		},
	)
}

func viewsFromLevel(level configtelemetry.Level) []metric.View {
	var views []metric.View

	if level == configtelemetry.LevelNone {
		return []metric.View{dropView(metric.Instrument{Name: "*"})}
	}

	// otel-arrow library metrics
	// See https://github.com/open-telemetry/otel-arrow/blob/c39257/pkg/otel/arrow_record/consumer.go#L174-L176
	if level < configtelemetry.LevelNormal {
		scope := instrumentation.Scope{Name: "otel-arrow/pkg/otel/arrow_record"}
		views = append(views,
			dropView(metric.Instrument{
				Name:  "arrow_batch_records",
				Scope: scope,
			}),
			dropView(metric.Instrument{
				Name:  "arrow_schema_resets",
				Scope: scope,
			}),
			dropView(metric.Instrument{
				Name:  "arrow_memory_inuse",
				Scope: scope,
			}),
		)
	}

	// contrib's internal/otelarrow/netstats metrics
	// See
	// - https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/a25f05/internal/otelarrow/netstats/netstats.go#L130
	// - https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/a25f05/internal/otelarrow/netstats/netstats.go#L165
	if level < configtelemetry.LevelDetailed {
		scope := instrumentation.Scope{Name: scopeName}
		// Compressed size metrics.
		views = append(views, dropView(metric.Instrument{
			Name:  "otelcol_*_compressed_size",
			Scope: scope,
		}))

		views = append(views, dropView(metric.Instrument{
			Name:  "otelcol_*_compressed_size",
			Scope: scope,
		}))

		// makeRecvMetrics for exporters.
		views = append(views, dropView(metric.Instrument{
			Name:  "otelcol_exporter_recv",
			Scope: scope,
		}))
		views = append(views, dropView(metric.Instrument{
			Name:  "otelcol_exporter_recv_wire",
			Scope: scope,
		}))

		// makeSentMetrics for receivers.
		views = append(views, dropView(metric.Instrument{
			Name:  "otelcol_receiver_sent",
			Scope: scope,
		}))
		views = append(views, dropView(metric.Instrument{
			Name:  "otelcol_receiver_sent_wire",
			Scope: scope,
		}))
	}
	return views
}

func metricValues(t *testing.T, rm metricdata.ResourceMetrics, expectMethod string) map[string]any {
	res := map[string]any{}
	for _, sm := range rm.ScopeMetrics {
		for _, mm := range sm.Metrics {
			var value int64
			var attrs attribute.Set
			switch t := mm.Data.(type) {
			case metricdata.Histogram[int64]:
				for _, dp := range t.DataPoints {
					value = dp.Sum // histogram tested as the sum
					attrs = dp.Attributes
				}
			case metricdata.Sum[int64]:
				for _, dp := range t.DataPoints {
					value = dp.Value
					attrs = dp.Attributes
				}
			}
			var method string
			for _, attr := range attrs.ToSlice() {
				if attr.Key == "method" {
					method = attr.Value.AsString()
				}
			}

			require.Equal(t, expectMethod, method)
			res[mm.Name] = value
		}
	}
	return res
}

func TestNetStatsExporterNone(t *testing.T) {
	testNetStatsExporter(t, configtelemetry.LevelNone, map[string]any{})
}

func TestNetStatsExporterNormal(t *testing.T) {
	testNetStatsExporter(t, configtelemetry.LevelNormal, map[string]any{
		"otelcol_exporter_sent":      int64(1000),
		"otelcol_exporter_sent_wire": int64(100),
	})
}

func TestNetStatsExporterDetailed(t *testing.T) {
	testNetStatsExporter(t, configtelemetry.LevelDetailed, map[string]any{
		"otelcol_exporter_sent":            int64(1000),
		"otelcol_exporter_sent_wire":       int64(100),
		"otelcol_exporter_recv_wire":       int64(10),
		"otelcol_exporter_compressed_size": int64(100), // same as sent_wire b/c sum metricValue uses histogram sum
	})
}

func testNetStatsExporter(t *testing.T, level configtelemetry.Level, expect map[string]any) {
	t.Helper()
	for _, apiDirect := range []bool{true, false} {
		t.Run(func() string {
			if apiDirect {
				return "direct"
			}
			return "grpc"
		}(), func(t *testing.T) {
			rdr := metric.NewManualReader()
			mp := metric.NewMeterProvider(
				metric.WithResource(resource.Empty()),
				metric.WithReader(rdr),
				metric.WithView(viewsFromLevel(level)...),
			)
			enr, err := NewExporterNetworkReporter(exporter.Settings{
				ID: component.NewID(component.MustNewType("test")),
				TelemetrySettings: component.TelemetrySettings{
					MeterProvider: mp,
				},
			})
			require.NoError(t, err)
			handler := enr.Handler()

			ctx := context.Background()
			for i := 0; i < 10; i++ {
				if apiDirect {
					// use the direct API
					enr.CountSend(ctx, SizesStruct{
						Method:     "Hello",
						Length:     100,
						WireLength: 10,
					})
					enr.CountReceive(ctx, SizesStruct{
						Method:     "Hello",
						Length:     10,
						WireLength: 1,
					})
				} else {
					// simulate the RPC path
					handler.HandleRPC(handler.TagRPC(ctx, &stats.RPCTagInfo{
						FullMethodName: "Hello",
					}), &stats.OutPayload{
						Length:     100,
						WireLength: 10,
					})
					handler.HandleRPC(handler.TagRPC(ctx, &stats.RPCTagInfo{
						FullMethodName: "Hello",
					}), &stats.InPayload{
						Length:     10,
						WireLength: 1,
					})
				}
			}
			var rm metricdata.ResourceMetrics
			err = rdr.Collect(ctx, &rm)
			require.NoError(t, err)

			require.Equal(t, expect, metricValues(t, rm, "Hello"))
		})
	}
}

func TestNetStatsSetSpanAttrs(t *testing.T) {
	tests := []struct {
		name       string
		attrs      []attribute.KeyValue
		isExporter bool
		length     int
		wireLength int
	}{
		{
			name:       "set exporter attributes",
			isExporter: true,
			length:     1234567,
			wireLength: 123,
			attrs: []attribute.KeyValue{
				attribute.Int("sent_uncompressed", 1234567),
				attribute.Int("sent_compressed", 123),
				attribute.Int("received_uncompressed", 1234567*2),
				attribute.Int("received_compressed", 123*2),
			},
		},
		{
			name:       "set receiver attributes",
			isExporter: false,
			length:     8901234,
			wireLength: 890,
			attrs: []attribute.KeyValue{
				attribute.Int("sent_uncompressed", 8901234),
				attribute.Int("sent_compressed", 890),
				attribute.Int("received_uncompressed", 8901234*2),
				attribute.Int("received_compressed", 890*2),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			enr := &NetworkReporter{
				isExporter: tc.isExporter,
			}

			tp := sdktrace.NewTracerProvider()
			ctx, sp := tp.Tracer("test/span").Start(context.Background(), "test-op")

			var sized SizesStruct
			sized.Method = "test"
			sized.Length = int64(tc.length)
			sized.WireLength = int64(tc.wireLength)
			enr.CountSend(ctx, sized)
			sized.Length *= 2
			sized.WireLength *= 2
			enr.CountReceive(ctx, sized)

			actualAttrs := sp.(sdktrace.ReadOnlySpan).Attributes()

			require.Equal(t, attribute.NewSet(tc.attrs...), attribute.NewSet(actualAttrs...))
		})
	}
}

func TestNetStatsReceiverNone(t *testing.T) {
	testNetStatsReceiver(t, configtelemetry.LevelNone, map[string]any{})
}

func TestNetStatsReceiverNormal(t *testing.T) {
	testNetStatsReceiver(t, configtelemetry.LevelNormal, map[string]any{
		"otelcol_receiver_recv":      int64(1000),
		"otelcol_receiver_recv_wire": int64(100),
	})
}

func TestNetStatsReceiverDetailed(t *testing.T) {
	testNetStatsReceiver(t, configtelemetry.LevelDetailed, map[string]any{
		"otelcol_receiver_recv":            int64(1000),
		"otelcol_receiver_recv_wire":       int64(100),
		"otelcol_receiver_sent_wire":       int64(10),
		"otelcol_receiver_compressed_size": int64(100), // same as recv_wire b/c sum metricValue uses histogram sum
	})
}

func testNetStatsReceiver(t *testing.T, level configtelemetry.Level, expect map[string]any) {
	for _, apiDirect := range []bool{true, false} {
		t.Run(func() string {
			if apiDirect {
				return "direct"
			}
			return "grpc"
		}(), func(t *testing.T) {
			rdr := metric.NewManualReader()
			mp := metric.NewMeterProvider(
				metric.WithResource(resource.Empty()),
				metric.WithReader(rdr),
				metric.WithView(viewsFromLevel(level)...),
			)
			rer, err := NewReceiverNetworkReporter(receiver.Settings{
				ID: component.NewID(component.MustNewType("test")),
				TelemetrySettings: component.TelemetrySettings{
					MeterProvider: mp,
					MetricsLevel:  level,
				},
			})
			require.NoError(t, err)
			handler := rer.Handler()

			ctx := context.Background()
			for i := 0; i < 10; i++ {
				if apiDirect {
					// use the direct API
					rer.CountReceive(ctx, SizesStruct{
						Method:     "Hello",
						Length:     100,
						WireLength: 10,
					})
					rer.CountSend(ctx, SizesStruct{
						Method:     "Hello",
						Length:     10,
						WireLength: 1,
					})
				} else {
					// simulate the RPC path
					handler.HandleRPC(handler.TagRPC(ctx, &stats.RPCTagInfo{
						FullMethodName: "Hello",
					}), &stats.InPayload{
						Length:     100,
						WireLength: 10,
					})
					handler.HandleRPC(handler.TagRPC(ctx, &stats.RPCTagInfo{
						FullMethodName: "Hello",
					}), &stats.OutPayload{
						Length:     10,
						WireLength: 1,
					})
				}
			}
			var rm metricdata.ResourceMetrics
			err = rdr.Collect(ctx, &rm)
			require.NoError(t, err)

			require.Equal(t, expect, metricValues(t, rm, "Hello"))
		})
	}
}

func TestUncompressedSizeBypass(t *testing.T) {
	rdr := metric.NewManualReader()
	mp := metric.NewMeterProvider(
		metric.WithResource(resource.Empty()),
		metric.WithReader(rdr),
	)
	enr, err := NewExporterNetworkReporter(exporter.Settings{
		ID: component.NewID(component.MustNewType("test")),
		TelemetrySettings: component.TelemetrySettings{
			MeterProvider: mp,
		},
	})
	require.NoError(t, err)
	handler := enr.Handler()

	ctx := context.Background()
	for i := 0; i < 10; i++ {
		// simulate the RPC path
		handler.HandleRPC(handler.TagRPC(ctx, &stats.RPCTagInfo{
			FullMethodName: "my.arrow.v1.method",
		}), &stats.OutPayload{
			Length:     9999,
			WireLength: 10,
		})
		handler.HandleRPC(handler.TagRPC(ctx, &stats.RPCTagInfo{
			FullMethodName: "my.arrow.v1.method",
		}), &stats.InPayload{
			Length:     9999,
			WireLength: 1,
		})
		// There would bo no uncompressed size metric w/o this call
		// and if the bypass didn't work, we would count the 9999s above.
		enr.CountSend(ctx, SizesStruct{
			Method: "my.arrow.v1.method",
			Length: 100,
		})
	}
	var rm metricdata.ResourceMetrics
	err = rdr.Collect(ctx, &rm)
	require.NoError(t, err)

	expect := map[string]any{
		"otelcol_exporter_sent":            int64(1000),
		"otelcol_exporter_sent_wire":       int64(100),
		"otelcol_exporter_recv_wire":       int64(10),
		"otelcol_exporter_compressed_size": int64(100),
	}
	require.Equal(t, expect, metricValues(t, rm, "my.arrow.v1.method"))
}
