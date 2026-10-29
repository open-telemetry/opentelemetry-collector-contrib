// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/prometheus/common/promslog"
	promConfig "github.com/prometheus/prometheus/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/apiserver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/sharedpromconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/targetallocator"
)

func TestTargetAllocatorAndAPIServerSharePromConfig(t *testing.T) {
	t.Parallel()

	receiver := newReceiverWithTargetAllocatorAndAPIServer(t)

	taSharedCfg := extractSharedPromConfig(t, receiver.targetAllocatorManager)
	apiSharedCfg := extractSharedPromConfig(t, receiver.apiServerManager)
	require.NotNil(t, taSharedCfg)
	require.NotNil(t, apiSharedCfg)
	assert.Same(t, taSharedCfg, apiSharedCfg)

	// Make sure TA changes are visible to the API server manager
	promCfg := taSharedCfg.Get()
	initialLen := len(promCfg.ScrapeConfigs)
	taSharedCfg.Mutate(func(cfg *promConfig.Config) {
		cfg.ScrapeConfigs = append(cfg.ScrapeConfigs, &promConfig.ScrapeConfig{JobName: "ta-added"})
	})
	promCfg = apiSharedCfg.Get()
	require.Len(t, promCfg.ScrapeConfigs, initialLen+1)
	assert.Equal(t, "ta-added", promCfg.ScrapeConfigs[initialLen].JobName)
}

func TestPrometheusAPIServerManagerDisabledByDefault(t *testing.T) {
	t.Parallel()

	const promCfgYAML = `
global:
  scrape_interval: 15s
scrape_configs:
  - job_name: "initial"
    static_configs:
      - targets: ["127.0.0.1:1234"]
`

	promCfg, err := promConfig.Load(promCfgYAML, promslog.NewNopLogger())
	require.NoError(t, err)

	cfg := &Config{
		PrometheusConfig: (*PromConfig)(promCfg),
	}

	receiver, err := newPrometheusReceiver(receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.Nil(t, receiver.apiServerManager)
}

func newReceiverWithTargetAllocatorAndAPIServer(t *testing.T) *pReceiver {
	t.Helper()

	const promCfgYAML = `
global:
  scrape_interval: 15s
scrape_configs:
  - job_name: "initial"
    static_configs:
      - targets: ["127.0.0.1:1234"]
`

	promCfg, err := promConfig.Load(promCfgYAML, promslog.NewNopLogger())
	require.NoError(t, err)

	cfg := &Config{
		PrometheusConfig: (*PromConfig)(promCfg),
		APIServer: apiserver.Config{
			Enabled: true,
			ServerConfig: confighttp.ServerConfig{
				NetAddr: confignet.AddrConfig{
					Transport: "tcp",
					Endpoint:  "127.0.0.1:0",
				},
			},
		},
		TargetAllocator: configoptional.Some(targetallocator.Config{
			ClientConfig: confighttp.ClientConfig{
				Endpoint: "http://target-allocator:8080",
			},
			CollectorID: "collector-1",
			Interval:    time.Second,
		}),
	}

	receiver, err := newPrometheusReceiver(receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, receiver.targetAllocatorManager)
	require.NotNil(t, receiver.apiServerManager)
	return receiver
}

func extractSharedPromConfig(t *testing.T, manager any) *sharedpromconfig.Config {
	t.Helper()
	require.NotNil(t, manager)

	val := reflect.ValueOf(manager)
	require.True(t, val.IsValid())
	require.Equal(t, reflect.Pointer, val.Kind())

	field := val.Elem().FieldByName("promCfg")
	require.True(t, field.IsValid())
	require.True(t, field.CanAddr())

	return *(**sharedpromconfig.Config)(unsafe.Pointer(field.UnsafeAddr()))
}
