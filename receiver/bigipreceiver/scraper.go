// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bigipreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver"

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver/internal/models"
)

// custom errors
var (
	errClientNotInit    = errors.New("client not initialized")
	errScrapedNoMetrics = errors.New("Failed to scrape any metrics")
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
func newScraper(logger *zap.Logger, cfg *Config, settings component.TelemetrySettings) *bigipScraper {
	return &bigipScraper{
		logger:   logger,
		cfg:      cfg,
		settings: settings,
		mb:       metadata.NewMetricsBuilder(cfg.Metrics),
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

	// scrape metrics for virtual servers
	virtualServers, err := s.client.GetVirtualServers(ctx)
	if err != nil {
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
	if err != nil {
		s.logger.Warn("Failed to scrape pool member metrics", zap.Error(err))
	} else {
		collectedMetrics = true
		for key := range poolMembers.Entries {
			poolMemberStats := poolMembers.Entries[key]
			s.collectPoolMembers(&poolMemberStats, now)
		}
	}

	// scrape metrics for nodes
	nodes, err := s.client.GetNodes(ctx)
	if err != nil {
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

	return s.mb.Emit(), nil
}

// collectVirtualServers collects virtual server metrics
func (s *bigipScraper) collectVirtualServers(virtualServerStats *models.VirtualServerStats, now pcommon.Timestamp) {
	s.mb.RecordBigipVirtualServerDataTransmittedDataPoint(now, virtualServerStats.NestedStats.Entries.ClientsideBitsIn.Value, "received")
	s.mb.RecordBigipVirtualServerDataTransmittedDataPoint(now, virtualServerStats.NestedStats.Entries.ClientsideBitsOut.Value, "sent")
	s.mb.RecordBigipVirtualServerConnectionCountDataPoint(now, virtualServerStats.NestedStats.Entries.ClientsideCurConns.Value)
	s.mb.RecordBigipVirtualServerPacketCountDataPoint(now, virtualServerStats.NestedStats.Entries.ClientsidePktsIn.Value, "received")
	s.mb.RecordBigipVirtualServerPacketCountDataPoint(now, virtualServerStats.NestedStats.Entries.ClientsidePktsOut.Value, "sent")
	s.mb.RecordBigipVirtualServerRequestCountDataPoint(now, virtualServerStats.NestedStats.Entries.TotalRequests.Value)
	availability := virtualServerStats.NestedStats.Entries.AvailabilityState.Description
	if strings.HasPrefix(availability, "available") {
		s.mb.RecordBigipVirtualServerAvailabilityDataPoint(now, 0, "offline")
		s.mb.RecordBigipVirtualServerAvailabilityDataPoint(now, 0, "unknown")
		s.mb.RecordBigipVirtualServerAvailabilityDataPoint(now, 1, "online")
	} else if strings.HasPrefix(availability, "offline") {
		s.mb.RecordBigipVirtualServerAvailabilityDataPoint(now, 1, "offline")
		s.mb.RecordBigipVirtualServerAvailabilityDataPoint(now, 0, "unknown")
		s.mb.RecordBigipVirtualServerAvailabilityDataPoint(now, 0, "online")
	} else {
		s.mb.RecordBigipVirtualServerAvailabilityDataPoint(now, 0, "offline")
		s.mb.RecordBigipVirtualServerAvailabilityDataPoint(now, 1, "unknown")
		s.mb.RecordBigipVirtualServerAvailabilityDataPoint(now, 0, "online")
	}
	enabled := virtualServerStats.NestedStats.Entries.EnabledState.Description
	if strings.HasPrefix(enabled, "enabled") {
		s.mb.RecordBigipVirtualServerEnabledDataPoint(now, 0, "disabled")
		s.mb.RecordBigipVirtualServerEnabledDataPoint(now, 1, "enabled")
	} else {
		s.mb.RecordBigipVirtualServerEnabledDataPoint(now, 1, "disabled")
		s.mb.RecordBigipVirtualServerEnabledDataPoint(now, 0, "enabled")
	}

	s.mb.EmitForResource(
		metadata.WithBigipVirtualServerName(virtualServerStats.NestedStats.Entries.Name.Description),
		metadata.WithBigipVirtualServerDestination(virtualServerStats.NestedStats.Entries.Destination.Description),
		metadata.WithBigipPoolName(virtualServerStats.NestedStats.Entries.PoolName.Description),
	)
}

// collectPools collects virtual server metrics
func (s *bigipScraper) collectPools(poolStats *models.PoolStats, now pcommon.Timestamp) {
	s.mb.RecordBigipPoolDataTransmittedDataPoint(now, poolStats.NestedStats.Entries.ServersideBitsIn.Value, "received")
	s.mb.RecordBigipPoolDataTransmittedDataPoint(now, poolStats.NestedStats.Entries.ServersideBitsOut.Value, "sent")
	s.mb.RecordBigipPoolConnectionCountDataPoint(now, poolStats.NestedStats.Entries.ServersideCurConns.Value)
	s.mb.RecordBigipPoolPacketCountDataPoint(now, poolStats.NestedStats.Entries.ServersidePktsIn.Value, "received")
	s.mb.RecordBigipPoolPacketCountDataPoint(now, poolStats.NestedStats.Entries.ServersidePktsOut.Value, "sent")
	s.mb.RecordBigipPoolRequestCountDataPoint(now, poolStats.NestedStats.Entries.TotalRequests.Value)
	s.mb.RecordBigipPoolMemberCountDataPoint(now, poolStats.NestedStats.Entries.TotalMemberCount.Value)
	s.mb.RecordBigipPoolActiveMemberCountDataPoint(now, poolStats.NestedStats.Entries.ActiveMemberCount.Value)
	s.mb.RecordBigipPoolAvailableMemberCountDataPoint(now, poolStats.NestedStats.Entries.AvailableMemberCount.Value)
	availability := poolStats.NestedStats.Entries.AvailabilityState.Description
	if strings.HasPrefix(availability, "available") {
		s.mb.RecordBigipPoolAvailabilityDataPoint(now, 0, "offline")
		s.mb.RecordBigipPoolAvailabilityDataPoint(now, 0, "unknown")
		s.mb.RecordBigipPoolAvailabilityDataPoint(now, 1, "online")
	} else if strings.HasPrefix(availability, "offline") {
		s.mb.RecordBigipPoolAvailabilityDataPoint(now, 1, "offline")
		s.mb.RecordBigipPoolAvailabilityDataPoint(now, 0, "unknown")
		s.mb.RecordBigipPoolAvailabilityDataPoint(now, 0, "online")
	} else {
		s.mb.RecordBigipPoolAvailabilityDataPoint(now, 0, "offline")
		s.mb.RecordBigipPoolAvailabilityDataPoint(now, 1, "unknown")
		s.mb.RecordBigipPoolAvailabilityDataPoint(now, 0, "online")
	}
	enabled := poolStats.NestedStats.Entries.EnabledState.Description
	if strings.HasPrefix(enabled, "enabled") {
		s.mb.RecordBigipPoolEnabledDataPoint(now, 0, "disabled")
		s.mb.RecordBigipPoolEnabledDataPoint(now, 1, "enabled")
	} else {
		s.mb.RecordBigipPoolEnabledDataPoint(now, 1, "disabled")
		s.mb.RecordBigipPoolEnabledDataPoint(now, 0, "enabled")
	}

	s.mb.EmitForResource(
		metadata.WithBigipPoolName(poolStats.NestedStats.Entries.Name.Description),
	)
}

// collectPoolMembers collects virtual server metrics
func (s *bigipScraper) collectPoolMembers(poolMemberStats *models.PoolMemberStats, now pcommon.Timestamp) {
	s.mb.RecordBigipPoolMemberDataTransmittedDataPoint(now, poolMemberStats.NestedStats.Entries.ServersideBitsIn.Value, "received")
	s.mb.RecordBigipPoolMemberDataTransmittedDataPoint(now, poolMemberStats.NestedStats.Entries.ServersideBitsOut.Value, "sent")
	s.mb.RecordBigipPoolMemberConnectionCountDataPoint(now, poolMemberStats.NestedStats.Entries.ServersideCurConns.Value)
	s.mb.RecordBigipPoolMemberPacketCountDataPoint(now, poolMemberStats.NestedStats.Entries.ServersidePktsIn.Value, "received")
	s.mb.RecordBigipPoolMemberPacketCountDataPoint(now, poolMemberStats.NestedStats.Entries.ServersidePktsOut.Value, "sent")
	s.mb.RecordBigipPoolMemberRequestCountDataPoint(now, poolMemberStats.NestedStats.Entries.TotalRequests.Value)
	s.mb.RecordBigipPoolMemberSessionCountDataPoint(now, poolMemberStats.NestedStats.Entries.CurSessions.Value)
	availability := poolMemberStats.NestedStats.Entries.AvailabilityState.Description
	if strings.HasPrefix(availability, "available") {
		s.mb.RecordBigipPoolMemberAvailabilityDataPoint(now, 0, "offline")
		s.mb.RecordBigipPoolMemberAvailabilityDataPoint(now, 0, "unknown")
		s.mb.RecordBigipPoolMemberAvailabilityDataPoint(now, 1, "online")
	} else if strings.HasPrefix(availability, "offline") {
		s.mb.RecordBigipPoolMemberAvailabilityDataPoint(now, 1, "offline")
		s.mb.RecordBigipPoolMemberAvailabilityDataPoint(now, 0, "unknown")
		s.mb.RecordBigipPoolMemberAvailabilityDataPoint(now, 0, "online")
	} else {
		s.mb.RecordBigipPoolMemberAvailabilityDataPoint(now, 0, "offline")
		s.mb.RecordBigipPoolMemberAvailabilityDataPoint(now, 1, "unknown")
		s.mb.RecordBigipPoolMemberAvailabilityDataPoint(now, 0, "online")
	}
	enabled := poolMemberStats.NestedStats.Entries.EnabledState.Description
	if strings.HasPrefix(enabled, "enabled") {
		s.mb.RecordBigipPoolMemberEnabledDataPoint(now, 0, "disabled")
		s.mb.RecordBigipPoolMemberEnabledDataPoint(now, 1, "enabled")
	} else {
		s.mb.RecordBigipPoolMemberEnabledDataPoint(now, 1, "disabled")
		s.mb.RecordBigipPoolMemberEnabledDataPoint(now, 0, "enabled")
	}

	s.mb.EmitForResource(
		metadata.WithBigipPoolMemberName(
			poolMemberStats.NestedStats.Entries.Name.Description+":"+
				strconv.FormatInt(poolMemberStats.NestedStats.Entries.Port.Value, 10)),
		metadata.WithBigipPoolMemberIPAddress(poolMemberStats.NestedStats.Entries.IPAddress.Description),
		metadata.WithBigipPoolName(poolMemberStats.NestedStats.Entries.PoolName.Description),
	)
}

// collectNodes collects virtual server metrics
func (s *bigipScraper) collectNodes(nodeStats *models.NodeStats, now pcommon.Timestamp) {
	s.mb.RecordBigipNodeDataTransmittedDataPoint(now, nodeStats.NestedStats.Entries.ServersideBitsIn.Value, "received")
	s.mb.RecordBigipNodeDataTransmittedDataPoint(now, nodeStats.NestedStats.Entries.ServersideBitsOut.Value, "sent")
	s.mb.RecordBigipNodeConnectionCountDataPoint(now, nodeStats.NestedStats.Entries.ServersideCurConns.Value)
	s.mb.RecordBigipNodePacketCountDataPoint(now, nodeStats.NestedStats.Entries.ServersidePktsIn.Value, "received")
	s.mb.RecordBigipNodePacketCountDataPoint(now, nodeStats.NestedStats.Entries.ServersidePktsOut.Value, "sent")
	s.mb.RecordBigipNodeRequestCountDataPoint(now, nodeStats.NestedStats.Entries.TotalRequests.Value)
	s.mb.RecordBigipNodeSessionCountDataPoint(now, nodeStats.NestedStats.Entries.CurSessions.Value)
	availability := nodeStats.NestedStats.Entries.AvailabilityState.Description
	if strings.HasPrefix(availability, "available") {
		s.mb.RecordBigipNodeAvailabilityDataPoint(now, 0, "offline")
		s.mb.RecordBigipNodeAvailabilityDataPoint(now, 0, "unknown")
		s.mb.RecordBigipNodeAvailabilityDataPoint(now, 1, "online")
	} else if strings.HasPrefix(availability, "offline") {
		s.mb.RecordBigipNodeAvailabilityDataPoint(now, 1, "offline")
		s.mb.RecordBigipNodeAvailabilityDataPoint(now, 0, "unknown")
		s.mb.RecordBigipNodeAvailabilityDataPoint(now, 0, "online")
	} else {
		s.mb.RecordBigipNodeAvailabilityDataPoint(now, 0, "offline")
		s.mb.RecordBigipNodeAvailabilityDataPoint(now, 1, "unknown")
		s.mb.RecordBigipNodeAvailabilityDataPoint(now, 0, "online")
	}
	enabled := nodeStats.NestedStats.Entries.EnabledState.Description
	if strings.HasPrefix(enabled, "enabled") {
		s.mb.RecordBigipNodeEnabledDataPoint(now, 0, "disabled")
		s.mb.RecordBigipNodeEnabledDataPoint(now, 1, "enabled")
	} else {
		s.mb.RecordBigipNodeEnabledDataPoint(now, 1, "disabled")
		s.mb.RecordBigipNodeEnabledDataPoint(now, 0, "enabled")
	}

	s.mb.EmitForResource(
		metadata.WithBigipNodeName(nodeStats.NestedStats.Entries.Name.Description),
		metadata.WithBigipNodeIPAddress(nodeStats.NestedStats.Entries.IPAddress.Description),
	)
}
