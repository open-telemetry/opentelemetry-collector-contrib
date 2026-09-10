// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/xexporter"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metadata"
)

// pqTestHost is a component.Host exposing the storage extension the persistent
// queue looks up.
type pqTestHost struct {
	ext map[component.ID]component.Component
}

func (h *pqTestHost) GetExtensions() map[component.ID]component.Component {
	return h.ext
}

// pqStorageExtension is a minimal in-memory storage.Extension. Configuring
// sending_queue::storage selects exporterhelper's persistent-queue
// implementation, which marshals every request to bytes on Offer and
// unmarshals on consume regardless of what backs the storage client — so this
// exercises the exporter's serialization round-trip without touching disk,
// the same technique exporterhelper's own persistent-queue tests use.
type pqStorageExtension struct {
	component.StartFunc
	component.ShutdownFunc
}

func (*pqStorageExtension) GetClient(context.Context, component.Kind, component.ID, string) (storage.Client, error) {
	return &pqStorageClient{data: make(map[string][]byte)}, nil
}

type pqStorageClient struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (c *pqStorageClient) Get(_ context.Context, key string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.data[key], nil
}

func (c *pqStorageClient) Set(_ context.Context, key string, value []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
	return nil
}

func (c *pqStorageClient) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
	return nil
}

func (c *pqStorageClient) Batch(_ context.Context, ops ...*storage.Operation) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, op := range ops {
		switch op.Type {
		case storage.Get:
			op.Value = c.data[op.Key]
		case storage.Set:
			c.data[op.Key] = op.Value
		case storage.Delete:
			delete(c.data, op.Key)
		}
	}
	return nil
}

func (*pqStorageClient) Close(context.Context) error { return nil }

var (
	_ storage.Extension = (*pqStorageExtension)(nil)
	_ storage.Client    = (*pqStorageClient)(nil)
	_ component.Host    = (*pqTestHost)(nil)
)

// newPQDeliveryTest builds a config with a persistent sending queue
// (sending_queue.storage), a bulk-recording server, and a host exposing the
// storage extension.
func newPQDeliveryTest(t *testing.T) (*Config, component.Host, *bulkRecorder) {
	rec := newBulkRecorder()
	server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
		rec.Record(docs)
		return itemsAllOK(docs)
	})

	storageID := component.MustNewID("file_storage")
	cfg := withDefaultConfig(func(cfg *Config) {
		cfg.Endpoints = []string{server.URL}
		cfg.QueueBatchConfig.Get().NumConsumers = 1
		cfg.QueueBatchConfig.Get().Batch.Get().FlushTimeout = 10 * time.Millisecond
		cfg.QueueBatchConfig.Get().StorageID = &storageID
	})

	host := &pqTestHost{
		ext: map[component.ID]component.Component{
			storageID: &pqStorageExtension{},
		},
	}
	return cfg, host, rec
}

func pqLogs(n int) plog.Logs {
	logs := plog.NewLogs()
	sl := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
	for range n {
		lr := sl.LogRecords().AppendEmpty()
		lr.Body().SetStr("persistent queue delivery")
		lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1719000000, 0)))
	}
	return logs
}

func pqTraces(n int) ptrace.Traces {
	traces := ptrace.NewTraces()
	ss := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
	for range n {
		span := ss.Spans().AppendEmpty()
		span.SetName("span")
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1719000000, 0)))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(1719000001, 0)))
	}
	return traces
}

func pqProfiles() pprofile.Profiles {
	profiles := pprofile.NewProfiles()
	dic := profiles.Dictionary()
	profile := profiles.ResourceProfiles().AppendEmpty().ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()

	dic.StringTable().Append("samples", "count", "cpu", "nanoseconds")
	st := profile.SampleType()
	st.SetTypeStrindex(0)
	st.SetUnitStrindex(1)
	pt := profile.PeriodType()
	pt.SetTypeStrindex(2)
	pt.SetUnitStrindex(3)

	a := dic.AttributeTable().AppendEmpty()
	a.SetKeyStrindex(4)
	dic.StringTable().Append("process.executable.build_id.htlhash")
	a.Value().SetStr("600DCAFE4A110000F2BF38C493F5FB92")
	a = dic.AttributeTable().AppendEmpty()
	a.SetKeyStrindex(5)
	dic.StringTable().Append("profile.frame.type")
	a.Value().SetStr("native")
	a = dic.AttributeTable().AppendEmpty()
	a.SetKeyStrindex(6)
	dic.StringTable().Append("host.id")
	a.Value().SetStr("localhost")

	profile.AttributeIndices().Append(2)

	sample := profile.Samples().AppendEmpty()
	sample.TimestampsUnixNano().Append(0)

	stack := dic.StackTable().AppendEmpty()
	stack.LocationIndices().Append(0)

	m := dic.MappingTable().AppendEmpty()
	m.AttributeIndices().Append(0)

	l := dic.LocationTable().AppendEmpty()
	l.SetMappingIndex(0)
	l.SetAddress(111)
	l.AttributeIndices().Append(1)

	return profiles
}

// TestPersistentQueueDelivery pins end-to-end delivery through a persistent
// sending queue for every signal: documents offered to a storage-backed queue
// must round-trip through the queue's marshaling and reach Elasticsearch.
func TestPersistentQueueDelivery(t *testing.T) {
	f := NewFactory()
	set := exportertest.NewNopSettings(metadata.Type)

	t.Run("logs", func(t *testing.T) {
		cfg, host, rec := newPQDeliveryTest(t)
		exp, err := f.CreateLogs(t.Context(), set, cfg)
		require.NoError(t, err)
		require.NoError(t, exp.Start(t.Context(), host))
		t.Cleanup(func() { require.NoError(t, exp.Shutdown(context.WithoutCancel(t.Context()))) })
		require.NoError(t, exp.ConsumeLogs(t.Context(), pqLogs(3)))
		rec.WaitItems(3)
	})

	t.Run("metrics", func(t *testing.T) {
		cfg, host, rec := newPQDeliveryTest(t)
		exp, err := f.CreateMetrics(t.Context(), set, cfg)
		require.NoError(t, err)
		require.NoError(t, exp.Start(t.Context(), host))
		t.Cleanup(func() { require.NoError(t, exp.Shutdown(context.WithoutCancel(t.Context()))) })
		require.NoError(t, exp.ConsumeMetrics(t.Context(),
			groupingGauges("", time.Unix(1719000000, 0).UTC(), []string{"m.a", "m.b"}, []float64{1, 2})))
		rec.WaitItems(1)
	})

	t.Run("traces", func(t *testing.T) {
		cfg, host, rec := newPQDeliveryTest(t)
		exp, err := f.CreateTraces(t.Context(), set, cfg)
		require.NoError(t, err)
		require.NoError(t, exp.Start(t.Context(), host))
		t.Cleanup(func() { require.NoError(t, exp.Shutdown(context.WithoutCancel(t.Context()))) })
		require.NoError(t, exp.ConsumeTraces(t.Context(), pqTraces(2)))
		rec.WaitItems(2)
	})

	t.Run("profiles", func(t *testing.T) {
		cfg, host, rec := newPQDeliveryTest(t)
		exp, err := f.(xexporter.Factory).CreateProfiles(t.Context(), set, cfg)
		require.NoError(t, err)
		require.NoError(t, exp.Start(t.Context(), host))
		t.Cleanup(func() { require.NoError(t, exp.Shutdown(context.WithoutCancel(t.Context()))) })
		require.NoError(t, exp.ConsumeProfiles(t.Context(), pqProfiles()))
		rec.WaitItems(1)
	})
}
