// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grafanacloudconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/grafanacloudconnector"

import (
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type hostMetrics struct {
	mutex sync.RWMutex
	hosts map[string]struct{}
}

func newHostMetrics() *hostMetrics {
	return &hostMetrics{
		hosts: make(map[string]struct{}),
	}
}

func (h *hostMetrics) add(hostName string) {
	h.mutex.RLock()
	if _, ok := h.hosts[hostName]; !ok {
		h.mutex.RUnlock()
		h.mutex.Lock()
		defer h.mutex.Unlock()
		h.hosts[hostName] = struct{}{}
	} else {
		h.mutex.RUnlock()
	}
}

func (h *hostMetrics) count() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return len(h.hosts)
}

func (h *hostMetrics) metrics() (*pmetric.Metrics, int) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	count := len(h.hosts)
	var pm *pmetric.Metrics

	if count > 0 {
		metrics := pmetric.NewMetrics()
		pm = &metrics

		ilm := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
		m := ilm.Metrics().AppendEmpty()
		m.SetName(hostInfoMetric)
		m.SetEmptyGauge()
		dps := m.Gauge().DataPoints()

		dps.EnsureCapacity(count)
		timestamp := pcommon.NewTimestampFromTime(time.Now())
		for k := range h.hosts {
			dpCalls := dps.AppendEmpty()
			dpCalls.SetStartTimestamp(timestamp)
			dpCalls.SetTimestamp(timestamp)
			dpCalls.Attributes().PutStr(hostIdentifierAttr, k)
			dpCalls.SetIntValue(int64(1))
		}
	}

	return pm, count
}

func (h *hostMetrics) reset() {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.hosts = make(map[string]struct{})
}
