// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adaptivetailsamplingprocessor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/metadata"
)

// ctxCapturingSink records the context each batch is forwarded with, so tests
// can assert what client information a forward carries.
type ctxCapturingSink struct {
	mu       sync.Mutex
	contexts []context.Context
	traces   []ptrace.Traces
}

func (*ctxCapturingSink) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (s *ctxCapturingSink) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contexts = append(s.contexts, ctx)
	s.traces = append(s.traces, td)
	return nil
}

func (s *ctxCapturingSink) forwarded() ([]context.Context, []ptrace.Traces) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]context.Context(nil), s.contexts...), append([]ptrace.Traces(nil), s.traces...)
}

func metadataCtx(t *testing.T, tenant string) context.Context {
	return client.NewContext(t.Context(), client.Info{
		Metadata: client.NewMetadata(map[string][]string{"tenant": {tenant}}),
	})
}

// An evicted trace's spans arrived on earlier requests, so it must not be
// forwarded carrying the client information of whichever request happened to
// trigger the eviction. Attributing it to an unrelated client is worse than
// carrying none, because anything reading client.Info downstream (headers_setter
// with from_context, per-request auth) would route it as that other client.
func TestEvictedTrace_DoesNotInheritTriggeringRequestContext(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.TraceTimeout = time.Hour
	cfg.DecisionDelay = time.Hour
	cfg.NumTraces = 1 // the second trace evicts the first
	cfg.Rules = []RuleConfig{{Name: "keep-all", Sampler: SamplerConfig{Type: Probabilistic, SamplingPercentage: 100}}}

	sink := &ctxCapturingSink{}
	p, err := newProcessor(processortest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, p.Start(t.Context(), nil))
	t.Cleanup(func() { require.NoError(t, p.Shutdown(t.Context())) })

	first := newRootTrace(pcommon.TraceID([16]byte{0xA1}))
	require.NoError(t, p.ConsumeTraces(metadataCtx(t, "tenant-a"), first))

	second := newRootTrace(pcommon.TraceID([16]byte{0xB2}))
	require.NoError(t, p.ConsumeTraces(metadataCtx(t, "tenant-b"), second))

	contexts, traces := sink.forwarded()
	require.NotEmpty(t, contexts, "the evicted trace should have been forwarded")

	for i, ctx := range contexts {
		got := client.FromContext(ctx).Metadata.Get("tenant")
		assert.Emptyf(t, got, "forward %d carried tenant metadata %v from the triggering request", i, got)
		require.Positive(t, traces[i].SpanCount())
	}
}
