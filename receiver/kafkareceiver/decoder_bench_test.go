// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
)

var benchmarkDecodedMetrics pmetric.Metrics

// BenchmarkMetricsMessageDecoding compares the existing byte-only path with a
// message extension that applies its options and reads one metadata key. Kafka
// transport, metadata construction, and downstream consumption are excluded.
func BenchmarkMetricsMessageDecoding(b *testing.B) {
	ctx := client.NewContext(b.Context(), client.Info{Metadata: client.NewMetadata(map[string][]string{"tenant": {"benchmark"}})})
	set := receivertest.NewNopSettings(metadata.Type)
	for _, points := range []int{1, 100, 1000} {
		b.Run(fmt.Sprintf("points=%d", points), func(b *testing.B) {
			fixture := benchmarkMetricsFixture(points)
			payload, err := (&pmetric.ProtoMarshaler{}).MarshalMetrics(fixture)
			if err != nil {
				b.Fatal(err)
			}
			original, err := newMetricsUnmarshaler("otlp_proto", set, componenttest.NewNopHost())
			if err != nil {
				b.Fatal(err)
			}
			legacy, err := newMetricsDecoder("otlp_proto", set, componenttest.NewNopHost())
			if err != nil {
				b.Fatal(err)
			}
			message, err := newMetricsDecoder("benchmark", set, extensionsHost{
				component.MustNewID("benchmark"): &benchmarkMetricsMessageExtension{unmarshaler: &pmetric.ProtoUnmarshaler{}},
			})
			if err != nil {
				b.Fatal(err)
			}
			decoded, err := message(ctx, payload)
			if err != nil || decoded.DataPointCount() != points {
				b.Fatalf("invalid message decode result: %v", err)
			}
			b.Run("direct_unmarshaler", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(payload)))
				for b.Loop() {
					decoded, err := original.UnmarshalMetrics(payload)
					if err != nil {
						b.Fatal(err)
					}
					benchmarkDecodedMetrics = decoded
				}
			})
			for _, tc := range []struct {
				name   string
				decode messageDecoder[pmetric.Metrics]
			}{{"kafka_legacy", legacy}, {"message_metadata_lookup", message}} {
				b.Run(tc.name, func(b *testing.B) {
					b.ReportAllocs()
					b.SetBytes(int64(len(payload)))
					for b.Loop() {
						decoded, err := tc.decode(ctx, payload)
						if err != nil {
							b.Fatal(err)
						}
						benchmarkDecodedMetrics = decoded
					}
				})
			}
		})
	}
}

// BenchmarkMetricsMessageMetadata isolates the message dispatch, option
// application, and metadata lookup by returning an already decoded fixture.
func BenchmarkMetricsMessageMetadata(b *testing.B) {
	ctx := client.NewContext(b.Context(), client.Info{Metadata: client.NewMetadata(map[string][]string{"tenant": {"benchmark"}})})
	fixture := benchmarkMetricsFixture(1)
	fixture.MarkReadOnly()
	decode, err := newMetricsDecoder("benchmark", receivertest.NewNopSettings(metadata.Type), extensionsHost{
		component.MustNewID("benchmark"): &benchmarkMetricsMessageExtension{fixture: fixture},
	})
	if err != nil {
		b.Fatal(err)
	}
	payload := []byte("already decoded fixture")
	b.ReportAllocs()
	for b.Loop() {
		decoded, err := decode(ctx, payload)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkDecodedMetrics = decoded
	}
}

func benchmarkMetricsFixture(points int) pmetric.Metrics {
	data := pmetric.NewMetrics()
	metric := data.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetName("network.bytes")
	dps := metric.SetEmptyGauge().DataPoints()
	for i := range points {
		dp := dps.AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.Attributes().PutStr("interface", fmt.Sprintf("eth%d", i))
	}
	return data
}

type benchmarkMetricsMessageExtension struct {
	component.Component
	unmarshaler pmetric.Unmarshaler
	fixture     pmetric.Metrics
}

func (e *benchmarkMetricsMessageExtension) DecodeMetrics(_ context.Context, payload []byte, options ...encoding.MetricsDecodeOption) (pmetric.Metrics, error) {
	cfg, err := encoding.NewDecodeConfig(options...)
	if err != nil {
		return pmetric.Metrics{}, err
	}
	if values := cfg.MessageMetadata.Get("tenant"); len(values) != 1 || values[0] != "benchmark" {
		return pmetric.Metrics{}, errors.New("missing benchmark tenant metadata")
	}
	if e.unmarshaler == nil {
		return e.fixture, nil
	}
	return e.unmarshaler.UnmarshalMetrics(payload)
}
