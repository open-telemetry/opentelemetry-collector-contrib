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
	rb       *metadata.ResourceBuilder
	mb       *metadata.MetricsBuilder
}

// newScraper creates an initialized bigipScraper
func newScraper(logger *zap.Logger, cfg *Config, settings receiver.CreateSettings) *bigipScraper {
	return &bigipScraper{
		logger:   logger,
		cfg:      cfg,
		settings: settings.TelemetrySettings,
		rb:       metadata.NewResourceBuilder(cfg.MetricsBuilderConfig.ResourceAttributes),
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
	s.mb.RecordBigipVirtualServerDataTransmittedDataPoint(now, virtualServerStats.NestedStats.Entries.ClientsideBitsIn.Value, metadata.AttributeDirectionReceived)
	s.mb.RecordBigipVirtualServerDataTransmittedDataPoint(now, virtualServerStats.NestedStats.Entries.ClientsideBitsOut.Value, metadata.AttributeDirectionSent)
	s.mb.RecordBigipVirtualServerConnectionCountDataPoint(now, virtualServerStats.NestedStats.Entries.ClientsideCurConns.Value)
	s.mb.RecordBigipVirtualServerPacketCountDataPoint(now, virtualServerStats.NestedStats.Entries.ClientsidePktsIn.Value, metadata.AttributeDirectionReceived)
	s.mb.RecordBigipVirtualServerPacketCountDataPoint(now, virtualServerStats.NestedStats.Entries.ClientsidePktsOut.Value, metadata.AttributeDirectionSent)
	s.mb.RecordBigipVirtualServerRequestCountDataPoint(now, virtualServerStats.NestedStats.Entries.TotalRequests.Value)

	availability := virtualServerStats.NestedStats.Entries.AvailabilityState.Description
	switch {
	case strings.HasPrefix(availability, "available"):
		s.mb.RecordBigipVirtualServerAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusOffline)
		s.mb.RecordBigipVirtualServerAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusUnknown)
		s.mb.RecordBigipVirtualServerAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusAvailable)
	case strings.HasPrefix(availability, "offline"):
		s.mb.RecordBigipVirtualServerAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusOffline)
		s.mb.RecordBigipVirtualServerAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusUnknown)
		s.mb.RecordBigipVirtualServerAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusAvailable)
	default:
		s.mb.RecordBigipVirtualServerAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusOffline)
		s.mb.RecordBigipVirtualServerAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusUnknown)
		s.mb.RecordBigipVirtualServerAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusAvailable)
	}

	enabled := virtualServerStats.NestedStats.Entries.EnabledState.Description
	if strings.HasPrefix(enabled, "enabled") {
		s.mb.RecordBigipVirtualServerEnabledDataPoint(now, 0, metadata.AttributeEnabledStatusDisabled)
		s.mb.RecordBigipVirtualServerEnabledDataPoint(now, 1, metadata.AttributeEnabledStatusEnabled)
	} else {
		s.mb.RecordBigipVirtualServerEnabledDataPoint(now, 1, metadata.AttributeEnabledStatusDisabled)
		s.mb.RecordBigipVirtualServerEnabledDataPoint(now, 0, metadata.AttributeEnabledStatusEnabled)
	}

	s.rb.SetBigipVirtualServerName(virtualServerStats.NestedStats.Entries.Name.Description)
	s.rb.SetBigipVirtualServerDestination(virtualServerStats.NestedStats.Entries.Destination.Description)
	s.rb.SetBigipPoolName(virtualServerStats.NestedStats.Entries.PoolName.Description)
	s.mb.EmitForResource(metadata.WithResource(s.rb.Emit()))
}

// collectPools collects pool metrics
func (s *bigipScraper) collectPools(poolStats *models.PoolStats, now pcommon.Timestamp) {
	s.mb.RecordBigipPoolDataTransmittedDataPoint(now, poolStats.NestedStats.Entries.ServersideBitsIn.Value, metadata.AttributeDirectionReceived)
	s.mb.RecordBigipPoolDataTransmittedDataPoint(now, poolStats.NestedStats.Entries.ServersideBitsOut.Value, metadata.AttributeDirectionSent)
	s.mb.RecordBigipPoolConnectionCountDataPoint(now, poolStats.NestedStats.Entries.ServersideCurConns.Value)
	s.mb.RecordBigipPoolPacketCountDataPoint(now, poolStats.NestedStats.Entries.ServersidePktsIn.Value, metadata.AttributeDirectionReceived)
	s.mb.RecordBigipPoolPacketCountDataPoint(now, poolStats.NestedStats.Entries.ServersidePktsOut.Value, metadata.AttributeDirectionSent)
	s.mb.RecordBigipPoolRequestCountDataPoint(now, poolStats.NestedStats.Entries.TotalRequests.Value)
	s.mb.RecordBigipPoolMemberCountDataPoint(now, poolStats.NestedStats.Entries.ActiveMemberCount.Value, metadata.AttributeActiveStatusActive)
	inactiveCount := poolStats.NestedStats.Entries.TotalMemberCount.Value - poolStats.NestedStats.Entries.ActiveMemberCount.Value
	s.mb.RecordBigipPoolMemberCountDataPoint(now, inactiveCount, metadata.AttributeActiveStatusInactive)

	availability := poolStats.NestedStats.Entries.AvailabilityState.Description
	switch {
	case strings.HasPrefix(availability, "available"):
		s.mb.RecordBigipPoolAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusOffline)
		s.mb.RecordBigipPoolAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusUnknown)
		s.mb.RecordBigipPoolAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusAvailable)
	case strings.HasPrefix(availability, "offline"):
		s.mb.RecordBigipPoolAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusOffline)
		s.mb.RecordBigipPoolAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusUnknown)
		s.mb.RecordBigipPoolAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusAvailable)
	default:
		s.mb.RecordBigipPoolAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusOffline)
		s.mb.RecordBigipPoolAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusUnknown)
		s.mb.RecordBigipPoolAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusAvailable)
	}

	enabled := poolStats.NestedStats.Entries.EnabledState.Description
	if strings.HasPrefix(enabled, "enabled") {
		s.mb.RecordBigipPoolEnabledDataPoint(now, 0, metadata.AttributeEnabledStatusDisabled)
		s.mb.RecordBigipPoolEnabledDataPoint(now, 1, metadata.AttributeEnabledStatusEnabled)
	} else {
		s.mb.RecordBigipPoolEnabledDataPoint(now, 1, metadata.AttributeEnabledStatusDisabled)
		s.mb.RecordBigipPoolEnabledDataPoint(now, 0, metadata.AttributeEnabledStatusEnabled)
	}

	s.rb.SetBigipPoolName(poolStats.NestedStats.Entries.Name.Description)
	s.mb.EmitForResource(metadata.WithResource(s.rb.Emit()))
}

// collectPoolMembers collects pool member metrics
func (s *bigipScraper) collectPoolMembers(poolMemberStats *models.PoolMemberStats, now pcommon.Timestamp) {
	s.mb.RecordBigipPoolMemberDataTransmittedDataPoint(now, poolMemberStats.NestedStats.Entries.ServersideBitsIn.Value, metadata.AttributeDirectionReceived)
	s.mb.RecordBigipPoolMemberDataTransmittedDataPoint(now, poolMemberStats.NestedStats.Entries.ServersideBitsOut.Value, metadata.AttributeDirectionSent)
	s.mb.RecordBigipPoolMemberConnectionCountDataPoint(now, poolMemberStats.NestedStats.Entries.ServersideCurConns.Value)
	s.mb.RecordBigipPoolMemberPacketCountDataPoint(now, poolMemberStats.NestedStats.Entries.ServersidePktsIn.Value, metadata.AttributeDirectionReceived)
	s.mb.RecordBigipPoolMemberPacketCountDataPoint(now, poolMemberStats.NestedStats.Entries.ServersidePktsOut.Value, metadata.AttributeDirectionSent)
	s.mb.RecordBigipPoolMemberRequestCountDataPoint(now, poolMemberStats.NestedStats.Entries.TotalRequests.Value)
	s.mb.RecordBigipPoolMemberSessionCountDataPoint(now, poolMemberStats.NestedStats.Entries.CurSessions.Value)

	availability := poolMemberStats.NestedStats.Entries.AvailabilityState.Description
	switch {
	case strings.HasPrefix(availability, "available"):
		s.mb.RecordBigipPoolMemberAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusOffline)
		s.mb.RecordBigipPoolMemberAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusUnknown)
		s.mb.RecordBigipPoolMemberAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusAvailable)
	case strings.HasPrefix(availability, "offline"):
		s.mb.RecordBigipPoolMemberAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusOffline)
		s.mb.RecordBigipPoolMemberAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusUnknown)
		s.mb.RecordBigipPoolMemberAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusAvailable)
	default:
		s.mb.RecordBigipPoolMemberAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusOffline)
		s.mb.RecordBigipPoolMemberAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusUnknown)
		s.mb.RecordBigipPoolMemberAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusAvailable)
	}

	enabled := poolMemberStats.NestedStats.Entries.EnabledState.Description
	if strings.HasPrefix(enabled, "enabled") {
		s.mb.RecordBigipPoolMemberEnabledDataPoint(now, 0, metadata.AttributeEnabledStatusDisabled)
		s.mb.RecordBigipPoolMemberEnabledDataPoint(now, 1, metadata.AttributeEnabledStatusEnabled)
	} else {
		s.mb.RecordBigipPoolMemberEnabledDataPoint(now, 1, metadata.AttributeEnabledStatusDisabled)
		s.mb.RecordBigipPoolMemberEnabledDataPoint(now, 0, metadata.AttributeEnabledStatusEnabled)
	}

	s.rb.SetBigipPoolMemberName(fmt.Sprintf("%s:%d", poolMemberStats.NestedStats.Entries.Name.Description, poolMemberStats.NestedStats.Entries.Port.Value))
	s.rb.SetBigipPoolMemberIPAddress(poolMemberStats.NestedStats.Entries.IPAddress.Description)
	s.rb.SetBigipPoolName(poolMemberStats.NestedStats.Entries.PoolName.Description)
	s.mb.EmitForResource(metadata.WithResource(s.rb.Emit()))
}

// collectNodes collects node metrics
func (s *bigipScraper) collectNodes(nodeStats *models.NodeStats, now pcommon.Timestamp) {
	s.mb.RecordBigipNodeDataTransmittedDataPoint(now, nodeStats.NestedStats.Entries.ServersideBitsIn.Value, metadata.AttributeDirectionReceived)
	s.mb.RecordBigipNodeDataTransmittedDataPoint(now, nodeStats.NestedStats.Entries.ServersideBitsOut.Value, metadata.AttributeDirectionSent)
	s.mb.RecordBigipNodeConnectionCountDataPoint(now, nodeStats.NestedStats.Entries.ServersideCurConns.Value)
	s.mb.RecordBigipNodePacketCountDataPoint(now, nodeStats.NestedStats.Entries.ServersidePktsIn.Value, metadata.AttributeDirectionReceived)
	s.mb.RecordBigipNodePacketCountDataPoint(now, nodeStats.NestedStats.Entries.ServersidePktsOut.Value, metadata.AttributeDirectionSent)
	s.mb.RecordBigipNodeRequestCountDataPoint(now, nodeStats.NestedStats.Entries.TotalRequests.Value)
	s.mb.RecordBigipNodeSessionCountDataPoint(now, nodeStats.NestedStats.Entries.CurSessions.Value)

	availability := nodeStats.NestedStats.Entries.AvailabilityState.Description
	switch {
	case strings.HasPrefix(availability, "available"):
		s.mb.RecordBigipNodeAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusOffline)
		s.mb.RecordBigipNodeAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusUnknown)
		s.mb.RecordBigipNodeAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusAvailable)
	case strings.HasPrefix(availability, "offline"):
		s.mb.RecordBigipNodeAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusOffline)
		s.mb.RecordBigipNodeAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusUnknown)
		s.mb.RecordBigipNodeAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusAvailable)
	default:
		s.mb.RecordBigipNodeAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusOffline)
		s.mb.RecordBigipNodeAvailabilityDataPoint(now, 1, metadata.AttributeAvailabilityStatusUnknown)
		s.mb.RecordBigipNodeAvailabilityDataPoint(now, 0, metadata.AttributeAvailabilityStatusAvailable)
	}

	enabled := nodeStats.NestedStats.Entries.EnabledState.Description
	if strings.HasPrefix(enabled, "enabled") {
		s.mb.RecordBigipNodeEnabledDataPoint(now, 0, metadata.AttributeEnabledStatusDisabled)
		s.mb.RecordBigipNodeEnabledDataPoint(now, 1, metadata.AttributeEnabledStatusEnabled)
	} else {
		s.mb.RecordBigipNodeEnabledDataPoint(now, 1, metadata.AttributeEnabledStatusDisabled)
		s.mb.RecordBigipNodeEnabledDataPoint(now, 0, metadata.AttributeEnabledStatusEnabled)
	}

	s.rb.SetBigipNodeName(nodeStats.NestedStats.Entries.Name.Description)
	s.rb.SetBigipNodeIPAddress(nodeStats.NestedStats.Entries.IPAddress.Description)
	s.mb.EmitForResource(metadata.WithResource(s.rb.Emit()))
}
