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
	"go.uber.org/zap"
)

// aerospikeReceiver is a metrics receiver using the Aerospike interface to collect
type aerospikeReceiver struct {
	params   component.ReceiverCreateSettings
	config   *Config
	consumer consumer.Metrics
	host     string // host/IP of configured Aerospike node
	port     int    // port of configured Aerospike node
	mb       *metadata.MetricsBuilder
	logger   *zap.Logger
}

// newAerospikeReceiver creates a new aerospikeReceiver connected to the endpoint provided in cfg
//
// If the host or port can't be parsed from endpoint, an error is returned.
func newAerospikeReceiver(params component.ReceiverCreateSettings, cfg *Config, consumer consumer.Metrics) (*aerospikeReceiver, error) {
	host, portStr, err := net.SplitHostPort(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("endpoint: %w", err)
	}
	var port int
	if portStr == "" {
		port = 3000
	} else {
		portI, err := strconv.ParseInt(portStr, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("port: %w", err)
		}
		port = int(portI)
	}

	return &aerospikeReceiver{
		params:   params,
		config:   cfg,
		consumer: consumer,
		host:     host,
		port:     int(port),
		mb:       metadata.NewMetricsBuilder(cfg.Metrics),
		logger:   params.Logger.Named("aerospikereceiver"),
	}, nil
}

func (r *aerospikeReceiver) scrape(ctx context.Context) (pmetric.Metrics, error) {
	errs := &scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now().UTC())
	client, err := newASClient(r.host, r.port, r.config.Timeout)
	if err != nil {
		r.logger.Warn(fmt.Sprintf("failed to connect: %s", err.Error()))
		return r.mb.Emit(), nil
	}
	defer client.Close()

	info, err := client.Info()
	if err != nil {
		return r.mb.Emit(), err
	}
	r.emitNode(info, client, now)

	if r.config.CollectClusterMetrics {
		r.logger.Debug("Collecting peer nodes")
		for _, n := range info.Services {
			r.scrapeDiscoveredNode(n, now, errs)
		}
	}

	return r.mb.Emit(), errs.Combine()
}

// scrapeNode collects metrics from a single Aerospike node
func (r *aerospikeReceiver) scrapeNode(client Aerospike, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	info, err := client.Info()
	if err != nil {
		errs.AddPartial(-1, err)
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
		r.params.Logger.Sugar().Warnf("failed splitting endpoint %s: %s", endpoint, err)
		return
	}
	port, err := strconv.ParseInt(portStr, 10, 32)
	if err != nil {
		r.logger.Warn(fmt.Sprintf("failed parsing port %s: %s", portStr, err))
		return
	}
	nClient, err := newASClient(host, int(port), r.config.Timeout)
	if err != nil {
		r.logger.Warn(err.Error())
		return
	}
	defer nClient.Close()

	r.scrapeNode(nClient, now, errs)
}

// emitNode records node metrics and emits the resource. It collects namespace metrics for each namespace on the node
//
// The given client is used to collect namespace metrics, which may be connected to a discovered node
func (r *aerospikeReceiver) emitNode(info *model.NodeInfo, client Aerospike, now pcommon.Timestamp) {
	if stats := info.Statistics; stats != nil {
		if stats.ClientConnections != nil {
			r.mb.RecordAerospikeNodeConnectionOpenDataPoint(now, *stats.ClientConnections, metadata.AttributeConnectionType.Client)
		}
		if stats.FabricConnections != nil {
			r.mb.RecordAerospikeNodeConnectionOpenDataPoint(now, *stats.FabricConnections, metadata.AttributeConnectionType.Fabric)
		}
		if stats.HeartbeatConnections != nil {
			r.mb.RecordAerospikeNodeConnectionOpenDataPoint(now, *stats.HeartbeatConnections, metadata.AttributeConnectionType.Heartbeat)
		}

		if stats.ClientConnectionsClosed != nil {
			r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, *stats.ClientConnectionsClosed, metadata.AttributeConnectionType.Client, metadata.AttributeConnectionOp.Close)
		}
		if stats.ClientConnectionsOpened != nil {
			r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, *stats.ClientConnectionsOpened, metadata.AttributeConnectionType.Client, metadata.AttributeConnectionOp.Open)
		}
		if stats.FabricConnectionsClosed != nil {
			r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, *stats.FabricConnectionsClosed, metadata.AttributeConnectionType.Fabric, metadata.AttributeConnectionOp.Close)
		}
		if stats.FabricConnectionsOpened != nil {
			r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, *stats.FabricConnectionsOpened, metadata.AttributeConnectionType.Fabric, metadata.AttributeConnectionOp.Open)
		}
		if stats.HeartbeatConnectionsClosed != nil {
			r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, *stats.HeartbeatConnectionsClosed, metadata.AttributeConnectionType.Heartbeat, metadata.AttributeConnectionOp.Close)
		}
		if stats.HeartbeatConnectionsOpened != nil {
			r.mb.RecordAerospikeNodeConnectionCountDataPoint(now, *stats.HeartbeatConnectionsOpened, metadata.AttributeConnectionType.Heartbeat, metadata.AttributeConnectionOp.Open)
		}
	}

	r.mb.EmitForResource(metadata.WithNodeName(info.Name))

	if info.Namespaces != nil {
		for _, n := range info.Namespaces {
			nInfo, err := client.NamespaceInfo(n)
			if err != nil {
				r.params.Logger.Sugar().Warnf("failed getting namespace %s: %s", n, err.Error())
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
		r.mb.RecordAerospikeNamespaceMemoryUsageDataPoint(now, *info.MemoryUsedDataBytes, metadata.AttributeNamespaceComponent.Data)
	}
	if info.MemoryUsedIndexBytes != nil {
		r.mb.RecordAerospikeNamespaceMemoryUsageDataPoint(now, *info.MemoryUsedIndexBytes, metadata.AttributeNamespaceComponent.Index)
	}
	if info.MemoryUsedSIndexBytes != nil {
		r.mb.RecordAerospikeNamespaceMemoryUsageDataPoint(now, *info.MemoryUsedSIndexBytes, metadata.AttributeNamespaceComponent.Sindex)
	}
	if info.MemoryUsedSetIndexBytes != nil {
		r.mb.RecordAerospikeNamespaceMemoryUsageDataPoint(now, *info.MemoryUsedSetIndexBytes, metadata.AttributeNamespaceComponent.SetIndex)
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
