// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
)

func TestGroupForMetricName(t *testing.T) {
	tests := []struct {
		name   string
		metric string
		want   ScraperGroup
	}{
		{"vsan cluster", "vcenter.cluster.vsan.throughput", ScraperGroupVSAN},
		{"vsan host", "vcenter.host.vsan.latency.avg", ScraperGroupVSAN},
		{"vsan vm", "vcenter.vm.vsan.operations", ScraperGroupVSAN},
		{"cluster non-vsan", "vcenter.cluster.cpu.effective", ScraperGroupCluster},
		{"cluster host count", "vcenter.cluster.host.count", ScraperGroupCluster},
		{"datacenter", "vcenter.datacenter.vm.count", ScraperGroupDatacenter},
		{"datastore", "vcenter.datastore.disk.utilization", ScraperGroupDatastore},
		{"host", "vcenter.host.cpu.utilization", ScraperGroupHost},
		{"resource pool", "vcenter.resource_pool.cpu.usage", ScraperGroupResourcePool},
		{"vm", "vcenter.vm.memory.usage", ScraperGroupVM},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GroupForMetricName(tt.metric)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsScraperGroupEnabled(t *testing.T) {
	t.Run("nil Scrapers means all enabled", func(t *testing.T) {
		cfg := &Config{Scrapers: nil}
		assert.True(t, IsScraperGroupEnabled(cfg, ScraperGroupVSAN))
		assert.True(t, IsScraperGroupEnabled(cfg, ScraperGroupHost))
	})

	t.Run("missing group means enabled", func(t *testing.T) {
		cfg := &Config{
			Scrapers: map[ScraperGroup]ScraperConfig{
				ScraperGroupVM: {Enabled: false},
			},
		}
		assert.True(t, IsScraperGroupEnabled(cfg, ScraperGroupVSAN))
		assert.False(t, IsScraperGroupEnabled(cfg, ScraperGroupVM))
	})

	t.Run("explicit enabled/disabled", func(t *testing.T) {
		cfg := &Config{
			Scrapers: map[ScraperGroup]ScraperConfig{
				ScraperGroupVSAN: {Enabled: true},
				ScraperGroupHost: {Enabled: false},
			},
		}
		assert.True(t, IsScraperGroupEnabled(cfg, ScraperGroupVSAN))
		assert.False(t, IsScraperGroupEnabled(cfg, ScraperGroupHost))
	})
}

func TestEffectiveMetricsBuilderConfig(t *testing.T) {
	t.Run("nil Scrapers returns config unchanged", func(t *testing.T) {
		cfg := &Config{Scrapers: nil}
		mbc := metadata.DefaultMetricsBuilderConfig()
		got := EffectiveMetricsBuilderConfig(cfg, mbc)
		require.True(t, got.Metrics.VcenterClusterCPUEffective.Enabled)
		require.True(t, got.Metrics.VcenterHostCPUUtilization.Enabled)
	})

	t.Run("disabled vsan group disables vsan metrics only", func(t *testing.T) {
		cfg := &Config{
			Scrapers: map[ScraperGroup]ScraperConfig{
				ScraperGroupVSAN: {Enabled: false},
			},
		}
		mbc := metadata.DefaultMetricsBuilderConfig()
		got := EffectiveMetricsBuilderConfig(cfg, mbc)
		// Non-VSAN cluster metric still enabled
		assert.True(t, got.Metrics.VcenterClusterCPUEffective.Enabled)
		// VSAN metrics disabled
		assert.False(t, got.Metrics.VcenterClusterVsanThroughput.Enabled)
		assert.False(t, got.Metrics.VcenterHostVsanCacheHitRate.Enabled)
		assert.False(t, got.Metrics.VcenterVMVsanThroughput.Enabled)
	})

	t.Run("disabled host group disables host metrics", func(t *testing.T) {
		cfg := &Config{
			Scrapers: map[ScraperGroup]ScraperConfig{
				ScraperGroupHost: {Enabled: false},
			},
		}
		mbc := metadata.DefaultMetricsBuilderConfig()
		got := EffectiveMetricsBuilderConfig(cfg, mbc)
		assert.False(t, got.Metrics.VcenterHostCPUUtilization.Enabled)
		assert.True(t, got.Metrics.VcenterVMCPUUsage.Enabled)
	})
}

func TestAllScraperGroups(t *testing.T) {
	groups := AllScraperGroups()
	require.Len(t, groups, 7)
	set := make(map[ScraperGroup]bool)
	for _, g := range groups {
		set[g] = true
	}
	for _, want := range []ScraperGroup{
		ScraperGroupCluster, ScraperGroupDatacenter, ScraperGroupDatastore,
		ScraperGroupHost, ScraperGroupResourcePool, ScraperGroupVM, ScraperGroupVSAN,
	} {
		assert.True(t, set[want], "missing group %s", want)
	}
}
