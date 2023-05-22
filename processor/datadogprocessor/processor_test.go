// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadogprocessor"

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/pkg/trace/pb"
	"github.com/DataDog/sketches-go/ddsketch"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

func TestProcessorStart(t *testing.T) {
	ctx := context.Background()

	t.Run("fail", func(t *testing.T) {
		p, err := newProcessor(ctx, zap.NewNop(), createDefaultConfig(), &mockTracesConsumer{})
		require.NoError(t, err)
		defer p.Shutdown(ctx) //nolint:errcheck
		require.True(t, p.Capabilities().MutatesData)
		err = p.Start(ctx, &mockHost{
			Exporters: exporters(map[string]exporter.Metrics{
				"test-exporter": &mockMetricsExporter{},
			}),
		})
		require.ErrorContains(t, err, `failed to find metrics exporter "datadog"`)
	})

	t.Run("fail/2", func(t *testing.T) {
		p, err := newProcessor(ctx, zap.NewNop(), createDefaultConfig(), &mockTracesConsumer{})
		require.NoError(t, err)
		defer p.Shutdown(ctx) //nolint:errcheck
		err = p.Start(ctx, &mockHost{
			Exporters: exporters(map[string]exporter.Metrics{
				"test-exporter": &mockMetricsExporter{},
				"datadog/1":     &mockMetricsExporter{},
				"datadog/2":     &mockMetricsExporter{},
			}),
		})
		require.ErrorContains(t, err, `too many exporters of type "datadog"`)
	})

	t.Run("succeed/0", func(t *testing.T) {
		p, err := newProcessor(ctx, zap.NewNop(), createDefaultConfig(), &mockTracesConsumer{})
		require.NoError(t, err)
		defer p.Shutdown(ctx) //nolint:errcheck
		err = p.Start(ctx, &mockHost{
			Exporters: exporters(map[string]exporter.Metrics{
				"test-exporter": &mockMetricsExporter{},
				"datadog":       &mockMetricsExporter{},
			}),
		})
		require.NoError(t, err)
	})

	t.Run("succeed/1", func(t *testing.T) {
		p, err := newProcessor(ctx, zap.NewNop(), createDefaultConfig(), &mockTracesConsumer{})
		require.NoError(t, err)
		defer p.Shutdown(ctx) //nolint:errcheck
		err = p.Start(ctx, &mockHost{
			Exporters: exporters(map[string]exporter.Metrics{
				"test-exporter": &mockMetricsExporter{},
				"datadog/2":     &mockMetricsExporter{},
			}),
		})
		require.NoError(t, err)
	})

	t.Run("succeed/2", func(t *testing.T) {
		lp, err := newProcessor(ctx, zap.NewNop(), &Config{MetricsExporter: component.NewIDWithName("datadog", "2")}, &mockTracesConsumer{})
		require.NoError(t, err)
		defer lp.Shutdown(ctx) //nolint:errcheck
		err = lp.Start(ctx, &mockHost{
			Exporters: exporters(map[string]exporter.Metrics{
				"test-exporter": &mockMetricsExporter{},
				"datadog/1":     &mockMetricsExporter{},
				"datadog/2":     &mockMetricsExporter{},
			}),
		})
		require.NoError(t, err)
	})
}

func TestProcessorIngest(t *testing.T) {
	var mockConsumer mockTracesConsumer
	ctx := context.Background()
	p, err := newProcessor(ctx, zap.NewNop(), createDefaultConfig(), &mockConsumer)
	require.NoError(t, err)
	out := make(chan pb.StatsPayload, 1)
	ing := &mockIngester{Out: out}
	p.agent = ing
	p.in = out
	mexporter := &mockMetricsExporter{}
	err = p.Start(ctx, &mockHost{
		Exporters: exporters(map[string]exporter.Metrics{
			"test-exporter": &mockMetricsExporter{},
			"datadog":       mexporter,
		}),
	})
	require.NoError(t, err)
	defer p.Shutdown(ctx) //nolint:errcheck
	tracesin := ptrace.NewTraces()
	err = p.ConsumeTraces(ctx, tracesin)
	require.NoError(t, err)
	require.Equal(t, ing.ingested, tracesin)         // ingester has ingested the consumed traces
	require.Equal(t, mockConsumer.In()[0], tracesin) // the consumed traces are sent to the next consumer
	timeout := time.After(time.Second)
loop:
	for {
		select {
		case <-timeout:
			t.Fatal("metrics exporter should have received metrics")
		default:
			if len(mexporter.In()) > 0 {
				break loop // the exporter has consumed the generated metrics
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func exporters(names map[string]exporter.Metrics) map[component.DataType]map[component.ID]component.Component {
	out := map[component.DataType]map[component.ID]component.Component{
		component.DataTypeTraces: {
			component.NewID("trace-exporter-a"): &mockTracesExporter{},
			component.NewID("trace-exporter-b"): &mockTracesExporter{},
		},
	}
	exps := make(map[component.ID]component.Component)
	for name, exp := range names {
		if strings.Contains(name, "/") {
			p := strings.Split(name, "/")
			exps[component.NewIDWithName(component.Type(p[0]), p[1])] = exp
		} else {
			exps[component.NewID(component.Type(name))] = exp
		}
	}
	out[component.DataTypeMetrics] = exps
	return out
}

var _ consumer.Traces = (*mockTracesConsumer)(nil)

// mockTracesConsumer implements consumer.Traces.
type mockTracesConsumer struct {
	mu sync.RWMutex
	in []ptrace.Traces
}

func (m *mockTracesConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (m *mockTracesConsumer) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	m.mu.Lock()
	m.in = append(m.in, td)
	m.mu.Unlock()
	return nil
}

func (m *mockTracesConsumer) In() []ptrace.Traces {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.in
}

var _ consumer.Metrics = (*mockMetricsConsumer)(nil)

// mockMetricsConsumer implements consumer.Traces.
type mockMetricsConsumer struct {
	mu sync.RWMutex
	in []pmetric.Metrics
}

func (m *mockMetricsConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (m *mockMetricsConsumer) ConsumeMetrics(_ context.Context, td pmetric.Metrics) error {
	m.mu.Lock()
	m.in = append(m.in, td)
	m.mu.Unlock()
	return nil
}

func (m *mockMetricsConsumer) In() []pmetric.Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.in
}

var _ component.Host = (*mockHost)(nil)

// mockHost implements component.Host.
type mockHost struct {
	Exporters map[component.DataType]map[component.ID]component.Component
}

func (m *mockHost) ReportFatalError(err error) {}

func (m *mockHost) GetFactory(kind component.Kind, componentType component.Type) component.Factory {
	return nil
}

func (m *mockHost) GetExtensions() map[component.ID]extension.Extension {
	return nil
}

func (m *mockHost) GetExporters() map[component.DataType]map[component.ID]component.Component {
	return m.Exporters
}

var _ component.Component = (*mockComponent)(nil)

// mockComponent implements component.Component.
type mockComponent struct{}

func (m *mockComponent) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (m *mockComponent) Shutdown(_ context.Context) error {
	return nil
}

var _ exporter.Metrics = (*mockMetricsExporter)(nil)

// mockMetricsExporter implements exporter.Metrics.
type mockMetricsExporter struct {
	mockComponent
	mockMetricsConsumer
}

var _ exporter.Traces = (*mockTracesExporter)(nil)

// mockTracesExporter implements exporter.Traces.
type mockTracesExporter struct {
	mockComponent
	mockTracesConsumer
}

var _ ingester = (*mockIngester)(nil)

// mockIngester implements ingester.
type mockIngester struct {
	Out         chan pb.StatsPayload
	start, stop bool
	ingested    ptrace.Traces
}

// Start starts the ingester.
func (m *mockIngester) Start() {
	m.start = true
}

// Ingest ingests the set of traces.
func (m *mockIngester) Ingest(_ context.Context, traces ptrace.Traces) {
	m.ingested = traces
	m.Out <- testStatsPayload
}

// Stop stops the ingester.
func (m *mockIngester) Stop() {
	m.stop = true
}

// TODO(gbbr): this is copied from exporter/datadogexporter/internal/testutil.
// We should find a way to have a shared set of test utilities in either the processor
// or the exporter.
var testStatsPayload = pb.StatsPayload{
	Stats: []pb.ClientStatsPayload{
		{
			Hostname:         "host",
			Env:              "prod",
			Version:          "v1.2",
			Lang:             "go",
			TracerVersion:    "v44",
			RuntimeID:        "123jkl",
			Sequence:         2,
			AgentAggregation: "blah",
			Service:          "mysql",
			ContainerID:      "abcdef123456",
			Tags:             []string{"a:b", "c:d"},
			Stats: []pb.ClientStatsBucket{
				{
					Start:    10,
					Duration: 1,
					Stats: []pb.ClientGroupedStats{
						{
							Service:        "kafka",
							Name:           "queue.add",
							Resource:       "append",
							HTTPStatusCode: 220,
							Type:           "queue",
							Hits:           15,
							Errors:         3,
							Duration:       143,
							OkSummary:      testSketchBytes(1, 2, 3),
							ErrorSummary:   testSketchBytes(4, 5, 6),
							TopLevelHits:   5,
						},
					},
				},
			},
		},
		{
			Hostname:         "host2",
			Env:              "prod2",
			Version:          "v1.22",
			Lang:             "go2",
			TracerVersion:    "v442",
			RuntimeID:        "123jkl2",
			Sequence:         22,
			AgentAggregation: "blah2",
			Service:          "mysql2",
			ContainerID:      "abcdef1234562",
			Tags:             []string{"a:b2", "c:d2"},
			Stats: []pb.ClientStatsBucket{
				{
					Start:    102,
					Duration: 12,
					Stats: []pb.ClientGroupedStats{
						{
							Service:        "kafka2",
							Name:           "queue.add2",
							Resource:       "append2",
							HTTPStatusCode: 2202,
							Type:           "queue2",
							Hits:           152,
							Errors:         32,
							Duration:       1432,
							OkSummary:      testSketchBytes(7, 8),
							ErrorSummary:   testSketchBytes(9, 10, 11),
							TopLevelHits:   52,
						},
					},
				},
			},
		},
	},
}

// The sketch's relative accuracy and maximum number of bins is identical
// to the one used in the trace-agent for consistency:
// https://github.com/DataDog/datadog-agent/blob/cbac965/pkg/trace/stats/statsraw.go#L18-L26
const (
	sketchRelativeAccuracy = 0.01
	sketchMaxBins          = 2048
)

// testSketchBytes returns the proto-encoded version of a DDSketch containing the
// points in nums.
func testSketchBytes(nums ...float64) []byte {
	sketch, err := ddsketch.LogCollapsingLowestDenseDDSketch(sketchRelativeAccuracy, sketchMaxBins)
	if err != nil {
		// the only possible error is if the relative accuracy is < 0 or > 1;
		// we know that's not the case because it's a constant defined as 0.01
		panic(err)
	}
	for _, num := range nums {
		if err2 := sketch.Add(num); err2 != nil {
			panic(err2) // invalid input
		}
	}
	buf, err := proto.Marshal(sketch.ToProto())
	if err != nil {
		// there should be no error under any circumstances here
		panic(err)
	}
	return buf
}
