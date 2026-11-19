// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package partitioningprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
)

// --- parser tests ---

func TestNewLogsPartitioner_ResourceContext(t *testing.T) {
	expressions := []string{
		`resource.attributes["tenant.id"]`,
		`resource.attributes["service.name"]`,
	}
	p, err := newLogsPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	_, ok := p.(*resourceLogsPartitioner)
	assert.True(t, ok, "expected resourceLogsPartitioner when all expressions use resource context")
}

func TestNewLogsPartitioner_LogContext(t *testing.T) {
	expressions := []string{`log.severity_text`}
	p, err := newLogsPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	_, ok := p.(*logRecordPartitioner)
	assert.True(t, ok, "expected logRecordPartitioner when expression uses log context")
}

func TestNewLogsPartitioner_OTelColContext(t *testing.T) {
	expressions := []string{`otelcol.client.metadata["x-tenant-id"][0]`}
	p, err := newLogsPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	_, ok := p.(*otelcolLogsPartitioner)
	assert.True(t, ok, "expected otelcolLogsPartitioner when expression uses otelcol context")
}

func TestNewLogsPartitioner_ScopeContext(t *testing.T) {
	expressions := []string{`scope.name`}
	p, err := newLogsPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	_, ok := p.(*scopeLogsPartitioner)
	assert.True(t, ok, "expected scopeLogsPartitioner when expression uses scope context")
}

func TestNewLogsPartitioner_ScopeAndResourceContext(t *testing.T) {
	expressions := []string{
		`resource.attributes["tenant.id"]`,
		`scope.name`,
	}
	p, err := newLogsPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	_, ok := p.(*scopeLogsPartitioner)
	assert.True(t, ok, "expected scopeLogsPartitioner when mixed resource+scope contexts used")
}

func TestNewLogsPartitioner_MixedContext(t *testing.T) {
	expressions := []string{
		`resource.attributes["tenant.id"]`,
		`log.severity_text`,
	}
	p, err := newLogsPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	_, ok := p.(*logRecordPartitioner)
	assert.True(t, ok, "expected logRecordPartitioner when mixed contexts used")
}

func TestNewLogsPartitioner_InvalidExpression(t *testing.T) {
	expressions := []string{`not_a_valid_expression(`}
	_, err := newLogsPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	assert.Error(t, err)
}

func TestConsumeLogs_OTelColContext_ClientMetadata(t *testing.T) {
	var gotMD client.Metadata
	next := &capturingLogsConsumer{fn: func(ctx context.Context, _ plog.Logs) error {
		gotMD = client.FromContext(ctx).Metadata
		return nil
	}}

	proc := buildLogsProcessor(t, &Config{Keys: map[string]string{
		"logs_topic": `otelcol.client.metadata["x-tenant-id"][0]`,
	}}, next)

	ctx := client.NewContext(t.Context(), client.Info{
		Metadata: client.NewMetadata(map[string][]string{
			"x-tenant-id": {"acme"},
		}),
	})

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	require.NoError(t, proc.ConsumeLogs(ctx, logs))
	assert.Equal(t, []string{"acme"}, gotMD.Get("x-tenant-id"), "incoming metadata should be preserved")
	assert.Equal(t, []string{"acme"}, gotMD.Get("logs_topic"), "partition key should be derived from otelcol client metadata")
}

// --- partitioner tests ---

func TestPartitionKey(t *testing.T) {
	kv := func(v string) keyValue { return keyValue{value: v} }
	kvNil := keyValue{isNil: true}
	partitionKey := func(v []keyValue) string { return string(appendPartitionKey(nil, v)) }

	assert.Equal(t, "11:111:2", partitionKey([]keyValue{kv("1"), kv("2")}))
	assert.Equal(t, "11:y", partitionKey([]keyValue{kv("y")}))
	assert.Equal(t, "0", partitionKey([]keyValue{kvNil}))
	assert.Empty(t, partitionKey(nil))

	// nil and non-nil are distinct.
	assert.NotEqual(t,
		partitionKey([]keyValue{kvNil}),
		partitionKey([]keyValue{kv("")}),
	)
	// Length prefixing avoids collisions between distinct value sequences.
	assert.NotEqual(t,
		partitionKey([]keyValue{kv("b"), kv("")}),
		partitionKey([]keyValue{kv("bc")}),
	)
	assert.NotEqual(t,
		partitionKey([]keyValue{kv("a"), kv("bc")}),
		partitionKey([]keyValue{kv("ab"), kv("c")}),
	)
}

func TestPartitionLogs_SinglePartition(t *testing.T) {
	logs := plog.NewLogs()
	rl1 := logs.ResourceLogs().AppendEmpty()
	rl1.Resource().Attributes().PutStr("tenant.id", "t1")
	rl1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log1")

	rl2 := logs.ResourceLogs().AppendEmpty()
	rl2.Resource().Attributes().PutStr("tenant.id", "t1")
	rl2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log2")

	result := partitionLogs(t, logs,
		`resource.attributes["tenant.id"]`,
	)
	assert.Len(t, result, 1)
	assert.Equal(t, 2, result[0].data.ResourceLogs().Len())
}

func TestPartitionLogs_MultiplePartitions(t *testing.T) {
	logs := plog.NewLogs()
	rl1 := logs.ResourceLogs().AppendEmpty()
	rl1.Resource().Attributes().PutStr("tenant.id", "t1")
	rl1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log1")

	rl2 := logs.ResourceLogs().AppendEmpty()
	rl2.Resource().Attributes().PutStr("tenant.id", "t2")
	rl2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log2")

	result := partitionLogs(t, logs,
		`resource.attributes["tenant.id"]`,
	)
	require.Len(t, result, 2)

	tenants := make(map[string]bool)
	for _, pl := range result {
		require.Len(t, pl.values, 1)
		tenants[pl.values[0].value] = true
	}
	assert.True(t, tenants["t1"])
	assert.True(t, tenants["t2"])
}

func TestPartitionLogs_NilAttributeValue(t *testing.T) {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("other", "value")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log1")

	result := partitionLogs(t, logs,
		`resource.attributes["missing"]`,
	)
	require.Len(t, result, 1)
	require.Len(t, result[0].values, 1)
	assert.True(t, result[0].values[0].isNil)
}

func TestPartitionLogs_MultipleKeys(t *testing.T) {
	logs := plog.NewLogs()
	rl1 := logs.ResourceLogs().AppendEmpty()
	rl1.Resource().Attributes().PutStr("tenant.id", "t1")
	rl1.Resource().Attributes().PutStr("service.name", "svc-a")
	rl1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log1")

	rl2 := logs.ResourceLogs().AppendEmpty()
	rl2.Resource().Attributes().PutStr("tenant.id", "t1")
	rl2.Resource().Attributes().PutStr("service.name", "svc-b")
	rl2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log2")

	result := partitionLogs(t, logs,
		`resource.attributes["tenant.id"]`,
		`resource.attributes["service.name"]`,
	)
	assert.Len(t, result, 2)
}

func TestPartitionLogs_LogLevelPartitioning(t *testing.T) {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test")
	sl := rl.ScopeLogs().AppendEmpty()

	lr1 := sl.LogRecords().AppendEmpty()
	lr1.Body().SetStr("error log")
	lr1.SetSeverityText("ERROR")

	lr2 := sl.LogRecords().AppendEmpty()
	lr2.Body().SetStr("info log")
	lr2.SetSeverityText("INFO")

	lr3 := sl.LogRecords().AppendEmpty()
	lr3.Body().SetStr("another error")
	lr3.SetSeverityText("ERROR")

	result := partitionLogs(t, logs,
		`log.severity_text`,
	)
	require.Len(t, result, 2)

	severities := make(map[string]int)
	for _, pl := range result {
		require.Len(t, pl.values, 1)
		severities[pl.values[0].value] = pl.data.LogRecordCount()
	}
	assert.Equal(t, 2, severities["ERROR"])
	assert.Equal(t, 1, severities["INFO"])
}

func TestPartitionLogs_ScopePartitioning(t *testing.T) {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test")

	sl1 := rl.ScopeLogs().AppendEmpty()
	sl1.Scope().SetName("scope-a")
	sl1.LogRecords().AppendEmpty().Body().SetStr("log1")
	sl1.LogRecords().AppendEmpty().Body().SetStr("log2")

	sl2 := rl.ScopeLogs().AppendEmpty()
	sl2.Scope().SetName("scope-b")
	sl2.LogRecords().AppendEmpty().Body().SetStr("log3")

	result := partitionLogs(t, logs,
		`scope.name`,
	)
	require.Len(t, result, 2)

	scopes := make(map[string]int)
	for _, pl := range result {
		require.Len(t, pl.values, 1)
		scopes[pl.values[0].value] = pl.data.LogRecordCount()
	}
	assert.Equal(t, 2, scopes["scope-a"])
	assert.Equal(t, 1, scopes["scope-b"])
}

func TestPartitionLogs_NonStringValueErrors(t *testing.T) {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutInt("count", 42)
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log1")

	p, err := newLogsPartitioner([]string{`resource.attributes["count"]`}, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	_, err = p.partitionLogs(t.Context(), logs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected value expression to evaluate to a string")
}

func TestPartitionLogs_KeyOrderInvariance(t *testing.T) {
	build := func() plog.Logs {
		logs := plog.NewLogs()
		for _, tenant := range []string{"t1", "t2", "t1"} {
			for _, svc := range []string{"svc-a", "svc-b"} {
				rl := logs.ResourceLogs().AppendEmpty()
				rl.Resource().Attributes().PutStr("tenant.id", tenant)
				rl.Resource().Attributes().PutStr("service.name", svc)
				rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("x")
			}
		}
		return logs
	}

	_, tenantFirstExpressions := sortedPartitionKeys(map[string]string{
		"tenant_id": `resource.attributes["tenant.id"]`,
		"service":   `resource.attributes["service.name"]`,
	})
	tenantFirst := partitionLogs(t, build(), tenantFirstExpressions...)

	_, serviceFirstExpressions := sortedPartitionKeys(map[string]string{
		"service":   `resource.attributes["service.name"]`,
		"tenant_id": `resource.attributes["tenant.id"]`,
	})
	serviceFirst := partitionLogs(t, build(), serviceFirstExpressions...)

	// Sort order is [service, tenant_id] regardless of input order.
	summarize := func(parts []partitionedLogs) map[string]int {
		out := make(map[string]int)
		for _, pl := range parts {
			out[pl.values[1].value+"|"+pl.values[0].value] = pl.data.LogRecordCount()
		}
		return out
	}
	assert.Equal(t, summarize(tenantFirst), summarize(serviceFirst))
	assert.Len(t, tenantFirst, 4) // 2 tenants x 2 services
}

func TestPartitionLogs_LogRecordPartitionerDedup(t *testing.T) {
	t.Run("split across partitions", func(t *testing.T) {
		logs := plog.NewLogs()
		rl := logs.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "test")
		sl := rl.ScopeLogs().AppendEmpty()
		sl.Scope().SetName("s")
		for _, sev := range []string{"ERROR", "INFO", "ERROR"} {
			lr := sl.LogRecords().AppendEmpty()
			lr.SetSeverityText(sev)
		}

		result := partitionLogs(t, logs,
			`log.severity_text`,
		)
		require.Len(t, result, 2)
		for _, pl := range result {
			assert.Equal(t, 1, pl.data.ResourceLogs().Len())
			assert.Equal(t, 1, pl.data.ResourceLogs().At(0).ScopeLogs().Len())
		}
	})

	t.Run("all same partition", func(t *testing.T) {
		logs := plog.NewLogs()
		rl := logs.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "test")
		sl := rl.ScopeLogs().AppendEmpty()
		sl.Scope().SetName("s")
		for range 3 {
			sl.LogRecords().AppendEmpty().SetSeverityText("ERROR")
		}

		result := partitionLogs(t, logs,
			`log.severity_text`,
		)
		require.Len(t, result, 1)
		require.Equal(t, 1, result[0].data.ResourceLogs().Len())
		destRL := result[0].data.ResourceLogs().At(0)
		require.Equal(t, 1, destRL.ScopeLogs().Len())
		assert.Equal(t, 3, destRL.ScopeLogs().At(0).LogRecords().Len())
	})

	t.Run("two source RLs same partition", func(t *testing.T) {
		logs := plog.NewLogs()
		for _, svc := range []string{"svc-a", "svc-b"} {
			rl := logs.ResourceLogs().AppendEmpty()
			rl.Resource().Attributes().PutStr("service.name", svc)
			rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().SetSeverityText("ERROR")
		}

		result := partitionLogs(t, logs,
			`log.severity_text`,
		)
		require.Len(t, result, 1)
		assert.Equal(t, 2, result[0].data.ResourceLogs().Len())
	})
}

func TestPartitionLogs_ScopePartitionerDedup(t *testing.T) {
	t.Run("two scopes same RL same partition", func(t *testing.T) {
		logs := plog.NewLogs()
		rl := logs.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("tenant.id", "t1")
		rl.ScopeLogs().AppendEmpty().Scope().SetName("scope-a")
		rl.ScopeLogs().AppendEmpty().Scope().SetName("scope-b")

		result := partitionLogs(t, logs,
			`resource.attributes["tenant.id"]`,
		)
		require.Len(t, result, 1)
		require.Equal(t, 1, result[0].data.ResourceLogs().Len())
		assert.Equal(t, 2, result[0].data.ResourceLogs().At(0).ScopeLogs().Len())
	})

	t.Run("two scopes different RLs same partition", func(t *testing.T) {
		logs := plog.NewLogs()
		for _, svc := range []string{"svc-a", "svc-b"} {
			rl := logs.ResourceLogs().AppendEmpty()
			rl.Resource().Attributes().PutStr("service.name", svc)
			rl.ScopeLogs().AppendEmpty().Scope().SetName("shared-scope")
		}

		result := partitionLogs(t, logs,
			`scope.name`,
		)
		require.Len(t, result, 1)
		require.Equal(t, 2, result[0].data.ResourceLogs().Len())
		for i := range result[0].data.ResourceLogs().Len() {
			assert.Equal(t, 1, result[0].data.ResourceLogs().At(i).ScopeLogs().Len())
		}
	})
}

func TestPartitionLogs_PreservesSchemaURL(t *testing.T) {
	t.Run("scope partitioner", func(t *testing.T) {
		logs := plog.NewLogs()
		rl := logs.ResourceLogs().AppendEmpty()
		rl.SetSchemaUrl("https://example.com/resource-schema")
		rl.Resource().Attributes().PutStr("service.name", "test")
		sl := rl.ScopeLogs().AppendEmpty()
		sl.SetSchemaUrl("https://example.com/scope-schema")
		sl.Scope().SetName("s")
		sl.LogRecords().AppendEmpty()

		result := partitionLogs(t, logs,
			`scope.name`,
		)
		require.Len(t, result, 1)
		destRL := result[0].data.ResourceLogs().At(0)
		assert.Equal(t, "https://example.com/resource-schema", destRL.SchemaUrl())
		assert.Equal(t, "https://example.com/scope-schema", destRL.ScopeLogs().At(0).SchemaUrl())
	})

	t.Run("log record partitioner", func(t *testing.T) {
		logs := plog.NewLogs()
		rl := logs.ResourceLogs().AppendEmpty()
		rl.SetSchemaUrl("https://example.com/resource-schema")
		sl := rl.ScopeLogs().AppendEmpty()
		sl.SetSchemaUrl("https://example.com/scope-schema")
		sl.Scope().SetName("s")
		sl.LogRecords().AppendEmpty().SetSeverityText("ERROR")

		result := partitionLogs(t, logs,
			`log.severity_text`,
		)
		require.Len(t, result, 1)
		destRL := result[0].data.ResourceLogs().At(0)
		assert.Equal(t, "https://example.com/resource-schema", destRL.SchemaUrl())
		assert.Equal(t, "https://example.com/scope-schema", destRL.ScopeLogs().At(0).SchemaUrl())
	})
}

func TestPartitionLogs_SinglePartitionForwardsInputWithoutCopy(t *testing.T) {
	for _, expr := range []string{`resource.attributes["k"]`, `scope.name`, `log.attributes["k"]`} {
		t.Run(expr, func(t *testing.T) {
			logs := plog.NewLogs()
			for range 2 {
				rl := logs.ResourceLogs().AppendEmpty()
				rl.Resource().Attributes().PutStr("k", "same")
				sl := rl.ScopeLogs().AppendEmpty()
				sl.Scope().SetName("same")
				sl.LogRecords().AppendEmpty().Attributes().PutStr("k", "same")
			}
			result := partitionLogs(t, logs, expr)
			require.Len(t, result, 1)
			assert.Equal(t, logs, result[0].data, "single partition must forward the input as-is")
		})
	}
}

func TestPartitionLogs_FirstSeenOrder(t *testing.T) {
	logs := plog.NewLogs()
	for _, tenant := range []string{"t3", "t1", "t3", "t2"} {
		rl := logs.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("tenant.id", tenant)
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	}
	result := partitionLogs(t, logs, `resource.attributes["tenant.id"]`)
	require.Len(t, result, 3)
	got := make([]string, len(result))
	for i, pl := range result {
		got[i] = pl.values[0].value
	}
	assert.Equal(t, []string{"t3", "t1", "t2"}, got)
}

func TestPartitionLogs_EmptyInput(t *testing.T) {
	result := partitionLogs(t, plog.NewLogs(),
		`resource.attributes["tenant.id"]`,
	)
	assert.Empty(t, result)
}

func partitionLogs(t *testing.T, logs plog.Logs, expressions ...string) []partitionedLogs {
	t.Helper()
	p, err := newLogsPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	result, err := p.partitionLogs(t.Context(), logs)
	require.NoError(t, err)
	return result
}
