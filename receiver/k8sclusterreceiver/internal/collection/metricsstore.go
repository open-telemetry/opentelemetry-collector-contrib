// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collection // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/collection"

import (
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/utils"
)

// metricsStore keeps track of the metrics being pushed along the pipeline
// every interval. Since Kubernetes events that generate these metrics are
// aperiodic, the values in this cache will be pushed along the pipeline
// until the next Kubernetes event pertaining to an object.
type metricsStore struct {
	sync.RWMutex
	metricsCache map[types.UID]pmetric.Metrics
}

// updates metricsStore with latest metrics.
func (ms *metricsStore) update(obj runtime.Object, md pmetric.Metrics) error {
	ms.Lock()
	defer ms.Unlock()

	key, err := utils.GetUIDForObject(obj)
	if err != nil {
		return err
	}

	ms.metricsCache[key] = md
	return nil
}

// removes entry from metric cache when resources are deleted.
func (ms *metricsStore) remove(obj runtime.Object) error {
	ms.Lock()
	defer ms.Unlock()

	key, err := utils.GetUIDForObject(obj)
	if err != nil {
		return err
	}

	delete(ms.metricsCache, key)
	return nil
}

// getMetricData returns metricsCache stored in the cache at a given point in time.
func (ms *metricsStore) getMetricData(currentTime time.Time) pmetric.Metrics {
	ms.RLock()
	defer ms.RUnlock()

	currentTimestamp := pcommon.NewTimestampFromTime(currentTime)
	out := pmetric.NewMetrics()
	for _, md := range ms.metricsCache {
		// Set datapoint timestamp to be time of retrieval from cache.
		applyCurrentTime(md, currentTimestamp)
		rms := pmetric.NewResourceMetricsSlice()
		md.ResourceMetrics().CopyTo(rms)
		rms.MoveAndAppendTo(out.ResourceMetrics())
	}

	return out
}

func applyCurrentTime(md pmetric.Metrics, t pcommon.Timestamp) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				switch ms.At(k).Type() {
				case pmetric.MetricTypeGauge:
					applyCurrentTimeNumberDataPoint(ms.At(k).Gauge().DataPoints(), t)
				case pmetric.MetricTypeSum:
					applyCurrentTimeNumberDataPoint(ms.At(k).Sum().DataPoints(), t)
				}
			}
		}
	}
}

func applyCurrentTimeNumberDataPoint(dps pmetric.NumberDataPointSlice, t pcommon.Timestamp) {
	for i := 0; i < dps.Len(); i++ {
		switch dps.At(i).ValueType() {
		case pmetric.NumberDataPointValueTypeDouble:
			dps.At(i).SetTimestamp(t)
		case pmetric.NumberDataPointValueTypeInt:
			dps.At(i).SetTimestamp(t)
		}
	}
}
