// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aerospikereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver"

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"time"

	as "github.com/aerospike/aerospike-client-go/v8"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"
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
func newAerospikeReceiver(params receiver.Settings, cfg *Config, consumer consumer.Metrics) (*aerospikeReceiver, error) {
	var err error
	var tlsCfg *tls.Config
	if cfg.TLS != nil {
		tlsCfg, err = cfg.TLS.LoadTLSConfig(context.Background())
		if err != nil {
			return nil, fmt.Errorf("%w: %s", errFailedTLSLoad, err.Error())
		}
	}

	host, portStr, err := net.SplitHostPort(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errBadEndpoint, err.Error())
	}

	port, err := strconv.ParseInt(portStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errBadPort, err.Error())
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
				password:              string(cfg.Password),
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
		mb: metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, params),
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
func (r *aerospikeReceiver) scrape(_ context.Context) (pmetric.Metrics, error) {
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
		case "query_tracked":
			addPartialIfError(errs, r.mb.RecordAerospikeNodeQueryTrackedDataPoint(now, v))
		}
	}

	rb := r.mb.NewResourceBuilder()
	rb.SetAerospikeNodeName(info["node"])
	r.mb.EmitForResource(metadata.WithResource(rb.Emit()))
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

		// Pre Aerospike 6.0 query metrics. These were always done on secondary indexes, otherwise they were counted as a scan.
		case "query_aggr_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeAggregation, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultAbort))
		case "query_aggr_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeAggregation, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultComplete))
		case "query_aggr_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeAggregation, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultError))
		case "query_basic_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeBasic, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultAbort))
		case "query_basic_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeBasic, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultComplete))
		case "query_basic_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeBasic, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultError))
		case "query_ops_bg_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeOpsBackground, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultAbort))
		case "query_ops_bg_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeOpsBackground, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultComplete))
		case "query_ops_bg_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeOpsBackground, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultError))
		case "query_udf_bg_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeUdfBackground, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultAbort))
		case "query_udf_bg_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeUdfBackground, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultComplete))
		case "query_udf_bg_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeUdfBackground, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultError))

		// PI queries
		case "pi_query_aggr_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeAggregation, metadata.AttributeIndexTypePrimary, metadata.AttributeQueryResultAbort))
		case "pi_query_aggr_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeAggregation, metadata.AttributeIndexTypePrimary, metadata.AttributeQueryResultComplete))
		case "pi_query_aggr_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeAggregation, metadata.AttributeIndexTypePrimary, metadata.AttributeQueryResultError))
		case "pi_query_long_basic_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeLongBasic, metadata.AttributeIndexTypePrimary, metadata.AttributeQueryResultAbort))
		case "pi_query_long_basic_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeLongBasic, metadata.AttributeIndexTypePrimary, metadata.AttributeQueryResultComplete))
		case "pi_query_long_basic_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeLongBasic, metadata.AttributeIndexTypePrimary, metadata.AttributeQueryResultError))
		case "pi_query_ops_bg_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeOpsBackground, metadata.AttributeIndexTypePrimary, metadata.AttributeQueryResultAbort))
		case "pi_query_ops_bg_basic_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeOpsBackground, metadata.AttributeIndexTypePrimary, metadata.AttributeQueryResultComplete))
		case "pi_query_ops_bg_basic_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeOpsBackground, metadata.AttributeIndexTypePrimary, metadata.AttributeQueryResultError))
		case "pi_query_short_basic_timeout":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeShortBasic, metadata.AttributeIndexTypePrimary, metadata.AttributeQueryResultTimeout))
		case "pi_query_short_basic_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeShortBasic, metadata.AttributeIndexTypePrimary, metadata.AttributeQueryResultComplete))
		case "pi_query_short_basic_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeShortBasic, metadata.AttributeIndexTypePrimary, metadata.AttributeQueryResultError))
		case "pi_query_udf_bg_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeUdfBackground, metadata.AttributeIndexTypePrimary, metadata.AttributeQueryResultAbort))
		case "pi_query_udf_bg_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeUdfBackground, metadata.AttributeIndexTypePrimary, metadata.AttributeQueryResultComplete))
		case "pi_query_udf_bg_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeUdfBackground, metadata.AttributeIndexTypePrimary, metadata.AttributeQueryResultError))

		// SI queries
		case "si_query_aggr_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeAggregation, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultAbort))
		case "si_query_aggr_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeAggregation, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultComplete))
		case "si_query_aggr_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeAggregation, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultError))
		case "si_query_long_basic_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeLongBasic, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultAbort))
		case "si_query_long_basic_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeLongBasic, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultComplete))
		case "si_query_long_basic_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeLongBasic, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultError))
		case "si_query_ops_bg_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeOpsBackground, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultAbort))
		case "si_query_ops_bg_basic_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeOpsBackground, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultComplete))
		case "si_query_ops_bg_basic_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeOpsBackground, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultError))
		case "si_query_short_basic_timeout":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeShortBasic, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultTimeout))
		case "si_query_short_basic_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeShortBasic, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultComplete))
		case "si_query_short_basic_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeShortBasic, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultError))
		case "si_query_udf_bg_abort":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeUdfBackground, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultAbort))
		case "si_query_udf_bg_complete":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeUdfBackground, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultComplete))
		case "si_query_udf_bg_error":
			addPartialIfError(errs, r.mb.RecordAerospikeNamespaceQueryCountDataPoint(now, v, metadata.AttributeQueryTypeUdfBackground, metadata.AttributeIndexTypeSecondary, metadata.AttributeQueryResultError))

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

	rb := r.mb.NewResourceBuilder()
	rb.SetAerospikeNamespace(info["name"])
	rb.SetAerospikeNodeName(info["node"])
	r.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	r.logger.Debug("finished emitNamespace")
}

// addPartialIfError adds a partial error if the given error isn't nil
func addPartialIfError(errs *scrapererror.ScrapeErrors, err error) {
	if err != nil {
		errs.AddPartial(1, err)
	}
}
