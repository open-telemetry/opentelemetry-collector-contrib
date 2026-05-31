// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/kafkatest"
)

// TestFranzAdminProvider_SharesSingleClient verifies admin() returns the same
// client across calls and closes it only after all references are released.
func TestFranzAdminProvider_SharesSingleClient(t *testing.T) {
	_, clientCfg := kafkatest.NewCluster(t, kfake.SeedTopics(1, "meta-topic"))
	p := newFranzAdminProvider(clientCfg, zap.NewNop())

	// Two simulated scrapers retain the provider.
	p.retain()
	p.retain()

	host := componenttest.NewNopHost()
	adm1, err := p.admin(t.Context(), host)
	require.NoError(t, err)
	require.NotNil(t, adm1)

	adm2, err := p.admin(t.Context(), host)
	require.NoError(t, err)
	// Both scrapers get the exact same shared client instance.
	require.Same(t, adm1, adm2)

	// Releasing one reference must NOT close the shared client; the other
	// scraper is still using it.
	p.release()
	p.mu.Lock()
	stillOpen := p.adm != nil
	p.mu.Unlock()
	require.True(t, stillOpen, "client closed while a reference is still held")

	// admin() should still return the same client.
	adm3, err := p.admin(t.Context(), host)
	require.NoError(t, err)
	require.Same(t, adm1, adm3)

	// Releasing the last reference closes the client.
	p.release()
	p.mu.Lock()
	closed := p.adm == nil
	p.mu.Unlock()
	require.True(t, closed, "client not closed after last reference released")
}

// TestFranzAdminProvider_LazyCreation verifies the client is created lazily on
// the first admin() call, not before.
func TestFranzAdminProvider_LazyCreation(t *testing.T) {
	_, clientCfg := kafkatest.NewCluster(t, kfake.SeedTopics(1, "meta-topic"))
	p := newFranzAdminProvider(clientCfg, zap.NewNop())

	p.retain()
	p.mu.Lock()
	created := p.adm != nil
	p.mu.Unlock()
	require.False(t, created, "client created before first admin() call")

	_, err := p.admin(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	p.release()
}
