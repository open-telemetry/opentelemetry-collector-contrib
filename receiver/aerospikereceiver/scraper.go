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
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"time"

	as "github.com/aerospike/aerospike-client-go/v6"
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
	clientFactory clientFactoryFunc
	client        Aerospike
	mb            *metadata.MetricsBuilder
	logger        *zap.SugaredLogger
}

// clientFactoryFunc creates an Aerospike connection to the given host and port
type clientFactoryFunc func() (Aerospike, error)

// newAerospikeReceiver creates a new aerospikeReceiver connected to the endpoint provided in cfg
//
// If the host or port can't be parsed from endpoint, an error is returned.
func newAerospikeReceiver(params component.ReceiverCreateSettings, cfg *Config, consumer consumer.Metrics) (*aerospikeReceiver, error) {
	var err error
	var tlsCfg *tls.Config
	if cfg.TLS != nil {
		tlsCfg, err = cfg.TLS.LoadTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("%w: %s", errFailedTLSLoad, err)
		}
	}

	host, portStr, err := net.SplitHostPort(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errBadEndpoint, err)
	}

	port, err := strconv.ParseInt(portStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errBadPort, err)
	}

	ashost := as.NewHost(host, int(port))
	ashost.TLSName = cfg.TLSName

	sugaredLogger := params.Logger.Sugar()
	return &aerospikeReceiver{
		logger:   sugaredLogger,
		config:   cfg,
		consumer: consumer,
		clientFactory: func() (Aerospike, error) {
			conf := &clientConfig{
				host:                  ashost,
				username:              cfg.Username,
				password:              cfg.Password,
				timeout:               cfg.Timeout,
				logger:                sugaredLogger,
				collectClusterMetrics: cfg.CollectClusterMetrics,
				tls:                   tlsCfg,
			}
			return newASClient(
				conf,
				nodeGetterFactory,
			)
		},
		mb: metadata.NewMetricsBuilder(cfg.Metrics, params.BuildInfo),
	}, nil
}

func (r *aerospikeReceiver) start(_ context.Context, _ component.Host) error {
	r.logger.Debug("executing start")

	client, err := r.clientFactory()
	if err != nil {
		client = nil
		r.logger.Warn("initial client creation failed: %w", err) //  .Sugar().Warnf("initial client creation failed: %w", err)
	}

	r.client = client
	return nil
}

func (r *aerospikeReceiver) shutdown(_ context.Context) error {
	r.logger.Debug("executing close")
	if r.client != nil {
		r.client.Close()
	}
	return nil
}

// scrape scrapes both Node and Namespace metrics from the provided Aerospike node.
// If CollectClusterMetrics is true, it then scrapes every discovered node
func (r *aerospikeReceiver) scrape(ctx context.Context) (pmetric.Metrics, error) {
	r.logger.Debug("beginning scrape")
	errs := &scrapererror.ScrapeErrors{}

	if r.client == nil {
		var err error
		r.logger.Debug("client is nil, attempting to create a new client")
		r.client, err = r.clientFactory()
		if err != nil {
			r.client = nil
			addPartialIfError(errs, fmt.Errorf("client creation failed: %w", err))
			return r.mb.Emit(), errs.Combine()
		}
	}

	now := pcommon.NewTimestampFromTime(time.Now().UTC())
	client := r.client

	info := client.Info()
	for _, nodeInfo := range info {
		r.emitNode(nodeInfo, now, errs)
	}
	r.scrapeNamespaces(client, now, errs)

	return r.mb.Emit(), errs.Combine()
}

// emitNode records node metrics and emits the resource. If statistics are missing in INFO, nothing is recorded
func (r *aerospikeReceiver) emitNode(info map[string]string, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	r.logger.Debugf("emitNode len(info): %v", len(info))
	for k, v := range info {
		switch k {
		case "client_connections":
			addPartialIfError(errs, r.mb.RecordAerospikeNodeConnectionOpenDataPoint(now, v, metadata.AttributeClient))
		case "fabric_connections":
			addPartialIfError(errs, r.mb.RecordAerospikeNodeConnectionOpenDataPoint(now, v, metadata.AttributeFabric))
		case "heartbeat_connections":
			addPartialIfError(errs, r.mb.RecordAerospikeNodeConnectionOpenDataPoint(now, v, metadata.AttributeHeartbeat))
		case "client_connections_closed":
			addPartialIfError(errs, r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, v, metadata.AttributeClose, metadata.AttributeClient))
		case "client_connections_opened":
			addPartialIfError(errs, r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, v, metadata.AttributeOpen, metadata.AttributeClient))
		case "fabric_connections_closed":
			addPartialIfError(errs, r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, v, metadata.AttributeClose, metadata.AttributeFabric))
		case "fabric_connections_opened":
			addPartialIfError(errs, r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, v, metadata.AttributeOpen, metadata.AttributeFabric))
		case "heartbeat_connections_closed":
			addPartialIfError(errs, r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, v, metadata.AttributeClose, metadata.AttributeHeartbeat))
		case "heartbeat_connections_opened":
			addPartialIfError(errs, r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, v, metadata.AttributeOpen, metadata.AttributeHeartbeat))
		case "system_free_mem_pct":
			addPartialIfError(errs, r.mb.RecordAerospikeNodeMemoryFreeDataPoint(now, v))
		case "query_tracked":
			addPartialIfError(errs, r.mb.RecordAerospikeNodeQueryTrackedDataPoint(now, v))
		}
	}

	r.mb.EmitForResource(metadata.WithAerospikeNodeName(info["node"]))
	r.logger.Debug("finished emitNode")
}

// scrapeNamespaces records metrics for all namespaces on a node
// The given client is used to collect namespace metrics, which is connected to a single node
func (r *aerospikeReceiver) scrapeNamespaces(client Aerospike, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	r.logger.Debug("scraping namespaces")
	nInfo := client.NamespaceInfo()
	r.logger.Debugf("scrapeNamespaces len(nInfo): %v", len(nInfo))
	for node, nsMap := range nInfo {
		for nsName, nsStats := range nsMap {
			nsStats["node"] = node
			nsStats["name"] = nsName
			r.emitNamespace(nsStats, now, errs)
		}
	}
}

// emitNamespace emits a namespace resource with its name as resource attribute
func (r *aerospikeReceiver) emitNamespace(info map[string]string, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	r.logger.Debugf("emitNamespace len(info): %v", len(info))
	for k, v := range info {
		switch k {
		// Capacity
		case "device_available_pct":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceDiskAvailableDataPoint(now, v))
		case "memory_free_pct":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceMemoryFreeDataPoint(now, v))

		// Memory usage
		case "memory_used_data_bytes":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceMemoryUsageDataPoint(now, v, metadata.AttributeData))
		case "memory_used_index_bytes":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceMemoryUsageDataPoint(now, v, metadata.AttributeIndex))
		case "memory_used_sindex_bytes":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceMemoryUsageDataPoint(now, v, metadata.AttributeSecondaryIndex))
		case "memory_used_set_index_bytes":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceMemoryUsageDataPoint(now, v, metadata.AttributeSetIndex))

		// Scans
		case "scan_aggr_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeAbort, metadata.AttributeAggregation))
		case "scan_aggr_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeComplete, metadata.AttributeAggregation))
		case "scan_aggr_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeError, metadata.AttributeAggregation))
		case "scan_basic_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeAbort, metadata.AttributeBasic))
		case "scan_basic_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeComplete, metadata.AttributeBasic))
		case "scan_basic_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeError, metadata.AttributeBasic))
		case "scan_ops_bg_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeAbort, metadata.AttributeOpsBackground))
		case "scan_ops_bg_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeComplete, metadata.AttributeOpsBackground))
		case "scan_ops_bg_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeError, metadata.AttributeOpsBackground))
		case "scan_udf_bg_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeAbort, metadata.AttributeUdfBackground))
		case "scan_udf_bg_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeComplete, metadata.AttributeUdfBackground))
		case "scan_udf_bg_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceScanCountDataPoint(now, v, metadata.AttributeError, metadata.AttributeUdfBackground))

		// Pre Aerospike 6.0 query metrics. These were always done on secondary indexes, otherwise they were counted as a scan.
		case "query_aggr_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeAbort, metadata.AttributeAggregation))
		case "query_aggr_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeComplete, metadata.AttributeAggregation))
		case "query_aggr_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeError, metadata.AttributeAggregation))
		case "query_basic_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeAbort, metadata.AttributeBasic))
		case "query_basic_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeComplete, metadata.AttributeBasic))
		case "query_basic_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeError, metadata.AttributeBasic))
		case "query_ops_bg_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeAbort, metadata.AttributeOpsBackground))
		case "query_ops_bg_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeComplete, metadata.AttributeOpsBackground))
		case "query_ops_bg_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeError, metadata.AttributeOpsBackground))
		case "query_udf_bg_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeAbort, metadata.AttributeUdfBackground))
		case "query_udf_bg_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeComplete, metadata.AttributeUdfBackground))
		case "query_udf_bg_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeError, metadata.AttributeUdfBackground))

		// PI queries
		case "pi_query_aggr_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributePrimary, metadata.AttributeAbort, metadata.AttributeAggregation))
		case "pi_query_aggr_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributePrimary, metadata.AttributeComplete, metadata.AttributeAggregation))
		case "pi_query_aggr_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributePrimary, metadata.AttributeError, metadata.AttributeAggregation))
		case "pi_query_long_basic_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributePrimary, metadata.AttributeAbort, metadata.AttributeLongBasic))
		case "pi_query_long_basic_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributePrimary, metadata.AttributeComplete, metadata.AttributeLongBasic))
		case "pi_query_long_basic_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributePrimary, metadata.AttributeError, metadata.AttributeLongBasic))
		case "pi_query_ops_bg_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributePrimary, metadata.AttributeAbort, metadata.AttributeOpsBackground))
		case "pi_query_ops_bg_basic_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributePrimary, metadata.AttributeComplete, metadata.AttributeOpsBackground))
		case "pi_query_ops_bg_basic_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributePrimary, metadata.AttributeError, metadata.AttributeOpsBackground))
		case "pi_query_short_basic_timeout":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributePrimary, metadata.AttributeTimeout, metadata.AttributeShortBasic))
		case "pi_query_short_basic_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributePrimary, metadata.AttributeComplete, metadata.AttributeShortBasic))
		case "pi_query_short_basic_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributePrimary, metadata.AttributeError, metadata.AttributeShortBasic))
		case "pi_query_udf_bg_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributePrimary, metadata.AttributeAbort, metadata.AttributeUdfBackground))
		case "pi_query_udf_bg_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributePrimary, metadata.AttributeComplete, metadata.AttributeUdfBackground))
		case "pi_query_udf_bg_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributePrimary, metadata.AttributeError, metadata.AttributeUdfBackground))

		// SI queries
		case "si_query_aggr_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeAbort, metadata.AttributeAggregation))
		case "si_query_aggr_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeComplete, metadata.AttributeAggregation))
		case "si_query_aggr_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeError, metadata.AttributeAggregation))
		case "si_query_long_basic_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeAbort, metadata.AttributeLongBasic))
		case "si_query_long_basic_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeComplete, metadata.AttributeLongBasic))
		case "si_query_long_basic_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeError, metadata.AttributeLongBasic))
		case "si_query_ops_bg_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeAbort, metadata.AttributeOpsBackground))
		case "si_query_ops_bg_basic_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeComplete, metadata.AttributeOpsBackground))
		case "si_query_ops_bg_basic_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeError, metadata.AttributeOpsBackground))
		case "si_query_short_basic_timeout":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeTimeout, metadata.AttributeShortBasic))
		case "si_query_short_basic_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeComplete, metadata.AttributeShortBasic))
		case "si_query_short_basic_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeError, metadata.AttributeShortBasic))
		case "si_query_udf_bg_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeAbort, metadata.AttributeUdfBackground))
		case "si_query_udf_bg_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeComplete, metadata.AttributeUdfBackground))
		case "si_query_udf_bg_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeSecondary, metadata.AttributeError, metadata.AttributeUdfBackground))

		// GeoJSON region queries
		case "geo_region_query_cells":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceGeojsonRegionQueryCellsDataPoint(now, v))
		case "geo_region_query_falsepos":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceGeojsonRegionQueryFalsePositiveDataPoint(now, v))
		case "geo_region_query_points":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceGeojsonRegionQueryPointsDataPoint(now, v))
		case "geo_region_query_reqs":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceGeojsonRegionQueryRequestsDataPoint(now, v))

		// Compression

		// 'Delete' transactions
		case "client_delete_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeError, metadata.AttributeDelete))
		case "client_delete_filtered_out":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeFilteredOut, metadata.AttributeDelete))
		case "client_delete_not_found":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeNotFound, metadata.AttributeDelete))
		case "client_delete_success":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeSuccess, metadata.AttributeDelete))
		case "client_delete_timeout":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTimeout, metadata.AttributeDelete))

		// 'Read' transactions
		case "client_read_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeError, metadata.AttributeRead))
		case "client_read_filtered_out":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeFilteredOut, metadata.AttributeRead))
		case "client_read_not_found":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeNotFound, metadata.AttributeRead))
		case "client_read_success":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeSuccess, metadata.AttributeRead))
		case "client_read_timeout":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTimeout, metadata.AttributeRead))

		// UDF transactions
		case "client_udf_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeError, metadata.AttributeUdf))
		case "client_udf_filtered_out":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeFilteredOut, metadata.AttributeUdf))
		case "client_udf_not_found":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeNotFound, metadata.AttributeUdf))
		case "client_udf_success":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeSuccess, metadata.AttributeUdf))
		case "client_udf_timeout":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTimeout, metadata.AttributeUdf))

		// 'Write' transactions
		case "client_write_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeError, metadata.AttributeWrite))
		case "client_write_filtered_out":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeFilteredOut, metadata.AttributeWrite))
		case "client_write_not_found":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeNotFound, metadata.AttributeWrite))
		case "client_write_success":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeSuccess, metadata.AttributeWrite))
		case "client_write_timeout":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceTransactionCountDataPoint(now, v, metadata.AttributeTimeout, metadata.AttributeWrite))

		}
	}

	r.mb.EmitForResource(metadata.WithAerospikeNamespace(info["name"]), metadata.WithAerospikeNodeName(info["node"]))
	r.logger.Debug("finished emitNamespace")
}

// addPartialIfError adds a partial error if the given error isn't nil
func addPartialIfError(errs *scrapererror.ScrapeErrors, err error) {
	if err != nil {
		errs.AddPartial(1, err)
	}
}
