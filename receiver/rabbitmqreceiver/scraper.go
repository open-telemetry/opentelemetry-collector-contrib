// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver/internal/models"
)

var errClientNotInit = errors.New("client not initialized")

// Names of metrics in message_stats
const (
	deliverStat        = "deliver"
	publishStat        = "publish"
	ackStat            = "ack"
	dropUnroutableStat = "drop_unroutable"
)

// Metrics to gather from queue message_stats structure
var messageStatMetrics = []string{
	deliverStat,
	publishStat,
	ackStat,
	dropUnroutableStat,
}

// rabbitmqScraper handles scraping of RabbitMQ metrics
type rabbitmqScraper struct {
	client   client
	logger   *zap.Logger
	cfg      *Config
	settings component.TelemetrySettings
	mb       *metadata.MetricsBuilder
}

// newScraper creates a new scraper
func newScraper(logger *zap.Logger, cfg *Config, settings receiver.Settings) *rabbitmqScraper {
	return &rabbitmqScraper{
		logger:   logger,
		cfg:      cfg,
		settings: settings.TelemetrySettings,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
	}
}

// start starts the scraper by creating a new HTTP Client on the scraper
func (r *rabbitmqScraper) start(ctx context.Context, host component.Host) (err error) {
	r.client, err = newClient(ctx, r.cfg, host, r.settings, r.logger)
	return
}

// scrape collects metrics from the RabbitMQ API
func (r *rabbitmqScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())

	// Validate we don't attempt to scrape without initializing the client
	if r.client == nil {
		return pmetric.NewMetrics(), errClientNotInit
	}

	var scrapeErrors scrapererror.ScrapeErrors

	// Collect queue metrics
	if err := r.collectQueueMetrics(ctx, now); err != nil {
		scrapeErrors.AddPartial(0, fmt.Errorf("failed to collect queue metrics: %w", err))
	}

	// Collect node metrics
	if err := r.collectNodeMetrics(ctx, now); err != nil {
		scrapeErrors.AddPartial(0, fmt.Errorf("failed to collect node metrics: %w", err))
	}

	// Emit collected metrics
	metrics := r.mb.Emit()

	// Define err at the correct scope
	err := scrapeErrors.Combine()

	// Return a PartialScrapeError if no metrics were collected
	if err != nil && metrics.ResourceMetrics().Len() == 0 {
		err = scrapererror.NewPartialScrapeError(err, metrics.MetricCount())
	}

	return metrics, err
}

func (r *rabbitmqScraper) collectQueueMetrics(ctx context.Context, now pcommon.Timestamp) error {
	queues, err := r.client.GetQueues(ctx)
	if err != nil {
		return err
	}

	// Collect metrics for each queue
	for _, queue := range queues {
		r.collectQueue(queue, now)
	}
	return nil
}

func (r *rabbitmqScraper) collectNodeMetrics(ctx context.Context, now pcommon.Timestamp) error {
	nodes, err := r.client.GetNodes(ctx)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		r.collectNode(node, now)
	}
	return nil
}

// collectQueue collects metrics
func (r *rabbitmqScraper) collectQueue(queue *models.Queue, now pcommon.Timestamp) {
	r.mb.RecordRabbitmqConsumerCountDataPoint(now, queue.Consumers)
	r.mb.RecordRabbitmqMessageCurrentDataPoint(now, queue.UnacknowledgedMessages, metadata.AttributeMessageStateUnacknowledged)
	r.mb.RecordRabbitmqMessageCurrentDataPoint(now, queue.ReadyMessages, metadata.AttributeMessageStateReady)

	for _, messageStatMetric := range messageStatMetrics {
		// Get metric value
		val, ok := queue.MessageStats[messageStatMetric]
		// A metric may not exist if the actions that increment it do not occur
		if !ok {
			r.logger.Debug("metric not found", zap.String("Metric", messageStatMetric), zap.String("Queue", queue.Name))
			continue
		}

		// Convert to int64
		val64, ok := convertValToInt64(val)
		if !ok {
			// Log warning if the metric is not in the format we expect
			r.logger.Warn("metric not int64", zap.String("Metric", messageStatMetric), zap.String("Queue", queue.Name))
			continue
		}

		switch messageStatMetric {
		case deliverStat:
			r.mb.RecordRabbitmqMessageDeliveredDataPoint(now, val64)
		case publishStat:
			r.mb.RecordRabbitmqMessagePublishedDataPoint(now, val64)
		case ackStat:
			r.mb.RecordRabbitmqMessageAcknowledgedDataPoint(now, val64)
		case dropUnroutableStat:
			r.mb.RecordRabbitmqMessageDroppedDataPoint(now, val64)
		}
	}
	rb := r.mb.NewResourceBuilder()
	rb.SetRabbitmqQueueName(queue.Name)
	rb.SetRabbitmqNodeName(queue.Node)
	rb.SetRabbitmqVhostName(queue.VHost)
	r.mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

// collectNode collects metrics for a specific RabbitMQ node
func (r *rabbitmqScraper) collectNode(node *models.Node, now pcommon.Timestamp) {
	r.mb.RecordRabbitmqNodeDiskFreeDataPoint(now, node.DiskFree)
	r.mb.RecordRabbitmqNodeDiskFreeLimitDataPoint(now, node.DiskFreeLimit)
	r.mb.RecordRabbitmqNodeDiskFreeAlarmDataPoint(now, boolToInt64(node.DiskFreeAlarm))
	r.mb.RecordRabbitmqNodeDiskFreeDetailsRateDataPoint(now, node.DiskFreeRate)

	r.mb.RecordRabbitmqNodeFdUsedDataPoint(now, node.FDUsed)
	r.mb.RecordRabbitmqNodeFdTotalDataPoint(now, node.FDTotal)
	r.mb.RecordRabbitmqNodeFdUsedDetailsRateDataPoint(now, node.FDUsedRate)

	r.mb.RecordRabbitmqNodeSocketsUsedDataPoint(now, node.SocketsUsed)
	r.mb.RecordRabbitmqNodeSocketsTotalDataPoint(now, node.SocketsTotal)
	r.mb.RecordRabbitmqNodeSocketsUsedDetailsRateDataPoint(now, node.SocketsUsedRate)

	r.mb.RecordRabbitmqNodeProcUsedDataPoint(now, node.ProcUsed)
	r.mb.RecordRabbitmqNodeProcTotalDataPoint(now, node.ProcTotal)
	r.mb.RecordRabbitmqNodeProcUsedDetailsRateDataPoint(now, node.ProcUsedRate)

	r.mb.RecordRabbitmqNodeMemUsedDataPoint(now, node.MemUsed)
	r.mb.RecordRabbitmqNodeMemUsedDetailsRateDataPoint(now, node.MemUsedRate)
	r.mb.RecordRabbitmqNodeMemLimitDataPoint(now, node.MemLimit)
	r.mb.RecordRabbitmqNodeMemAlarmDataPoint(now, boolToInt64(node.MemAlarm))

	r.mb.RecordRabbitmqNodeUptimeDataPoint(now, node.Uptime)
	r.mb.RecordRabbitmqNodeRunQueueDataPoint(now, node.RunQueue)
	r.mb.RecordRabbitmqNodeProcessorsDataPoint(now, node.Processors)

	r.mb.RecordRabbitmqNodeContextSwitchesDataPoint(now, node.ContextSwitches)
	r.mb.RecordRabbitmqNodeContextSwitchesDetailsRateDataPoint(now, node.ContextSwitchesRate)

	r.mb.RecordRabbitmqNodeGcNumDataPoint(now, node.GCNum)
	r.mb.RecordRabbitmqNodeGcNumDetailsRateDataPoint(now, node.GCNumRate)
	r.mb.RecordRabbitmqNodeGcBytesReclaimedDataPoint(now, node.GCBytesReclaimed)
	r.mb.RecordRabbitmqNodeGcBytesReclaimedDetailsRateDataPoint(now, node.GCBytesReclaimedRate)

	r.mb.RecordRabbitmqNodeIoReadCountDataPoint(now, node.IOReadCount)
	r.mb.RecordRabbitmqNodeIoReadCountDetailsRateDataPoint(now, node.IOReadCountRate)
	r.mb.RecordRabbitmqNodeIoReadBytesDataPoint(now, node.IOReadBytes)
	r.mb.RecordRabbitmqNodeIoReadBytesDetailsRateDataPoint(now, node.IOReadBytesRate)
	r.mb.RecordRabbitmqNodeIoReadAvgTimeDataPoint(now, node.IOReadAvgTime)
	r.mb.RecordRabbitmqNodeIoReadAvgTimeDetailsRateDataPoint(now, node.IOReadAvgTimeRate)

	r.mb.RecordRabbitmqNodeIoWriteCountDataPoint(now, node.IOWriteCount)
	r.mb.RecordRabbitmqNodeIoWriteCountDetailsRateDataPoint(now, node.IOWriteCountRate)
	r.mb.RecordRabbitmqNodeIoWriteBytesDataPoint(now, node.IOWriteBytes)
	r.mb.RecordRabbitmqNodeIoWriteBytesDetailsRateDataPoint(now, node.IOWriteBytesRate)
	r.mb.RecordRabbitmqNodeIoWriteAvgTimeDataPoint(now, node.IOWriteAvgTime)
	r.mb.RecordRabbitmqNodeIoWriteAvgTimeDetailsRateDataPoint(now, node.IOWriteAvgTimeRate)

	r.mb.RecordRabbitmqNodeIoSyncCountDataPoint(now, node.IOSyncCount)
	r.mb.RecordRabbitmqNodeIoSyncCountDetailsRateDataPoint(now, node.IOSyncCountRate)
	r.mb.RecordRabbitmqNodeIoSyncAvgTimeDataPoint(now, node.IOSyncAvgTime)
	r.mb.RecordRabbitmqNodeIoSyncAvgTimeDetailsRateDataPoint(now, node.IOSyncAvgTimeRate)

	r.mb.RecordRabbitmqNodeIoSeekCountDataPoint(now, node.IOSeekCount)
	r.mb.RecordRabbitmqNodeIoSeekCountDetailsRateDataPoint(now, node.IOSeekCountRate)
	r.mb.RecordRabbitmqNodeIoSeekAvgTimeDataPoint(now, node.IOSeekAvgTime)
	r.mb.RecordRabbitmqNodeIoSeekAvgTimeDetailsRateDataPoint(now, node.IOSeekAvgTimeRate)

	r.mb.RecordRabbitmqNodeIoReopenCountDataPoint(now, node.IOReopenCount)
	r.mb.RecordRabbitmqNodeIoReopenCountDetailsRateDataPoint(now, node.IOReopenCountRate)

	r.mb.RecordRabbitmqNodeMnesiaRAMTxCountDataPoint(now, node.MnesiaRAMTxCount)
	r.mb.RecordRabbitmqNodeMnesiaRAMTxCountDetailsRateDataPoint(now, node.MnesiaRAMTxRate)
	r.mb.RecordRabbitmqNodeMnesiaDiskTxCountDataPoint(now, node.MnesiaDiskTxCount)
	r.mb.RecordRabbitmqNodeMnesiaDiskTxCountDetailsRateDataPoint(now, node.MnesiaDiskTxRate)

	r.mb.RecordRabbitmqNodeMsgStoreReadCountDataPoint(now, node.MsgStoreReadCount)
	r.mb.RecordRabbitmqNodeMsgStoreReadCountDetailsRateDataPoint(now, node.MsgStoreReadRate)
	r.mb.RecordRabbitmqNodeMsgStoreWriteCountDataPoint(now, node.MsgStoreWriteCount)
	r.mb.RecordRabbitmqNodeMsgStoreWriteCountDetailsRateDataPoint(now, node.MsgStoreWriteRate)
	r.mb.RecordRabbitmqNodeQueueIndexWriteCountDataPoint(now, node.QueueIndexWriteCount)
	r.mb.RecordRabbitmqNodeQueueIndexWriteCountDetailsRateDataPoint(now, node.QueueIndexWriteRate)
	r.mb.RecordRabbitmqNodeQueueIndexReadCountDataPoint(now, node.QueueIndexReadCount)
	r.mb.RecordRabbitmqNodeQueueIndexReadCountDetailsRateDataPoint(now, node.QueueIndexReadRate)

	r.mb.RecordRabbitmqNodeConnectionCreatedDataPoint(now, node.ConnectionCreated)
	r.mb.RecordRabbitmqNodeConnectionCreatedDetailsRateDataPoint(now, node.ConnectionCreatedRate)
	r.mb.RecordRabbitmqNodeConnectionClosedDataPoint(now, node.ConnectionClosed)
	r.mb.RecordRabbitmqNodeConnectionClosedDetailsRateDataPoint(now, node.ConnectionClosedRate)

	r.mb.RecordRabbitmqNodeChannelCreatedDataPoint(now, node.ChannelCreated)
	r.mb.RecordRabbitmqNodeChannelCreatedDetailsRateDataPoint(now, node.ChannelCreatedRate)
	r.mb.RecordRabbitmqNodeChannelClosedDataPoint(now, node.ChannelClosed)
	r.mb.RecordRabbitmqNodeChannelClosedDetailsRateDataPoint(now, node.ChannelClosedRate)

	r.mb.RecordRabbitmqNodeQueueDeclaredDataPoint(now, node.QueueDeclared)
	r.mb.RecordRabbitmqNodeQueueDeclaredDetailsRateDataPoint(now, node.QueueDeclaredRate)
	r.mb.RecordRabbitmqNodeQueueCreatedDataPoint(now, node.QueueCreated)
	r.mb.RecordRabbitmqNodeQueueCreatedDetailsRateDataPoint(now, node.QueueCreatedRate)
	r.mb.RecordRabbitmqNodeQueueDeletedDataPoint(now, node.QueueDeleted)
	r.mb.RecordRabbitmqNodeQueueDeletedDetailsRateDataPoint(now, node.QueueDeletedRate)

	rb := r.mb.NewResourceBuilder()
	rb.SetRabbitmqNodeName(node.Name)
	r.mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// convertValToInt64 values from message state unmarshal as float64s but should be int64.
// Need to do a double cast to get an int64.
// This should never fail but worth checking just in case.
func convertValToInt64(val any) (int64, bool) {
	f64Val, ok := val.(float64)
	if !ok {
		return 0, ok
	}

	return int64(f64Val), true
}
