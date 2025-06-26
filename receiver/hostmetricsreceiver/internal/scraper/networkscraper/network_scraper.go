// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package networkscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper"

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/net"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper/internal/metadata"
)

const (
	networkMetricsLen     = 4
	connectionsMetricsLen = 1
)

// scraper for Network Metrics
type networkScraper struct {
	settings  scraper.Settings
	config    *Config
	mb        *metadata.MetricsBuilder
	startTime pcommon.Timestamp
	includeFS filterset.FilterSet
	excludeFS filterset.FilterSet

	// for mocking
	bootTime    func(context.Context) (uint64, error)
	ioCounters  func(context.Context, bool) ([]net.IOCountersStat, error)
	connections func(context.Context, string) ([]net.ConnectionStat, error)
	conntrack   func(context.Context) ([]net.FilterStat, error)
}

// newNetworkScraper creates a set of Network related metrics
func newNetworkScraper(_ context.Context, settings scraper.Settings, cfg *Config) (*networkScraper, error) {
	scraper := &networkScraper{
		settings:    settings,
		config:      cfg,
		bootTime:    host.BootTimeWithContext,
		ioCounters:  net.IOCountersWithContext,
		connections: net.ConnectionsWithContext,
		conntrack:   net.FilterCountersWithContext,
	}

	var err error

	if len(cfg.Include.Interfaces) > 0 {
		scraper.includeFS, err = filterset.CreateFilterSet(cfg.Include.Interfaces, &cfg.Include.Config)
		if err != nil {
			return nil, fmt.Errorf("error creating network interface include filters: %w", err)
		}
	}

	if len(cfg.Exclude.Interfaces) > 0 {
		scraper.excludeFS, err = filterset.CreateFilterSet(cfg.Exclude.Interfaces, &cfg.Exclude.Config)
		if err != nil {
			return nil, fmt.Errorf("error creating network interface exclude filters: %w", err)
		}
	}

	return scraper, nil
}

func (s *networkScraper) start(ctx context.Context, _ component.Host) error {
	bootTime, err := s.bootTime(ctx)
	if err != nil {
		return err
	}

	s.startTime = pcommon.Timestamp(bootTime * 1e9)
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
	return nil
}

func (s *networkScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	var errors scrapererror.ScrapeErrors

	err := s.recordNetworkCounterMetrics(ctx)
	if err != nil {
		errors.AddPartial(networkMetricsLen, err)
	}

	err = s.recordNetworkConnectionsMetrics(ctx)
	if err != nil {
		errors.AddPartial(connectionsMetricsLen, err)
	}

	err = s.recordNetworkConntrackMetrics(ctx)
	if err != nil {
		errors.AddPartial(connectionsMetricsLen, err)
	}

	return s.mb.Emit(), errors.Combine()
}

func (s *networkScraper) recordNetworkCounterMetrics(ctx context.Context) error {
	now := pcommon.NewTimestampFromTime(time.Now())

	// get total stats only
	ioCounters, err := s.ioCounters(ctx, true /*perNetworkInterfaceController=*/)
	if err != nil {
		return fmt.Errorf("failed to read network IO stats: %w", err)
	}

	// filter network interfaces by name
	ioCounters = s.filterByInterface(ioCounters)

	if len(ioCounters) > 0 {
		s.recordNetworkPacketsMetric(now, ioCounters)
		s.recordNetworkDroppedPacketsMetric(now, ioCounters)
		s.recordNetworkErrorPacketsMetric(now, ioCounters)
		s.recordNetworkIOMetric(now, ioCounters)
	}

	return nil
}

func (s *networkScraper) recordNetworkPacketsMetric(now pcommon.Timestamp, ioCountersSlice []net.IOCountersStat) {
	for _, ioCounters := range ioCountersSlice {
		s.mb.RecordSystemNetworkPacketsDataPoint(now, int64(ioCounters.PacketsSent), ioCounters.Name, metadata.AttributeDirectionTransmit)
		s.mb.RecordSystemNetworkPacketsDataPoint(now, int64(ioCounters.PacketsRecv), ioCounters.Name, metadata.AttributeDirectionReceive)
	}
}

func (s *networkScraper) recordNetworkDroppedPacketsMetric(now pcommon.Timestamp, ioCountersSlice []net.IOCountersStat) {
	for _, ioCounters := range ioCountersSlice {
		s.mb.RecordSystemNetworkDroppedDataPoint(now, int64(ioCounters.Dropout), ioCounters.Name, metadata.AttributeDirectionTransmit)
		s.mb.RecordSystemNetworkDroppedDataPoint(now, int64(ioCounters.Dropin), ioCounters.Name, metadata.AttributeDirectionReceive)
	}
}

func (s *networkScraper) recordNetworkErrorPacketsMetric(now pcommon.Timestamp, ioCountersSlice []net.IOCountersStat) {
	for _, ioCounters := range ioCountersSlice {
		s.mb.RecordSystemNetworkErrorsDataPoint(now, int64(ioCounters.Errout), ioCounters.Name, metadata.AttributeDirectionTransmit)
		s.mb.RecordSystemNetworkErrorsDataPoint(now, int64(ioCounters.Errin), ioCounters.Name, metadata.AttributeDirectionReceive)
	}
}

func (s *networkScraper) recordNetworkIOMetric(now pcommon.Timestamp, ioCountersSlice []net.IOCountersStat) {
	for _, ioCounters := range ioCountersSlice {
		s.mb.RecordSystemNetworkIoDataPoint(now, int64(ioCounters.BytesSent), ioCounters.Name, metadata.AttributeDirectionTransmit)
		s.mb.RecordSystemNetworkIoDataPoint(now, int64(ioCounters.BytesRecv), ioCounters.Name, metadata.AttributeDirectionReceive)
	}
}

func (s *networkScraper) recordNetworkConnectionsMetrics(ctx context.Context) error {
	if !s.config.Metrics.SystemNetworkConnections.Enabled {
		return nil
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	connections, err := s.connections(ctx, "tcp")
	if err != nil {
		return fmt.Errorf("failed to read TCP connections: %w", err)
	}

	tcpConnectionStatusCounts := getTCPConnectionStatusCounts(connections)

	s.recordNetworkConnectionsMetric(now, tcpConnectionStatusCounts)
	return nil
}

func getTCPConnectionStatusCounts(connections []net.ConnectionStat) map[string]int64 {
	tcpStatuses := make(map[string]int64, len(allTCPStates))
	for _, state := range allTCPStates {
		tcpStatuses[state] = 0
	}

	for _, connection := range connections {
		tcpStatuses[connection.Status]++
	}
	return tcpStatuses
}

func (s *networkScraper) recordNetworkConnectionsMetric(now pcommon.Timestamp, connectionStateCounts map[string]int64) {
	for connectionState, count := range connectionStateCounts {
		s.mb.RecordSystemNetworkConnectionsDataPoint(now, count, metadata.AttributeProtocolTcp, connectionState)
	}
}

func (s *networkScraper) filterByInterface(ioCounters []net.IOCountersStat) []net.IOCountersStat {
	if s.includeFS == nil && s.excludeFS == nil {
		return ioCounters
	}

	filteredIOCounters := make([]net.IOCountersStat, 0, len(ioCounters))
	for _, io := range ioCounters {
		if s.includeInterface(io.Name) {
			filteredIOCounters = append(filteredIOCounters, io)
		}
	}
	return filteredIOCounters
}

func (s *networkScraper) includeInterface(interfaceName string) bool {
	return (s.includeFS == nil || s.includeFS.Matches(interfaceName)) &&
		(s.excludeFS == nil || !s.excludeFS.Matches(interfaceName))
}
