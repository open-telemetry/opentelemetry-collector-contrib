// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisfleettrackerextension

import (
	"regexp"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/fleettracker/redisfleettrackerextension/internal/metadata"
)

const (
	testHeartbeatInterval = 20 * time.Millisecond
	testTTL               = 80 * time.Millisecond
	testTimeout           = 5 * time.Second
	testTick              = 5 * time.Millisecond
)

var hexMemberIDPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

func newTestConfig(addr string) *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = addr
	cfg.HeartbeatInterval = testHeartbeatInterval
	cfg.TTL = testTTL
	return cfg
}

// newTestExtension builds an extension the same way the factory does,
// optionally seeding a service.instance.id resource attribute.
func newTestExtension(t *testing.T, cfg *Config, resourceAttrs map[string]string) *fleetTrackerExtension {
	t.Helper()

	set := extensiontest.NewNopSettings(component.MustNewType("redis_fleet_tracker"))
	if len(resourceAttrs) > 0 {
		res := pcommon.NewResource()
		for k, v := range resourceAttrs {
			res.Attributes().PutStr(k, v)
		}
		set.Resource = res
	}

	tb, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	require.NoError(t, err)

	memberID := computeMemberID(set)
	return newExtension(cfg, zap.NewNop(), tb, memberID)
}

func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(testTimeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(testTick)
	}
	require.Fail(t, "timed out waiting: "+msg)
}

func TestExtension_RegisterOnStart(t *testing.T) {
	srv := miniredis.RunT(t)
	cfg := newTestConfig(srv.Addr())
	e := newTestExtension(t, cfg, nil)

	require.NoError(t, e.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, e.Shutdown(t.Context())) })

	members, err := srv.ZMembers(e.key)
	require.NoError(t, err)
	require.Contains(t, members, e.memberID)
	assert.True(t, hexMemberIDPattern.MatchString(e.memberID), "member id must be 16 hex chars, got %q", e.memberID)

	score, err := srv.ZScore(e.key, e.memberID)
	require.NoError(t, err)
	assert.Greater(t, score, float64(time.Now().UnixMilli()), "member score must be in the future")
}

func TestExtension_MemberID_ServiceInstanceID(t *testing.T) {
	srv := miniredis.RunT(t)
	cfg := newTestConfig(srv.Addr())
	e1 := newTestExtension(t, cfg, map[string]string{"service.instance.id": "same-instance"})
	e2 := newTestExtension(t, cfg, map[string]string{"service.instance.id": "same-instance"})

	assert.Equal(t, e1.memberID, e2.memberID, "identical service.instance.id must produce identical member ids")
	assert.True(t, hexMemberIDPattern.MatchString(e1.memberID))
}

func TestExtension_MemberID_UUIDFallback(t *testing.T) {
	srv := miniredis.RunT(t)
	cfg := newTestConfig(srv.Addr())
	e1 := newTestExtension(t, cfg, nil)
	e2 := newTestExtension(t, cfg, nil)

	assert.NotEqual(t, e1.memberID, e2.memberID, "without service.instance.id each instance gets a unique member id")
	assert.True(t, hexMemberIDPattern.MatchString(e1.memberID))
	assert.True(t, hexMemberIDPattern.MatchString(e2.memberID))
}

func TestExtension_RefreshAdvancesScore(t *testing.T) {
	srv := miniredis.RunT(t)
	cfg := newTestConfig(srv.Addr())
	e := newTestExtension(t, cfg, nil)

	require.NoError(t, e.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, e.Shutdown(t.Context())) })

	firstScore, err := srv.ZScore(e.key, e.memberID)
	require.NoError(t, err)

	waitFor(t, func() bool {
		score, err := srv.ZScore(e.key, e.memberID)
		return err == nil && score > firstScore
	}, "score should advance across ticks")
}

func TestExtension_PruneExpiredMembers(t *testing.T) {
	srv := miniredis.RunT(t)
	cfg := newTestConfig(srv.Addr())
	e := newTestExtension(t, cfg, nil)

	require.NoError(t, e.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, e.Shutdown(t.Context())) })

	// Seed a stale member directly, with a score in the past.
	staleScore := float64(time.Now().Add(-time.Minute).UnixMilli())
	_, err := srv.ZAdd(e.key, staleScore, "stale-member")
	require.NoError(t, err)

	waitFor(t, func() bool {
		members, err := srv.ZMembers(e.key)
		require.NoError(t, err)
		return !slices.Contains(members, "stale-member")
	}, "stale member should be pruned")

	e.mu.Lock()
	count := e.count
	e.mu.Unlock()
	assert.Equal(t, 1, count, "count must exclude the pruned stale member")
}

func TestExtension_SubscribeMemberCount_TwoInstances(t *testing.T) {
	srv := miniredis.RunT(t)
	cfg := newTestConfig(srv.Addr())
	cfg.Fleet = "shared-fleet"

	a := newTestExtension(t, cfg, nil)
	require.NoError(t, a.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, a.Shutdown(t.Context())) })

	var mu sync.Mutex
	var lastSeen int
	cancel, err := a.SubscribeMemberCount(func(count int) {
		mu.Lock()
		lastSeen = count
		mu.Unlock()
	})
	require.NoError(t, err)
	t.Cleanup(cancel)

	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return lastSeen == 1
	}, "A should see itself as the only member")

	b := newTestExtension(t, cfg, nil)
	require.NoError(t, b.Start(t.Context(), componenttest.NewNopHost()))

	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return lastSeen == 2
	}, "A should see B join")

	require.NoError(t, b.Shutdown(t.Context()))

	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return lastSeen == 1
	}, "A should see B leave")
}

func TestExtension_SubscribeMemberCount_Dedup(t *testing.T) {
	srv := miniredis.RunT(t)
	cfg := newTestConfig(srv.Addr())
	e := newTestExtension(t, cfg, nil)

	require.NoError(t, e.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, e.Shutdown(t.Context())) })

	var mu sync.Mutex
	deliveries := 0
	cancel, err := e.SubscribeMemberCount(func(count int) {
		mu.Lock()
		deliveries++
		mu.Unlock()
		assert.Equal(t, 1, count)
	})
	require.NoError(t, err)
	t.Cleanup(cancel)

	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return deliveries >= 1
	}, "should get the first delivery")

	// Let several more heartbeats tick with stable membership.
	time.Sleep(10 * testHeartbeatInterval)

	mu.Lock()
	got := deliveries
	mu.Unlock()
	assert.Equal(t, 1, got, "stable membership must deliver exactly once")
}

func TestExtension_SubscribeMemberCount_CancelStopsDeliveries(t *testing.T) {
	srv := miniredis.RunT(t)
	cfg := newTestConfig(srv.Addr())
	e := newTestExtension(t, cfg, nil)

	require.NoError(t, e.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, e.Shutdown(t.Context())) })

	var mu sync.Mutex
	deliveries := 0
	cancel, err := e.SubscribeMemberCount(func(int) {
		mu.Lock()
		deliveries++
		mu.Unlock()
	})
	require.NoError(t, err)

	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return deliveries >= 1
	}, "should get the first delivery")

	cancel()
	cancel() // must be safe to call twice

	mu.Lock()
	afterCancel := deliveries
	mu.Unlock()

	time.Sleep(10 * testHeartbeatInterval)

	mu.Lock()
	final := deliveries
	mu.Unlock()
	assert.LessOrEqual(t, final, afterCancel+1, "at most one in-flight delivery may complete after cancel")
}

func TestExtension_MultipleSubscribers_OneMember(t *testing.T) {
	srv := miniredis.RunT(t)
	cfg := newTestConfig(srv.Addr())
	e := newTestExtension(t, cfg, nil)

	require.NoError(t, e.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, e.Shutdown(t.Context())) })

	cancel1, err := e.SubscribeMemberCount(func(int) {})
	require.NoError(t, err)
	t.Cleanup(cancel1)
	cancel2, err := e.SubscribeMemberCount(func(int) {})
	require.NoError(t, err)
	t.Cleanup(cancel2)

	members, err := srv.ZMembers(e.key)
	require.NoError(t, err)
	assert.Len(t, members, 1, "two subscribers on one extension still register only one zset member")
}

func TestExtension_DifferentFleetsAreIsolated(t *testing.T) {
	srv := miniredis.RunT(t)

	cfgA := newTestConfig(srv.Addr())
	cfgA.Fleet = "fleet-a"
	a := newTestExtension(t, cfgA, nil)
	require.NoError(t, a.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, a.Shutdown(t.Context())) })

	cfgB := newTestConfig(srv.Addr())
	cfgB.Fleet = "fleet-b"
	b := newTestExtension(t, cfgB, nil)
	require.NoError(t, b.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, b.Shutdown(t.Context())) })

	membersA, err := srv.ZMembers(a.key)
	require.NoError(t, err)
	membersB, err := srv.ZMembers(b.key)
	require.NoError(t, err)
	assert.Len(t, membersA, 1)
	assert.Len(t, membersB, 1)
	assert.NotEqual(t, a.key, b.key)
}

func TestExtension_UnreachableEndpoint(t *testing.T) {
	cfg := newTestConfig("127.0.0.1:1")
	e := newTestExtension(t, cfg, nil)

	var called bool
	_, err := e.SubscribeMemberCount(func(int) { called = true })
	require.NoError(t, err)

	require.NoError(t, e.Start(t.Context(), componenttest.NewNopHost()))
	time.Sleep(3 * testHeartbeatInterval)
	assert.False(t, called, "no callback should fire without a reachable Redis")

	require.NoError(t, e.Shutdown(t.Context()))
}

func TestExtension_UsernameAuth(t *testing.T) {
	srv := miniredis.RunT(t)
	srv.RequireUserAuth("fleetuser", "fleetpw")

	cfg := newTestConfig(srv.Addr())
	cfg.Username = "fleetuser"
	cfg.Password = "fleetpw"

	e := newTestExtension(t, cfg, nil)
	require.NoError(t, e.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, e.Shutdown(t.Context())) })

	var mu sync.Mutex
	var lastSeen int
	cancel, err := e.SubscribeMemberCount(func(count int) {
		mu.Lock()
		lastSeen = count
		mu.Unlock()
	})
	require.NoError(t, err)
	t.Cleanup(cancel)

	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return lastSeen == 1
	}, "ACL-authenticated heartbeat should register this member")
}

func TestExtension_UsernameAuth_WrongUserCannotHeartbeat(t *testing.T) {
	srv := miniredis.RunT(t)
	srv.RequireUserAuth("fleetuser", "fleetpw")

	cfg := newTestConfig(srv.Addr())
	cfg.Username = "wronguser"
	cfg.Password = "fleetpw"

	e := newTestExtension(t, cfg, nil)
	// Start never fails on an unreachable or rejecting Redis by design.
	require.NoError(t, e.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, e.Shutdown(t.Context())) })

	cancel, err := e.SubscribeMemberCount(func(int) {
		t.Error("no count should be delivered while auth is rejected")
	})
	require.NoError(t, err)
	t.Cleanup(cancel)

	time.Sleep(3 * testHeartbeatInterval)
	e.mu.Lock()
	count := e.count
	e.mu.Unlock()
	assert.Equal(t, unknownCount, count, "a rejected heartbeat must leave the count unknown")
}
