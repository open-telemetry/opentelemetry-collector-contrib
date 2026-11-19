// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package redisfleettrackerextension

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

// TestE2E_TwoInstancesConverge runs two extensions against a real Redis
// instance (REDIS_ENDPOINT, default localhost:6379) and verifies that their
// member counts converge as instances join and leave, end to end.
func TestE2E_TwoInstancesConverge(t *testing.T) {
	endpoint := os.Getenv("REDIS_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:6379"
	}

	cfg := newTestConfig(endpoint)
	cfg.Fleet = "e2e-fleet-" + time.Now().Format("20060102150405.000000")
	cfg.HeartbeatInterval = 200 * time.Millisecond
	cfg.TTL = 800 * time.Millisecond

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
