// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package partitioningprocessor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"
)

// TestIntegration_SendingQueueMetadataKeys chains the partitioning processor
// into an exporter whose sending queue batches by
// batch.partition.metadata_keys, and checks that every exported batch holds
// a single tenant, with the tenant carried in the request metadata.
func TestIntegration_SendingQueueMetadataKeys(t *testing.T) {
	ctx := t.Context()

	var (
		mu       sync.Mutex
		exported []plog.Logs
		ctxs     []context.Context
	)
	push := func(ctx context.Context, ld plog.Logs) error {
		mu.Lock()
		defer mu.Unlock()
		exported = append(exported, ld)
		ctxs = append(ctxs, ctx)
		return nil
	}

	qCfg := exporterhelper.NewDefaultQueueConfig()
	batchCfg := exporterhelper.NewDefaultBatchConfig()
	batchCfg.FlushTimeout = time.Hour // flush only on shutdown, so batch counts are deterministic
	batchCfg.Sizer = exporterhelper.RequestSizerTypeItems
	batchCfg.MinSize = 10000
	batchCfg.Partition.MetadataKeys = []string{"tenant_id"}
	qCfg.Batch = configoptional.Some(batchCfg)

	exp, err := exporterhelper.NewLogs(ctx, exportertest.NewNopSettings(component.MustNewType("test")), &struct{}{}, push,
		exporterhelper.WithQueue(configoptional.Some(qCfg)))
	require.NoError(t, err)
	require.NoError(t, exp.Start(ctx, componenttest.NewNopHost()))

	part, err := NewFactory().CreateLogs(ctx, processortest.NewNopSettings(NewFactory().Type()), &Config{
		Keys: map[string]string{"tenant_id": `log.attributes["tenant"]`},
	}, exp)
	require.NoError(t, err)
	require.NoError(t, part.Start(ctx, componenttest.NewNopHost()))

	// Two inbound requests with interleaved tenants; "" means no attribute.
	for _, tenants := range [][]string{{"t1", "t2", "t1", ""}, {"t2", "t3", "t1"}} {
		ld := plog.NewLogs()
		lrs := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		for _, tenant := range tenants {
			lr := lrs.AppendEmpty()
			if tenant != "" {
				lr.Attributes().PutStr("tenant", tenant)
			}
		}
		require.NoError(t, part.ConsumeLogs(ctx, ld))
	}

	require.NoError(t, part.Shutdown(ctx))
	require.NoError(t, exp.Shutdown(ctx)) // drains the queue and flushes pending batches

	mu.Lock()
	defer mu.Unlock()
	got := make(map[string]int) // tenant -> record count
	for i, ld := range exported {
		md := client.FromContext(ctxs[i]).Metadata.Get("tenant_id")
		tenant := ""
		if len(md) > 0 {
			require.Len(t, md, 1)
			tenant = md[0]
		}
		_, dup := got[tenant]
		require.Falsef(t, dup, "tenant %q emitted in more than one batch", tenant)

		for _, rl := range ld.ResourceLogs().All() {
			for _, sl := range rl.ScopeLogs().All() {
				for _, lr := range sl.LogRecords().All() {
					v, ok := lr.Attributes().Get("tenant")
					if tenant == "" {
						assert.False(t, ok, "record with a tenant ended up in the no-tenant batch")
					} else {
						assert.Equal(t, tenant, v.Str())
					}
					got[tenant]++
				}
			}
		}
	}
	assert.Equal(t, map[string]int{"t1": 3, "t2": 2, "t3": 1, "": 1}, got)
}
