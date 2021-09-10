// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collection

import (
	"sync"
	"time"

	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"go.opentelemetry.io/collector/model/pdata"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	internaldata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/utils"
)

// metricsStore keeps track of the metrics being pushed along the pipeline
// every interval. Since Kubernetes events that generate these metrics are
// aperiodic, the values in this cache will be pushed along the pipeline
// until the next Kubernetes event pertaining to an object.
type metricsStore struct {
	sync.RWMutex
	metricsCache map[types.UID][]*agentmetricspb.ExportMetricsServiceRequest
}

// This probably wouldn't be required once the new OTLP ResourceMetrics
// struct is made available.
type resourceMetrics struct {
	resource *resourcepb.Resource
	metrics  []*metricspb.Metric
}

// updates metricsStore with latest metrics.
func (ms *metricsStore) update(obj runtime.Object, rms []*resourceMetrics) error {
	ms.Lock()
	defer ms.Unlock()

	key, err := utils.GetUIDForObject(obj)
	if err != nil {
		return err
	}

	origMds := make([]agentmetricspb.ExportMetricsServiceRequest, len(rms))
	mds := make([]*agentmetricspb.ExportMetricsServiceRequest, len(rms))
	for i, rm := range rms {
		mds[i] = &origMds[i]
		mds[i].Resource = rm.resource
		mds[i].Metrics = rm.metrics
	}

	ms.metricsCache[key] = mds
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
func (ms *metricsStore) getMetricData(currentTime time.Time) pdata.Metrics {
	ms.RLock()
	defer ms.RUnlock()

	out := pdata.NewMetrics()
	for _, mds := range ms.metricsCache {
		for i := range mds {
			// Set datapoint timestamp to be time of retrieval from cache.
			applyCurrentTime(mds[i].Metrics, currentTime)
			internaldata.OCToMetrics(mds[i].Node, mds[i].Resource, mds[i].Metrics).ResourceMetrics().MoveAndAppendTo(out.ResourceMetrics())
		}
	}

	return out
}

func applyCurrentTime(metrics []*metricspb.Metric, t time.Time) []*metricspb.Metric {
	currentTime := timestamppb.New(t)
	for _, metric := range metrics {
		if metric != nil {
			for i := range metric.Timeseries {
				metric.Timeseries[i].Points[0].Timestamp = currentTime
			}
		}
	}
	return metrics
}
