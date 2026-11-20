// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package integrationtest wires the redis_fleet_tracker extension to the
// adaptive_tail_sampling processor that consumes it. The processor resolves the
// extension by method shape rather than importing it, so neither module's
// build fails if the contract drifts and a mismatch is a silent no-op at
// runtime. This module is what catches that, and it is separate so the
// extension does not depend on the processor.
package integrationtest

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	rft "github.com/open-telemetry/opentelemetry-collector-contrib/extension/fleettracker/redisfleettrackerextension"
	ats "github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor"
)

// hostWithExtensions is the slice of component.Host the processor uses to
// resolve its fleet_tracker.
type hostWithExtensions struct {
	exts map[component.ID]component.Component
}

func (h hostWithExtensions) GetExtensions() map[component.ID]component.Component {
	return h.exts
}

// startTracker builds and starts a real extension with its own
// service.instance.id, so each registers as a distinct fleet member
// (member_id is derived from that attribute).
func startTracker(t *testing.T, id component.ID, endpoint, instanceID, fleet string) extension.Extension {
	t.Helper()

	f := rft.NewFactory()
	cfg := f.CreateDefaultConfig().(*rft.Config)
	cfg.Endpoint = endpoint
	cfg.Fleet = fleet
	cfg.HeartbeatInterval = 50 * time.Millisecond
	cfg.TTL = 500 * time.Millisecond

	set := extension.Settings{
		ID:                id,
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}
	set.Resource = pcommon.NewResource()
	set.Resource.Attributes().PutStr("service.instance.id", instanceID)

	ext, err := f.Create(t.Context(), set, cfg)
	require.NoError(t, err)
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { _ = ext.Shutdown(context.Background()) }) //nolint:usetesting // cleanup runs after the test context is cancelled
	return ext
}

// TestFleetTrackerDividesGoalByLiveMembers checks the pairing end to end. The
// processor resolves the extension structurally, then divides its configured
// goal_throughput by the member count the extension reports as instances join
// and leave.
func TestFleetTrackerDividesGoalByLiveMembers(t *testing.T) {
	srv := miniredis.RunT(t)
	trackerID := component.NewID(rft.NewFactory().Type())

	// The processor logs the goal it applies, which is where the divided value
	// is observable from outside the package.
	core, logs := observer.New(zapcore.DebugLevel)

	trackerA := startTracker(t, trackerID, srv.Addr(), "instance-a", "integration")

	pf := ats.NewFactory()
	cfg := pf.CreateDefaultConfig().(*ats.Config)
	cfg.DecisionDelay = 10 * time.Millisecond
	cfg.TraceTimeout = time.Hour
	cfg.FleetTrackerID = &trackerID
	cfg.Rules = []ats.RuleConfig{{
		Name: "throughput-cap",
		Sampler: ats.SamplerConfig{
			Type:           ats.AdaptiveThroughput,
			GoalThroughput: 1000,
		},
	}}

	pset := processor.Settings{
		ID:                component.NewID(pf.Type()),
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		BuildInfo:         component.NewDefaultBuildInfo(),
	}
	pset.Logger = zap.New(core)

	p, err := pf.CreateTraces(t.Context(), pset, cfg, consumertest.NewNop())
	require.NoError(t, err)

	host := hostWithExtensions{exts: map[component.ID]component.Component{
		trackerID: trackerA,
	}}
	// A Start error here means the processor could not resolve the extension
	// against its fleet_tracker contract.
	require.NoError(t, p.Start(t.Context(), host))
	t.Cleanup(func() { require.NoError(t, p.Shutdown(context.Background())) }) //nolint:usetesting // cleanup runs after the test context is cancelled

	// The processor starts at N=1, so a reported count of 1 is a no-change and
	// no goal is applied.
	assert.Empty(t, appliedGoals(logs), "no goal should be applied while N is unchanged at 1")

	// A second instance joins: 1000 / 2.
	trackerB := startTracker(t, trackerID, srv.Addr(), "instance-b", "integration")
	assert.Eventually(t, func() bool {
		return lastAppliedGoal(logs) == 500
	}, 10*time.Second, 50*time.Millisecond,
		"expected the goal to halve when a second member joined, saw %v", appliedGoals(logs))

	// It leaves again, deregistering on shutdown: back to 1000 / 1.
	require.NoError(t, trackerB.Shutdown(t.Context()))
	assert.Eventually(t, func() bool {
		return lastAppliedGoal(logs) == 1000
	}, 10*time.Second, 50*time.Millisecond,
		"expected the goal to return to 1000 when the member left, saw %v", appliedGoals(logs))
}

// appliedGoals returns the goal from every "applied fleet-divided goal" log
// line the processor has emitted, in order.
func appliedGoals(logs *observer.ObservedLogs) []int64 {
	var out []int64
	entries := logs.FilterMessage("applied fleet-divided goal").All()
	for i := range entries {
		for _, f := range entries[i].Context {
			if f.Key == "goal_per_sec" {
				out = append(out, f.Integer)
			}
		}
	}
	return out
}

func lastAppliedGoal(logs *observer.ObservedLogs) int64 {
	g := appliedGoals(logs)
	if len(g) == 0 {
		return -1
	}
	return g[len(g)-1]
}
