package natscoreexporter

// import (
// 	"context"
// 	"testing"
// 	"time"

// 	"github.com/stretchr/testify/assert"
// 	"go.opentelemetry.io/collector/component"
// 	"go.opentelemetry.io/collector/component/componenttest"
// 	"go.opentelemetry.io/collector/exporter/exportertest"

// 	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/grouper"
// 	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/marshaler"
// 	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/metadata"
// )

// type fakeGrouper struct{}

// func (g *fakeGrouper) Group(ctx context.Context, data string) ([]grouper.Group[string], error) {
// 	return []grouper.Group[string]{{Subject: data, Data: data}}, nil
// }

// var _ grouper.Grouper[string] = (*fakeGrouper)(nil)

// type fakeGenericMarshaler struct{}

// func (m *fakeGenericMarshaler) MarshalString(sd string) ([]byte, error) {
// 	return []byte(sd), nil
// }

// var _ marshaler.GenericMarshaler = (*fakeGenericMarshaler)(nil)

// type fakeResolver struct{}

// func (r *fakeResolver) Resolve(host component.Host) (marshaler.GenericMarshaler, error) {
// 	return &fakeGenericMarshaler{}, nil
// }

// var _ marshaler.Resolver = (*fakeResolver)(nil)

// func fakePick(genericMarshaler marshaler.GenericMarshaler) (marshaler.MarshalFunc[string], error) {
// 	return genericMarshaler.(*fakeGenericMarshaler).MarshalString, nil
// }

// var _ marshaler.PickFunc[string] = fakePick

// func newNatsCoreExporterWithFakes(cfg *Config) *natsCoreExporter[string] {
// 	set := exportertest.NewNopSettings(metadata.Type)
// 	grouper := &fakeGrouper{}
// 	resolver := &fakeResolver{}
// 	marshaler := marshaler.NewMarshaler(resolver, fakePick)
// 	return &natsCoreExporter[string]{
// 		set:       set,
// 		cfg:       cfg,
// 		grouper:   grouper,
// 		marshaler: marshaler,
// 	}
// }

// func TestNatsCoreExporter(t *testing.T) {
// 	t.Parallel()

// 	withTestServer(t, func(cfg *Config, recorder *recorder) {
// 		exporter := newNatsCoreExporterWithFakes(cfg)

// 		err := exporter.start(t.Context(), componenttest.NewNopHost())
// 		assert.NoError(t, err)

// 		err = exporter.export(t.Context(), "test")
// 		assert.NoError(t, err)
// 		err = exporter.export(t.Context(), "test")
// 		assert.NoError(t, err)

// 		err = exporter.shutdown(t.Context())
// 		assert.NoError(t, err)

// 		msgs := recorder.record(2, 5*time.Second)
// 		for _, msg := range msgs {
// 			t.Logf("%s: %s", msg.Subject, string(msg.Data))
// 		}
// 	})
// }

// func TestNewNatsCoreLogsExporter(t *testing.T) {
// 	t.Parallel()

// 	withTestServer(t, func(cfg *Config, recorder *recorder) {
// 		set := exportertest.NewNopSettings(metadata.Type)
// 		exporter, err := newNatsCoreLogsExporter(set, cfg)
// 		assert.NoError(t, err)

// 		err = exporter.start(t.Context(), componenttest.NewNopHost())
// 		assert.NoError(t, err)

// 		err = exporter.export(t.Context(), generateLifecycleTestLogs())
// 		assert.NoError(t, err)
// 		err = exporter.export(t.Context(), generateLifecycleTestLogs())
// 		assert.NoError(t, err)

// 		err = exporter.shutdown(t.Context())
// 		assert.NoError(t, err)

// 		msgs := recorder.record(2, 5*time.Second)
// 		for _, msg := range msgs {
// 			t.Logf("%s: %s", msg.Subject, string(msg.Data))
// 		}
// 	})
// }

// func TestNewNatsCoreMetricsExporter(t *testing.T) {
// 	t.Parallel()

// 	withTestServer(t, func(cfg *Config, recorder *recorder) {
// 		set := exportertest.NewNopSettings(metadata.Type)
// 		exporter, err := newNatsCoreMetricsExporter(set, cfg)
// 		assert.NoError(t, err)

// 		err = exporter.start(t.Context(), componenttest.NewNopHost())
// 		assert.NoError(t, err)

// 		err = exporter.export(t.Context(), generateLifecycleTestMetrics())
// 		assert.NoError(t, err)
// 		err = exporter.export(t.Context(), generateLifecycleTestMetrics())
// 		assert.NoError(t, err)

// 		err = exporter.shutdown(t.Context())
// 		assert.NoError(t, err)

// 		msgs := recorder.record(2, 5*time.Second)
// 		for _, msg := range msgs {
// 			t.Logf("%s: %s", msg.Subject, string(msg.Data))
// 		}
// 	})
// }

// func TestNewNatsCoreTracesExporter(t *testing.T) {
// 	t.Parallel()

// 	withTestServer(t, func(cfg *Config, recorder *recorder) {
// 		set := exportertest.NewNopSettings(metadata.Type)
// 		exporter, err := newNatsCoreTracesExporter(set, cfg)
// 		assert.NoError(t, err)

// 		err = exporter.start(t.Context(), componenttest.NewNopHost())
// 		assert.NoError(t, err)

// 		err = exporter.export(t.Context(), generateLifecycleTestTraces())
// 		assert.NoError(t, err)
// 		err = exporter.export(t.Context(), generateLifecycleTestTraces())
// 		assert.NoError(t, err)

// 		err = exporter.shutdown(t.Context())
// 		assert.NoError(t, err)

// 		msgs := recorder.record(2, 5*time.Second)
// 		for _, msg := range msgs {
// 			t.Logf("%s: %s", msg.Subject, string(msg.Data))
// 		}
// 	})
// }

// func TestSetNatsNkeyOption(t *testing.T) {
// 	t.Parallel()

// 	withTestServer(t, func(cfg *Config, recorder *recorder) {
// 		exporter := newNatsCoreExporterWithFakes(cfg)

// 		err := exporter.start(t.Context(), componenttest.NewNopHost())
// 		assert.NoError(t, err)

// 		err = exporter.shutdown(t.Context())
// 		assert.NoError(t, err)
// 	})
// }

// func TestSetNatsNkeyJWTOption(t *testing.T) {
// 	t.Parallel()

// 	withTestServer(t, func(cfg *Config, recorder *recorder) {
// 		exporter := newNatsCoreExporterWithFakes(cfg)

// 		err := exporter.start(t.Context(), componenttest.NewNopHost())
// 		assert.NoError(t, err)

// 		err = exporter.shutdown(t.Context())
// 		assert.NoError(t, err)
// 	})
// }

// func TestSetNatsNkeyUserFileOption(t *testing.T) {
// 	t.Parallel()

// 	withTestServer(t, func(cfg *Config, recorder *recorder) {
// 		exporter := newNatsCoreExporterWithFakes(cfg)

// 		err := exporter.start(t.Context(), componenttest.NewNopHost())
// 		assert.NoError(t, err)

// 		err = exporter.shutdown(t.Context())
// 		assert.NoError(t, err)
// 	})
// }
