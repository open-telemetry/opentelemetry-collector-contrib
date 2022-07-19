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

package aerospikereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver"

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/internal/metadata"
)

// aerospikeReceiver is a metrics receiver using the Aerospike interface to collect
type aerospikeReceiver struct {
	config        *Config
	consumer      consumer.Metrics
	host          string // host/IP of configured Aerospike node
	port          int    // port of configured Aerospike node
	clientFactory clientFactoryFunc
	mb            *metadata.MetricsBuilder
	logger        *zap.Logger
}

// clientFactoryFunc creates an Aerospike connection to the given host and port
type clientFactoryFunc func(host string, port int) (aerospike, error)

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
		logger:   params.Logger,
		config:   cfg,
		consumer: consumer,
		clientFactory: func(host string, port int) (aerospike, error) {
			return newASClient(host, port, cfg.Username, cfg.Password, cfg.Timeout, params.Logger)
		},
		host: host,
		port: int(port),
		mb:   metadata.NewMetricsBuilder(cfg.Metrics, params.BuildInfo),
	}, nil
}

// scrape scrapes both Node and Namespace metrics from the provided Aerospike node.
// If CollectClusterMetrics is true, it then scrapes every discovered node
func (r *aerospikeReceiver) scrape(ctx context.Context) (pmetric.Metrics, error) {
	errs := &scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now().UTC())
	client, err := r.clientFactory(r.host, r.port)
	if err != nil {
		return pmetric.NewMetrics(), fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	info, err := client.Info()
	if err != nil {
		r.logger.Warn(fmt.Sprintf("failed to get INFO: %s", err.Error()))
		return r.mb.Emit(), err
	}
	r.emitNode(info, now, errs)
	r.scrapeNamespaces(info, client, now, errs)

	if r.config.CollectClusterMetrics {
		r.logger.Debug("Collecting peer nodes")
		for _, n := range strings.Split(info["services"], ";") {
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

	r.emitNode(info, now, errs)
	r.scrapeNamespaces(info, client, now, errs)
}

// scrapeDiscoveredNode connects to a discovered Aerospike node and scrapes it using that connection
//
// If unable to parse the endpoint or connect, that error is logged and we return early
func (r *aerospikeReceiver) scrapeDiscoveredNode(endpoint string, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	host, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		r.logger.Warn(fmt.Sprintf("%s: %s", errBadEndpoint, err))
		errs.Add(err)
		return
	}
	port, err := strconv.ParseInt(portStr, 10, 32)
	if err != nil {
		r.logger.Warn(fmt.Sprintf("%s: %s", errBadPort, err))
		errs.Add(err)
		return
	}

	nClient, err := r.clientFactory(host, int(port))
	if err != nil {
		r.logger.Warn(err.Error())
		errs.Add(err)
		return
	}
	defer nClient.Close()

	r.scrapeNode(nClient, now, errs)
}

// emitNode records node metrics and emits the resource. If statistics are missing in INFO, nothing is recorded
func (r *aerospikeReceiver) emitNode(info map[string]string, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	for k, v := range info {
		switch k {
		case "client_connections":
			addPartialIfError(errs, r.mb.RecordAerospikeNodeConnectionOpenDataPoint(now, v, metadata.AttributeConnectionTypeClient))
		case "fabric_connections":
			addPartialIfError(errs, r.mb.RecordAerospikeNodeConnectionOpenDataPoint(now, v, metadata.AttributeConnectionTypeFabric))
		case "heartbeat_connections":
			addPartialIfError(errs, r.mb.RecordAerospikeNodeConnectionOpenDataPoint(now, v, metadata.AttributeConnectionTypeHeartbeat))
		case "client_connections_closed":
			addPartialIfError(errs, r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, v, metadata.AttributeConnectionTypeClient, metadata.AttributeConnectionOpClose))
		case "client_connections_opened":
			addPartialIfError(errs, r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, v, metadata.AttributeConnectionTypeClient, metadata.AttributeConnectionOpOpen))
		case "fabric_connections_closed":
			addPartialIfError(errs, r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, v, metadata.AttributeConnectionTypeFabric, metadata.AttributeConnectionOpClose))
		case "fabric_connections_opened":
			addPartialIfError(errs, r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, v, metadata.AttributeConnectionTypeFabric, metadata.AttributeConnectionOpOpen))
		case "heartbeat_connections_closed":
			addPartialIfError(errs, r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, v, metadata.AttributeConnectionTypeHeartbeat, metadata.AttributeConnectionOpClose))
		case "heartbeat_connections_opened":
			addPartialIfError(errs, r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, v, metadata.AttributeConnectionTypeHeartbeat, metadata.AttributeConnectionOpOpen))
		case "system_free_mem_pct":
			addPartialIfError(errs, r.mb.RecordAerospikeNodeMemoryFreeDataPoint(now, v))
		}
	}

	r.mb.EmitForResource(metadata.WithAerospikeNodeName(info["node"]))
}

// scrapeNamespaces records metrics for all namespaces on a node
// The given client is used to collect namespace metrics, which is connected to a single node
func (r *aerospikeReceiver) scrapeNamespaces(info map[string]string, client aerospike, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	namespaces := strings.Split(info["namespaces"], ";")
	for _, n := range namespaces {
		if n == "" {
			continue
		}
		nInfo, err := client.NamespaceInfo(n)
		if err != nil {
			r.logger.Warn(fmt.Sprintf("failed getting namespace %s: %s", n, err.Error()))
			errs.AddPartial(0, err)
			continue
		}
		nInfo["node"] = info["node"]
		r.emitNamespace(nInfo, now, errs)
	}
}

// emitNamespace emits a namespace resource with its name as resource attribute
func (r *aerospikeReceiver) emitNamespace(info map[string]string, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	for k, v := range info {
		switch k {
		// Capacity
		case "device_available_pct":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceDiskAvailableDataPoint(now, v))
		case "memory_free_pct":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceMemoryFreeDataPoint(now, v))

		// Memory usage
		case "memory_used_data_bytes":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceMemoryUsageDataPoint(now, v, metadata.AttributeNamespaceComponentData))
		case "memory_used_index_bytes":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceMemoryUsageDataPoint(now, v, metadata.AttributeNamespaceComponentIndex))
		case "memory_used_sindex_bytes":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceMemoryUsageDataPoint(now, v, metadata.AttributeNamespaceComponentSecondaryIndex))
		case "memory_used_set_index_bytes":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceMemoryUsageDataPoint(now, v, metadata.AttributeNamespaceComponentSetIndex))

		// Scans
		case "scan_aggr_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeScanTypeAggregation, metadata.AttributeScanResultAbort))
		case "scan_aggr_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeScanTypeAggregation, metadata.AttributeScanResultComplete))
		case "scan_aggr_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeScanTypeAggregation, metadata.AttributeScanResultError))
		case "scan_basic_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeScanTypeBasic, metadata.AttributeScanResultAbort))
		case "scan_basic_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeScanTypeBasic, metadata.AttributeScanResultComplete))
		case "scan_basic_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeScanTypeBasic, metadata.AttributeScanResultError))
		case "scan_ops_bg_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeScanTypeOpsBackground, metadata.AttributeScanResultAbort))
		case "scan_ops_bg_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeScanTypeOpsBackground, metadata.AttributeScanResultComplete))
		case "scan_ops_bg_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeScanTypeOpsBackground, metadata.AttributeScanResultError))
		case "scan_udf_bg_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeScanTypeUdfBackground, metadata.AttributeScanResultAbort))
		case "scan_udf_bg_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeScanTypeUdfBackground, metadata.AttributeScanResultComplete))
		case "scan_udf_bg_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeScanTypeUdfBackground, metadata.AttributeScanResultError))

		// 'Delete' transactions
		case "client_delete_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTransactionTypeDelete, metadata.AttributeTransactionResultError))
		case "client_delete_filtered_out":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTransactionTypeDelete, metadata.AttributeTransactionResultFilteredOut))
		case "client_delete_not_found":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTransactionTypeDelete, metadata.AttributeTransactionResultNotFound))
		case "client_delete_success":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTransactionTypeDelete, metadata.AttributeTransactionResultSuccess))
		case "client_delete_timeout":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTransactionTypeDelete, metadata.AttributeTransactionResultTimeout))

		// 'Read' transactions
		case "client_read_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTransactionTypeRead, metadata.AttributeTransactionResultError))
		case "client_read_filtered_out":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTransactionTypeRead, metadata.AttributeTransactionResultFilteredOut))
		case "client_read_not_found":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTransactionTypeRead, metadata.AttributeTransactionResultNotFound))
		case "client_read_success":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTransactionTypeRead, metadata.AttributeTransactionResultSuccess))
		case "client_read_timeout":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTransactionTypeRead, metadata.AttributeTransactionResultTimeout))

		// UDF transactions
		case "client_udf_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTransactionTypeUdf, metadata.AttributeTransactionResultError))
		case "client_udf_filtered_out":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTransactionTypeUdf, metadata.AttributeTransactionResultFilteredOut))
		case "client_udf_not_found":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTransactionTypeUdf, metadata.AttributeTransactionResultNotFound))
		case "client_udf_success":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTransactionTypeUdf, metadata.AttributeTransactionResultSuccess))
		case "client_udf_timeout":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTransactionTypeUdf, metadata.AttributeTransactionResultTimeout))

		// 'Write' transactions
		case "client_write_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTransactionTypeWrite, metadata.AttributeTransactionResultError))
		case "client_write_filtered_out":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTransactionTypeWrite, metadata.AttributeTransactionResultFilteredOut))
		case "client_write_not_found":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTransactionTypeWrite, metadata.AttributeTransactionResultNotFound))
		case "client_write_success":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTransactionTypeWrite, metadata.AttributeTransactionResultSuccess))
		case "client_write_timeout":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTransactionTypeWrite, metadata.AttributeTransactionResultTimeout))

		}
	}

	r.mb.EmitForResource(metadata.WithAerospikeNamespace(info["name"]), metadata.WithAerospikeNodeName(info["node"]))
}

// addPartialIfError adds a partial error if the given error isn't nil
func addPartialIfError(errs *scrapererror.ScrapeErrors, err error) {
	if err != nil {
		errs.AddPartial(1, err)
	}
}
