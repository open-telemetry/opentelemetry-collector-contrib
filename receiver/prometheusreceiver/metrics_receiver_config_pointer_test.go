// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"reflect"
	"sync"
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
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/targetallocator"
)

func TestTargetAllocatorAndAPIServerSharePromConfigPointer(t *testing.T) {
	t.Parallel()

	receiver := newReceiverWithTargetAllocatorAndAPIServer(t)

	taPromCfg := extractManagerPromConfig(t, receiver.targetAllocatorManager)
	apiPromCfg := extractManagerPromConfig(t, receiver.apiServerManager)
	require.NotNil(t, taPromCfg)
	require.NotNil(t, apiPromCfg)
	assert.Same(t, taPromCfg, apiPromCfg)

	initialLen := len(apiPromCfg.ScrapeConfigs)
	taPromCfg.ScrapeConfigs = append(taPromCfg.ScrapeConfigs, &promConfig.ScrapeConfig{JobName: "ta-added"})
	require.Len(t, apiPromCfg.ScrapeConfigs, initialLen+1)
	assert.Equal(t, "ta-added", apiPromCfg.ScrapeConfigs[initialLen].JobName)
}

func TestTargetAllocatorAndAPIServerShareConfigLock(t *testing.T) {
	t.Parallel()

	receiver := newReceiverWithTargetAllocatorAndAPIServer(t)

	taLock := extractManagerLock(t, receiver.targetAllocatorManager)
	apiLock := extractManagerLock(t, receiver.apiServerManager)
	require.NotNil(t, taLock)
	require.NotNil(t, apiLock)
	assert.Same(t, taLock, apiLock)
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
		APIServer: configoptional.Some(apiserver.Config{
			ServerConfig: confighttp.ServerConfig{
				NetAddr: confignet.AddrConfig{
					Transport: "tcp",
					Endpoint:  "127.0.0.1:0",
				},
			},
		}),
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

func extractManagerPromConfig(t *testing.T, manager any) *promConfig.Config {
	t.Helper()
	require.NotNil(t, manager)

	val := reflect.ValueOf(manager)
	require.True(t, val.IsValid())
	require.Equal(t, reflect.Pointer, val.Kind())

	field := val.Elem().FieldByName("promCfg")
	require.True(t, field.IsValid())
	require.True(t, field.CanAddr())

	return *(**promConfig.Config)(unsafe.Pointer(field.UnsafeAddr()))
}

func extractManagerLock(t *testing.T, manager any) *sync.RWMutex {
	t.Helper()
	require.NotNil(t, manager)

	val := reflect.ValueOf(manager)
	require.True(t, val.IsValid())
	require.Equal(t, reflect.Pointer, val.Kind())

	field := val.Elem().FieldByName("cfgLock")
	require.True(t, field.IsValid())
	require.True(t, field.CanAddr())

	return *(**sync.RWMutex)(unsafe.Pointer(field.UnsafeAddr()))
}
