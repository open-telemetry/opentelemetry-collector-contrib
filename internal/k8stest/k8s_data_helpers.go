// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8stest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8stest"

import (
	"context"
	"fmt"
	"regexp"
	"runtime"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/multierr"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	Equal = iota
	Regex
	Exist
)

type ExpectedValue struct {
	mode  int
	value string
}

func NewExpectedValue(mode int, value string) *ExpectedValue {
	return &ExpectedValue{
		mode:  mode,
		value: value,
	}
}

func WaitForData(t *testing.T, entriesNum int, mc *consumertest.MetricsSink, tc *consumertest.TracesSink, lc *consumertest.LogsSink) {
	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)

	_, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, mc)
	require.NoError(t, err, "failed creating metrics receiver")
	_, err = f.CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, tc)
	require.NoError(t, err, "failed creating traces receiver")
	rcvr, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, lc)
	require.NoError(t, err, "failed creating logs receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	}()

	timeoutMinutes := 3
	require.Eventuallyf(t, func() bool {
		return len(mc.AllMetrics()) > entriesNum && len(tc.AllTraces()) > entriesNum && len(lc.AllLogs()) > entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d metrics, %d traces, %d logs in %d minutes", entriesNum,
		len(mc.AllMetrics()), len(tc.AllTraces()), len(lc.AllLogs()), timeoutMinutes)
}

func ScanTracesForAttributes(t *testing.T, ts *consumertest.TracesSink, expectedService string,
	kvs map[string]*ExpectedValue) {
	// Iterate over the received set of traces starting from the most recent entries due to a bug in the processor:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18892
	// TODO: Remove the reverse loop once it's fixed. All the metrics should be properly annotated.
	for i := len(ts.AllTraces()) - 1; i >= 0; i-- {
		traces := ts.AllTraces()[i]
		for i := 0; i < traces.ResourceSpans().Len(); i++ {
			resource := traces.ResourceSpans().At(i).Resource()
			service, exist := resource.Attributes().Get("service.name")
			assert.Equal(t, true, exist, "span do not has 'service.name' attribute in resource")
			if service.AsString() != expectedService {
				continue
			}
			assert.NoError(t, resourceHasAttributes(resource, kvs))
			return
		}
	}
	t.Fatalf("no spans found for service %s", expectedService)
}

func ScanMetricsForAttributes(t *testing.T, ms *consumertest.MetricsSink, expectedService string,
	kvs map[string]*ExpectedValue) {
	// Iterate over the received set of metrics starting from the most recent entries due to a bug in the processor:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18892
	// TODO: Remove the reverse loop once it's fixed. All the metrics should be properly annotated.
	for i := len(ms.AllMetrics()) - 1; i >= 0; i-- {
		metrics := ms.AllMetrics()[i]
		for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
			resource := metrics.ResourceMetrics().At(i).Resource()
			service, exist := resource.Attributes().Get("service.name")
			assert.Equal(t, true, exist, "metric do not has 'service.name' attribute in resource")
			if service.AsString() != expectedService {
				continue
			}
			assert.NoError(t, resourceHasAttributes(resource, kvs))
			return
		}
	}
	t.Fatalf("no metric found for service %s", expectedService)
}

func ScanLogsForAttributes(t *testing.T, ls *consumertest.LogsSink, expectedService string,
	kvs map[string]*ExpectedValue) {
	// Iterate over the received set of logs starting from the most recent entries due to a bug in the processor:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18892
	// TODO: Remove the reverse loop once it's fixed. All the metrics should be properly annotated.
	for i := len(ls.AllLogs()) - 1; i >= 0; i-- {
		logs := ls.AllLogs()[i]
		for i := 0; i < logs.ResourceLogs().Len(); i++ {
			resource := logs.ResourceLogs().At(i).Resource()
			service, exist := resource.Attributes().Get("service.name")
			assert.Equal(t, true, exist, "log do not has 'service.name' attribute in resource")
			if service.AsString() != expectedService {
				continue
			}
			assert.NoError(t, resourceHasAttributes(resource, kvs))
			return
		}
	}
	t.Fatalf("no logs found for service %s", expectedService)
}

func resourceHasAttributes(resource pcommon.Resource, kvs map[string]*ExpectedValue) error {
	foundAttrs := make(map[string]bool)
	for k := range kvs {
		foundAttrs[k] = false
	}

	resource.Attributes().Range(
		func(k string, v pcommon.Value) bool {
			if val, ok := kvs[k]; ok {
				switch val.mode {
				case Equal:
					if val.value == v.AsString() {
						foundAttrs[k] = true
					}
				case Regex:
					matched, _ := regexp.MatchString(val.value, v.AsString())
					if matched {
						foundAttrs[k] = true
					}
				case Exist:
					foundAttrs[k] = true
				}

			}
			return true
		},
	)

	var err error
	for k, v := range foundAttrs {
		if !v {
			err = multierr.Append(err, fmt.Errorf("%v attribute not found", k))
		}
	}
	return err
}

func SelectorFromMap(labelMap map[string]any) labels.Selector {
	labelStringMap := make(map[string]string)
	for key, value := range labelMap {
		labelStringMap[key] = value.(string)
	}
	labelSet := labels.Set(labelStringMap)
	return labelSet.AsSelector()
}

func HostEndpoint(t *testing.T) string {
	if runtime.GOOS == "darwin" {
		return "host.docker.internal"
	}

	client, err := docker.NewClientWithOpts(docker.FromEnv)
	require.NoError(t, err)
	client.NegotiateAPIVersion(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	network, err := client.NetworkInspect(ctx, "kind", types.NetworkInspectOptions{})
	require.NoError(t, err)
	for _, ipam := range network.IPAM.Config {
		return ipam.Gateway
	}
	require.Fail(t, "failed to find host endpoint")
	return ""
}
