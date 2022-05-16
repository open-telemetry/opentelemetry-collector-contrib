// Copyright 2022, OpenTelemetry Authors
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

package aerospikereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver"

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/internal/model"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"
)

// aerospikeReceiver is a metrics receiver using the Aerospike interface to collect
type aerospikeReceiver struct {
	params   component.ReceiverCreateSettings
	config   *Config
	consumer consumer.Metrics
	host     string // host/IP of configured Aerospike node
	port     int    // port of configured Aerospike node
	mb       *metadata.MetricsBuilder
}

// newAerospikeReceiver creates a new aerospikeReceiver connected to the endpoint provided in cfg
//
// If the host or port can't be parsed from endpoint, an error is returned.
func newAerospikeReceiver(params component.ReceiverCreateSettings, cfg *Config, consumer consumer.Metrics) (*aerospikeReceiver, error) {
	host, portStr, err := net.SplitHostPort(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errBadEndpoint, err)
	}

	port, err := strconv.ParseInt(portStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errBadPort, err)
	}

	return &aerospikeReceiver{
		params:   params,
		config:   cfg,
		consumer: consumer,
		host:     host,
		port:     int(port),
		mb:       metadata.NewMetricsBuilder(cfg.Metrics),
	}, nil
}

// scrape scrapes both Node and Namespace metrics from the provided Aerospike node.
// If CollectClusterMetrics is true, it then scrapes every discovered node
func (r *aerospikeReceiver) scrape(ctx context.Context) (pmetric.Metrics, error) {
	errs := &scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now().UTC())
	client, err := newASClient(r.host, r.port, r.config.Username, r.config.Password, r.config.Timeout)
	if err != nil {
		return pmetric.NewMetrics(), fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	info, err := client.Info()
	if err != nil {
		r.params.Logger.Warn(fmt.Sprintf("failed to get INFO: %s", err.Error()))
		return r.mb.Emit(), err
	}
	r.emitNode(info, client, now)

	if r.config.CollectClusterMetrics {
		r.params.Logger.Debug("Collecting peer nodes")
		for _, n := range info.Services {
			r.scrapeDiscoveredNode(n, now, errs)
		}
	}

	return r.mb.Emit(), errs.Combine()
}

// scrapeNode collects metrics from a single Aerospike node
func (r *aerospikeReceiver) scrapeNode(client aerospike, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	info, err := client.Info()
	if err != nil {
		errs.AddPartial(0, err)
		return
	}

	r.emitNode(info, client, now)
}

// scrapeDiscoveredNode connects to a discovered Aerospike node and scrapes it using that connection
//
// If unable to parse the endpoint or connect, that error is logged and we return early
func (r *aerospikeReceiver) scrapeDiscoveredNode(endpoint string, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	host, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		r.params.Logger.Warn(fmt.Sprintf("%s: %s", errBadEndpoint, err))
		errs.Add(err)
		return
	}
	port, err := strconv.ParseInt(portStr, 10, 32)
	if err != nil {
		r.params.Logger.Warn(fmt.Sprintf("%s: %s", errBadPort, err))
		errs.Add(err)
		return
	}

	nClient, err := newASClient(host, int(port), r.config.Username, r.config.Password, r.config.Timeout)
	if err != nil {
		r.params.Logger.Warn(err.Error())
		errs.Add(err)
		return
	}
	defer nClient.Close()

	r.scrapeNode(nClient, now, errs)
}

// emitNode records node metrics and emits the resource. It collects namespace metrics for each namespace on the node
//
// The given client is used to collect namespace metrics, which is connected to a single node
func (r *aerospikeReceiver) emitNode(info *model.NodeInfo, client aerospike, now pcommon.Timestamp) {
	if stats := info.Statistics; stats != nil {
		if stats.ClientConnections != nil {
			r.mb.RecordAerospikeNodeConnectionOpenDataPoint(now, *stats.ClientConnections, metadata.AttributeConnectionTypeClient)
		}
		if stats.FabricConnections != nil {
			r.mb.RecordAerospikeNodeConnectionOpenDataPoint(now, *stats.FabricConnections, metadata.AttributeConnectionTypeFabric)
		}
		if stats.HeartbeatConnections != nil {
			r.mb.RecordAerospikeNodeConnectionOpenDataPoint(now, *stats.HeartbeatConnections, metadata.AttributeConnectionTypeHeartbeat)
		}

		if stats.ClientConnectionsClosed != nil {
			r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, *stats.ClientConnectionsClosed, metadata.AttributeConnectionTypeClient, metadata.AttributeConnectionOpClose)
		}
		if stats.ClientConnectionsOpened != nil {
			r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, *stats.ClientConnectionsOpened, metadata.AttributeConnectionTypeClient, metadata.AttributeConnectionOpOpen)
		}
		if stats.FabricConnectionsClosed != nil {
			r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, *stats.FabricConnectionsClosed, metadata.AttributeConnectionTypeFabric, metadata.AttributeConnectionOpClose)
		}
		if stats.FabricConnectionsOpened != nil {
			r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, *stats.FabricConnectionsOpened, metadata.AttributeConnectionTypeFabric, metadata.AttributeConnectionOpOpen)
		}
		if stats.HeartbeatConnectionsClosed != nil {
			r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, *stats.HeartbeatConnectionsClosed, metadata.AttributeConnectionTypeHeartbeat, metadata.AttributeConnectionOpClose)
		}
		if stats.HeartbeatConnectionsOpened != nil {
			r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, *stats.HeartbeatConnectionsOpened, metadata.AttributeConnectionTypeHeartbeat, metadata.AttributeConnectionOpOpen)
		}
	}

	r.mb.EmitForResource(metadata.WithNodeName(info.Name))

	if info.Namespaces != nil {
		for _, n := range info.Namespaces {
			nInfo, err := client.NamespaceInfo(n)
			if err != nil {
				r.params.Logger.Warn(fmt.Sprintf("failed getting namespace %s: %s", n, err.Error()))
				continue
			}
			nInfo.Node = info.Name
			r.emitNamespace(nInfo, now)
		}
	}
}

// emitNamespace emits a namespace resource with its name as resource attribute
func (r *aerospikeReceiver) emitNamespace(info *model.NamespaceInfo, now pcommon.Timestamp) {
	if info.DeviceAvailablePct != nil {
		r.mb.RecordAerospikeNamespaceDiskAvailableDataPoint(now, *info.DeviceAvailablePct)
	}
	if info.MemoryFreePct != nil {
		r.mb.RecordAerospikeNodeMemoryFreeDataPoint(now, *info.MemoryFreePct)
	}
	if info.MemoryFreePct != nil {
		r.mb.RecordAerospikeNamespaceMemoryFreeDataPoint(now, *info.MemoryFreePct)
	}
	if info.MemoryUsedDataBytes != nil {
		r.mb.RecordAerospikeNamespaceMemoryUsageDataPoint(now, *info.MemoryUsedDataBytes, metadata.AttributeNamespaceComponentData)
	}
	if info.MemoryUsedIndexBytes != nil {
		r.mb.RecordAerospikeNamespaceMemoryUsageDataPoint(now, *info.MemoryUsedIndexBytes, metadata.AttributeNamespaceComponentIndex)
	}
	if info.MemoryUsedSIndexBytes != nil {
		r.mb.RecordAerospikeNamespaceMemoryUsageDataPoint(now, *info.MemoryUsedSIndexBytes, metadata.AttributeNamespaceComponentSindex)
	}
	if info.MemoryUsedSetIndexBytes != nil {
		r.mb.RecordAerospikeNamespaceMemoryUsageDataPoint(now, *info.MemoryUsedSetIndexBytes, metadata.AttributeNamespaceComponentSetIndex)
	}
	r.mb.EmitForResource(metadata.WithNamespace(info.Name), metadata.WithNodeName(info.Node))
}

// start opens a new connection to the configured endpoint
func (r *aerospikeReceiver) start(context.Context, component.Host) error {
	return nil
}

// shutdown closes the connection to Aerospike, if the underlying client is set
func (r *aerospikeReceiver) shutdown(context.Context) error {
	return nil
}
