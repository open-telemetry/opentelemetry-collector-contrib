// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8sapiserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8sapiserver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	controlPlaneResourceType = "control_plane"
)

var (
	defaultResourceToType = map[string]string{
		controlPlaneResourceType: "Cluster",
	}
	defaultMetricsToResource = map[string]string{
		"apiserver_storage_objects":                                 controlPlaneResourceType,
		"apiserver_request_total":                                   controlPlaneResourceType,
		"apiserver_request_duration_seconds":                        controlPlaneResourceType,
		"apiserver_admission_controller_admission_duration_seconds": controlPlaneResourceType,
		"rest_client_request_duration_seconds":                      controlPlaneResourceType,
		"rest_client_requests_total":                                controlPlaneResourceType,
		"etcd_request_duration_seconds":                             controlPlaneResourceType,
		"etcd_db_total_size_in_bytes":                               controlPlaneResourceType,
	}
)

type prometheusConsumer struct {
	nextConsumer      consumer.Metrics
	logger            *zap.Logger
	clusterName       string
	nodeName          string
	resourcesToType   map[string]string
	metricsToResource map[string]string
}

func newPrometheusConsumer(logger *zap.Logger, nextConsumer consumer.Metrics, clusterName string, nodeName string) *prometheusConsumer {
	return &prometheusConsumer{
		logger:            logger,
		nextConsumer:      nextConsumer,
		clusterName:       clusterName,
		nodeName:          nodeName,
		resourcesToType:   defaultResourceToType,
		metricsToResource: defaultMetricsToResource,
	}
}
func (c prometheusConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: true,
	}
}

func (c prometheusConsumer) ConsumeMetrics(ctx context.Context, originalMetrics pmetric.Metrics) error {

	localScopeMetrics := map[string]pmetric.ScopeMetrics{}
	newMetrics := pmetric.NewMetrics()

	for key, value := range c.resourcesToType {
		newResourceMetrics := newMetrics.ResourceMetrics().AppendEmpty()
		// common attributes
		newResourceMetrics.Resource().Attributes().PutStr("ClusterName", c.clusterName)
		newResourceMetrics.Resource().Attributes().PutStr("Version", "0")
		newResourceMetrics.Resource().Attributes().PutStr("Sources", "[\"apiserver\"]")
		newResourceMetrics.Resource().Attributes().PutStr("NodeName", c.nodeName)

		// resource-specific type metric
		newResourceMetrics.Resource().Attributes().PutStr("Type", value)

		newScopeMetrics := newResourceMetrics.ScopeMetrics().AppendEmpty()
		localScopeMetrics[key] = newScopeMetrics
	}

	rms := originalMetrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		scopeMetrics := rms.At(i).ScopeMetrics()
		for j := 0; j < scopeMetrics.Len(); j++ {
			scopeMetric := scopeMetrics.At(j)
			for k := 0; k < scopeMetric.Metrics().Len(); k++ {
				metric := scopeMetric.Metrics().At(k)
				// check control plane metrics
				resourceName, ok := c.metricsToResource[metric.Name()]
				if !ok {
					continue
				}
				resourceSpecificScopeMetrics, ok := localScopeMetrics[resourceName]
				if !ok {
					continue
				}
				c.logger.Debug(fmt.Sprintf("Copying metric %s into resource %s", metric.Name(), resourceName))
				metric.CopyTo(resourceSpecificScopeMetrics.Metrics().AppendEmpty())
			}
		}
	}

	c.logger.Info("Forwarding on k8sapiserver prometheus metrics",
		zap.Int("MetricCount", newMetrics.MetricCount()),
		zap.Int("DataPointCount", newMetrics.DataPointCount()))

	// forward on the new metrics
	return c.nextConsumer.ConsumeMetrics(ctx, newMetrics)
}
