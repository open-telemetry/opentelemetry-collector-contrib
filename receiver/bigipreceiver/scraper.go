// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bigipreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver"

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver/internal/models"
)

// custom errors
var (
	errClientNotInit    = errors.New("client not initialized")
	errScrapedNoMetrics = errors.New("failed to scrape any metrics")
)

// bigipScraper handles scraping of Big-IP metrics
type bigipScraper struct {
	client   client
	logger   *zap.Logger
	cfg      *Config
	settings component.TelemetrySettings
	mb       *metadata.MetricsBuilder
}

// newScraper creates an initialized bigipScraper
func newScraper(logger *zap.Logger, cfg *Config, settings receiver.CreateSettings) *bigipScraper {
	return &bigipScraper{
		logger:   logger,
		cfg:      cfg,
		settings: settings.TelemetrySettings,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
	}
}

// start initializes a new big-ip client for the scraper
func (s *bigipScraper) start(_ context.Context, host component.Host) (err error) {
	s.client, err = newClient(s.cfg, host, s.settings, s.logger)
	return
}

// scrape collects and creates OTEL metrics from a Big-IP environment
func (s *bigipScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())

	// validate we don't attempt to scrape without initializing the client
	if s.client == nil {
		return pmetric.NewMetrics(), errClientNotInit
	}

	collectedMetrics := false

	// initialize auth token
	err := s.client.GetNewToken(ctx)
	if err != nil {
		return pmetric.NewMetrics(), err
	}

	var scrapeErrors scrapererror.ScrapeErrors
	// scrape metrics for virtual servers
	virtualServers, err := s.client.GetVirtualServers(ctx)
	if err != nil {
		scrapeErrors.AddPartial(1, err)
		s.logger.Warn("Failed to scrape virtual server metrics", zap.Error(err))
	} else {
		collectedMetrics = true
		for key := range virtualServers.Entries {
			virtualServerStats := virtualServers.Entries[key]
			s.collectVirtualServers(&virtualServerStats, now)
		}
	}

	// scrape metrics for pools
	pools, err := s.client.GetPools(ctx)
	if err != nil {
		scrapeErrors.AddPartial(1, err)
		s.logger.Warn("Failed to scrape pool metrics", zap.Error(err))
	} else {
		collectedMetrics = true
		for key := range pools.Entries {
			poolStats := pools.Entries[key]
			s.collectPools(&poolStats, now)
		}
	}

	// scrape metrics for pool members
	poolMembers, err := s.client.GetPoolMembers(ctx, pools)
	if errors.Is(err, errCollectedNoPoolMembers) {
		scrapeErrors.AddPartial(1, err)
		s.logger.Warn("Failed to scrape pool member metrics", zap.Error(err))
	} else {
		if err != nil {
			scrapeErrors.AddPartial(1, err)
			s.logger.Warn("Failed to scrape some pool member metrics", zap.Error(err))
		}

		collectedMetrics = true
		for key := range poolMembers.Entries {
			poolMemberStats := poolMembers.Entries[key]
			s.collectPoolMembers(&poolMemberStats, now)
		}
	}

	// scrape metrics for nodes
	nodes, err := s.client.GetNodes(ctx)
	if err != nil {
		scrapeErrors.AddPartial(1, err)
		s.logger.Warn("Failed to scrape node metrics", zap.Error(err))
	} else {
		collectedMetrics = true
		for key := range nodes.Entries {
			nodeStats := nodes.Entries[key]
			s.collectNodes(&nodeStats, now)
		}
	}

	if !collectedMetrics {
		return pmetric.NewMetrics(), errScrapedNoMetrics
	}

	return s.mb.Emit(), scrapeErrors.Combine()
}

// collectVirtualServers collects virtual server metrics
func (s *bigipScraper) collectVirtualServers(virtualServerStats *models.VirtualServerStats, now pcommon.Timestamp) {
	rb := s.mb.NewResourceBuilder()
	rb.SetBigipVirtualServerName(virtualServerStats.NestedStats.Entries.Name.Description)
	rb.SetBigipVirtualServerDestination(virtualServerStats.NestedStats.Entries.Destination.Description)
	rb.SetBigipPoolName(virtualServerStats.NestedStats.Entries.PoolName.Description)
	rmb := s.mb.ResourceMetricsBuilder(rb.Emit())

	rmb.RecordBigipVirtualServerDataTransmittedDataPoint(now, virtualServerStats.NestedStats.Entries.
		ClientsideBitsIn.Value, metadata.AttributeDirectionReceived)
	rmb.RecordBigipVirtualServerDataTransmittedDataPoint(now, virtualServerStats.NestedStats.Entries.
		ClientsideBitsOut.Value, metadata.AttributeDirectionSent)
	rmb.RecordBigipVirtualServerConnectionCountDataPoint(now, virtualServerStats.NestedStats.Entries.
		ClientsideCurConns.Value)
	rmb.RecordBigipVirtualServerPacketCountDataPoint(now, virtualServerStats.NestedStats.Entries.ClientsidePktsIn.
		Value, metadata.AttributeDirectionReceived)
	rmb.RecordBigipVirtualServerPacketCountDataPoint(now, virtualServerStats.NestedStats.Entries.ClientsidePktsOut.
		Value, metadata.AttributeDirectionSent)
	rmb.RecordBigipVirtualServerRequestCountDataPoint(now, virtualServerStats.NestedStats.Entries.TotalRequests.Value)

	availability := virtualServerStats.NestedStats.Entries.AvailabilityState.Description
	switch {
	case strings.HasPrefix(availability, "available"):
		rmb.RecordBigipVirtualServerAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusOffline)
		rmb.RecordBigipVirtualServerAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusUnknown)
		rmb.RecordBigipVirtualServerAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusAvailable)
	case strings.HasPrefix(availability, "offline"):
		rmb.RecordBigipVirtualServerAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusOffline)
		rmb.RecordBigipVirtualServerAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusUnknown)
		rmb.RecordBigipVirtualServerAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusAvailable)
	default:
		rmb.RecordBigipVirtualServerAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusOffline)
		rmb.RecordBigipVirtualServerAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusUnknown)
		rmb.RecordBigipVirtualServerAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusAvailable)
	}

	enabled := virtualServerStats.NestedStats.Entries.EnabledState.Description
	if strings.HasPrefix(enabled, "enabled") {
		rmb.RecordBigipVirtualServerEnabledDataPoint(now, 0, metadata.AttributeEnabledStatusDisabled)
		rmb.RecordBigipVirtualServerEnabledDataPoint(now, 1, metadata.AttributeEnabledStatusEnabled)
	} else {
		rmb.RecordBigipVirtualServerEnabledDataPoint(now, 1, metadata.AttributeEnabledStatusDisabled)
		rmb.RecordBigipVirtualServerEnabledDataPoint(now, 0, metadata.AttributeEnabledStatusEnabled)
	}
}

// collectPools collects pool metrics
func (s *bigipScraper) collectPools(poolStats *models.PoolStats, now pcommon.Timestamp) {
	rb := s.mb.NewResourceBuilder()
	rb.SetBigipPoolName(poolStats.NestedStats.Entries.Name.Description)
	rmb := s.mb.ResourceMetricsBuilder(rb.Emit())

	rmb.RecordBigipPoolDataTransmittedDataPoint(now, poolStats.NestedStats.Entries.ServersideBitsIn.Value,
		metadata.AttributeDirectionReceived)
	rmb.RecordBigipPoolDataTransmittedDataPoint(now, poolStats.NestedStats.Entries.ServersideBitsOut.Value,
		metadata.AttributeDirectionSent)
	rmb.RecordBigipPoolConnectionCountDataPoint(now, poolStats.NestedStats.Entries.ServersideCurConns.Value)
	rmb.RecordBigipPoolPacketCountDataPoint(now, poolStats.NestedStats.Entries.ServersidePktsIn.Value,
		metadata.AttributeDirectionReceived)
	rmb.RecordBigipPoolPacketCountDataPoint(now, poolStats.NestedStats.Entries.ServersidePktsOut.Value,
		metadata.AttributeDirectionSent)
	rmb.RecordBigipPoolRequestCountDataPoint(now, poolStats.NestedStats.Entries.TotalRequests.Value)
	rmb.RecordBigipPoolMemberCountDataPoint(now, poolStats.NestedStats.Entries.ActiveMemberCount.Value,
		metadata.AttributeActiveStatusActive)
	inactiveCount := poolStats.NestedStats.Entries.TotalMemberCount.Value - poolStats.NestedStats.Entries.ActiveMemberCount.Value
	rmb.RecordBigipPoolMemberCountDataPoint(now, inactiveCount, metadata.AttributeActiveStatusInactive)

	availability := poolStats.NestedStats.Entries.AvailabilityState.Description
	switch {
	case strings.HasPrefix(availability, "available"):
		rmb.RecordBigipPoolAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusOffline)
		rmb.RecordBigipPoolAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusUnknown)
		rmb.RecordBigipPoolAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusAvailable)
	case strings.HasPrefix(availability, "offline"):
		rmb.RecordBigipPoolAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusOffline)
		rmb.RecordBigipPoolAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusUnknown)
		rmb.RecordBigipPoolAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusAvailable)
	default:
		rmb.RecordBigipPoolAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusOffline)
		rmb.RecordBigipPoolAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusUnknown)
		rmb.RecordBigipPoolAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusAvailable)
	}

	enabled := poolStats.NestedStats.Entries.EnabledState.Description
	if strings.HasPrefix(enabled, "enabled") {
		rmb.RecordBigipPoolEnabledDataPoint(now, 0, metadata.AttributeEnabledStatusDisabled)
		rmb.RecordBigipPoolEnabledDataPoint(now, 1, metadata.AttributeEnabledStatusEnabled)
	} else {
		rmb.RecordBigipPoolEnabledDataPoint(now, 1, metadata.AttributeEnabledStatusDisabled)
		rmb.RecordBigipPoolEnabledDataPoint(now, 0, metadata.AttributeEnabledStatusEnabled)
	}
}

// collectPoolMembers collects pool member metrics
func (s *bigipScraper) collectPoolMembers(poolMemberStats *models.PoolMemberStats, now pcommon.Timestamp) {
	rb := s.mb.NewResourceBuilder()
	rb.SetBigipPoolMemberName(fmt.Sprintf("%s:%d", poolMemberStats.NestedStats.Entries.Name.Description, poolMemberStats.NestedStats.Entries.Port.Value))
	rb.SetBigipPoolMemberIPAddress(poolMemberStats.NestedStats.Entries.IPAddress.Description)
	rb.SetBigipPoolName(poolMemberStats.NestedStats.Entries.PoolName.Description)
	rmb := s.mb.ResourceMetricsBuilder(rb.Emit())

	rmb.RecordBigipPoolMemberDataTransmittedDataPoint(now, poolMemberStats.NestedStats.Entries.ServersideBitsIn.
		Value, metadata.AttributeDirectionReceived)
	rmb.RecordBigipPoolMemberDataTransmittedDataPoint(now, poolMemberStats.NestedStats.Entries.ServersideBitsOut.
		Value, metadata.AttributeDirectionSent)
	rmb.RecordBigipPoolMemberConnectionCountDataPoint(now, poolMemberStats.NestedStats.Entries.ServersideCurConns.Value)
	rmb.RecordBigipPoolMemberPacketCountDataPoint(now, poolMemberStats.NestedStats.Entries.ServersidePktsIn.Value,
		metadata.AttributeDirectionReceived)
	rmb.RecordBigipPoolMemberPacketCountDataPoint(now, poolMemberStats.NestedStats.Entries.ServersidePktsOut.Value,
		metadata.AttributeDirectionSent)
	rmb.RecordBigipPoolMemberRequestCountDataPoint(now, poolMemberStats.NestedStats.Entries.TotalRequests.Value)
	rmb.RecordBigipPoolMemberSessionCountDataPoint(now, poolMemberStats.NestedStats.Entries.CurSessions.Value)

	availability := poolMemberStats.NestedStats.Entries.AvailabilityState.Description
	switch {
	case strings.HasPrefix(availability, "available"):
		rmb.RecordBigipPoolMemberAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusOffline)
		rmb.RecordBigipPoolMemberAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusUnknown)
		rmb.RecordBigipPoolMemberAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusAvailable)
	case strings.HasPrefix(availability, "offline"):
		rmb.RecordBigipPoolMemberAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusOffline)
		rmb.RecordBigipPoolMemberAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusUnknown)
		rmb.RecordBigipPoolMemberAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusAvailable)
	default:
		rmb.RecordBigipPoolMemberAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusOffline)
		rmb.RecordBigipPoolMemberAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusUnknown)
		rmb.RecordBigipPoolMemberAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusAvailable)
	}

	enabled := poolMemberStats.NestedStats.Entries.EnabledState.Description
	if strings.HasPrefix(enabled, "enabled") {
		rmb.RecordBigipPoolMemberEnabledDataPoint(now, 0, metadata.AttributeEnabledStatusDisabled)
		rmb.RecordBigipPoolMemberEnabledDataPoint(now, 1, metadata.AttributeEnabledStatusEnabled)
	} else {
		rmb.RecordBigipPoolMemberEnabledDataPoint(now, 1, metadata.AttributeEnabledStatusDisabled)
		rmb.RecordBigipPoolMemberEnabledDataPoint(now, 0, metadata.AttributeEnabledStatusEnabled)
	}
}

// collectNodes collects node metrics
func (s *bigipScraper) collectNodes(nodeStats *models.NodeStats, now pcommon.Timestamp) {
	rb := s.mb.NewResourceBuilder()
	rb.SetBigipNodeName(nodeStats.NestedStats.Entries.Name.Description)
	rb.SetBigipNodeIPAddress(nodeStats.NestedStats.Entries.IPAddress.Description)
	rmb := s.mb.ResourceMetricsBuilder(rb.Emit())

	rmb.RecordBigipNodeDataTransmittedDataPoint(now, nodeStats.NestedStats.Entries.ServersideBitsIn.Value,
		metadata.AttributeDirectionReceived)
	rmb.RecordBigipNodeDataTransmittedDataPoint(now, nodeStats.NestedStats.Entries.ServersideBitsOut.Value,
		metadata.AttributeDirectionSent)
	rmb.RecordBigipNodeConnectionCountDataPoint(now, nodeStats.NestedStats.Entries.ServersideCurConns.Value)
	rmb.RecordBigipNodePacketCountDataPoint(now, nodeStats.NestedStats.Entries.ServersidePktsIn.Value,
		metadata.AttributeDirectionReceived)
	rmb.RecordBigipNodePacketCountDataPoint(now, nodeStats.NestedStats.Entries.ServersidePktsOut.Value,
		metadata.AttributeDirectionSent)
	rmb.RecordBigipNodeRequestCountDataPoint(now, nodeStats.NestedStats.Entries.TotalRequests.Value)
	rmb.RecordBigipNodeSessionCountDataPoint(now, nodeStats.NestedStats.Entries.CurSessions.Value)

	availability := nodeStats.NestedStats.Entries.AvailabilityState.Description
	switch {
	case strings.HasPrefix(availability, "available"):
		rmb.RecordBigipNodeAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusOffline)
		rmb.RecordBigipNodeAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusUnknown)
		rmb.RecordBigipNodeAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusAvailable)
	case strings.HasPrefix(availability, "offline"):
		rmb.RecordBigipNodeAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusOffline)
		rmb.RecordBigipNodeAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusUnknown)
		rmb.RecordBigipNodeAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusAvailable)
	default:
		rmb.RecordBigipNodeAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusOffline)
		rmb.RecordBigipNodeAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusUnknown)
		rmb.RecordBigipNodeAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusAvailable)
	}

	enabled := nodeStats.NestedStats.Entries.EnabledState.Description
	if strings.HasPrefix(enabled, "enabled") {
		rmb.RecordBigipNodeEnabledDataPoint(now, 0, metadata.AttributeEnabledStatusDisabled)
		rmb.RecordBigipNodeEnabledDataPoint(now, 1, metadata.AttributeEnabledStatusEnabled)
	} else {
		rmb.RecordBigipNodeEnabledDataPoint(now, 1, metadata.AttributeEnabledStatusDisabled)
		rmb.RecordBigipNodeEnabledDataPoint(now, 0, metadata.AttributeEnabledStatusEnabled)
	}
}
