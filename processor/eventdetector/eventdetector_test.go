// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eventdetector

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/collector/component"
)

type fakeState struct {
	opened  map[string]time.Duration
	nowCall int
}

func (f *fakeState) Open(_ context.Context, name string, ttl time.Duration) {
	if f.opened == nil {
		f.opened = map[string]time.Duration{}
	}
	f.opened[name] = ttl
}
func (f *fakeState) IsOpen(_ context.Context, _ string) bool        { return false }
func (f *fakeState) ExpiresAt(_ context.Context, _ string) time.Time { return time.Time{} }

func logsWith(body string) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	rec := sl.LogRecords().AppendEmpty()
	rec.Body().SetStr(body)
	return ld
}

func TestLogRuleOpensWindowAndPassesThrough(t *testing.T) {
	state := &fakeState{}
	sink := &consumertest.LogsSink{}

	cfg := &Config{
		SignalstateExtension: &component.ID{},
		LogRules: []LogRule{{
			Name:       "log-spike",
			Window:     "spike",
			Conditions: []string{`body == "boom"`},
			Ttl:        time.Minute,
			Cooldown:   30 * time.Second,
		}},
	}
	require.NoError(t, cfg.Validate())

	set := processortest.NewNopSettings(processortest.NopType)
	core, err := newCore(cfg, set)
	require.NoError(t, err)
	core.signalState = state

	proc := &detectorLogs{core: core, next: sink}

	// no match: window stays closed, record still passes
	require.NoError(t, proc.ConsumeLogs(context.Background(), logsWith("nothing")))
	assert.Empty(t, state.opened)
	assert.Equal(t, 1, sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len(),
		"log records must pass through unchanged")

	// match: window opens, ttl honored
	require.NoError(t, proc.ConsumeLogs(context.Background(), logsWith("boom")))
	op, ok := state.opened["spike"]
	require.True(t, ok, "matching log must open the window")
	assert.Equal(t, time.Minute, op)
}

func TestCoreCompilesRulesAndValidates(t *testing.T) {
	cfg := &Config{SignalstateExtension: &component.ID{}}
	require.NoError(t, cfg.Validate())
	core, err := newCore(cfg, processortest.NewNopSettings(processortest.NopType))
	require.NoError(t, err)
	assert.Empty(t, core.logRules)
	assert.Empty(t, core.metricRules)
}

var _ consumer.Logs = &consumertest.LogsSink{}
