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
	"strconv"
	"time"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type prometheusConsumer struct {
	nextConsumer consumer.Metrics
	logger       *zap.Logger
	clusterName  string
	nodeName     string
}

func newPrometheusConsumer(logger *zap.Logger, nextConsumer consumer.Metrics, clusterName string, nodeName string) prometheusConsumer {
	return prometheusConsumer{
		logger:       logger,
		nextConsumer: nextConsumer,
		clusterName:  clusterName,
		nodeName:     nodeName,
	}
}
func (c prometheusConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: true,
	}
}

func (c prometheusConsumer) ConsumeMetrics(ctx context.Context, ld pmetric.Metrics) error {
	rms := ld.ResourceMetrics()

	for i := 0; i < rms.Len(); i++ {

		rm := rms.At(i)
		timestampNs := strconv.FormatInt(time.Now().UnixNano(), 10)

		rm.Resource().Attributes().PutStr("ClusterName", c.clusterName)
		rm.Resource().Attributes().PutStr("Type", "Cluster")
		rm.Resource().Attributes().PutStr("Timestamp", timestampNs)
		rm.Resource().Attributes().PutStr("Version", "0")
		rm.Resource().Attributes().PutStr("Sources", "[\"apiserver\"]")
		rm.Resource().Attributes().PutStr("NodeName", c.nodeName)

		// TODO: need to separate out metrics by type (cluster, service, etc)
	}

	// forward on the metrics
	return c.nextConsumer.ConsumeMetrics(ctx, ld)
}
