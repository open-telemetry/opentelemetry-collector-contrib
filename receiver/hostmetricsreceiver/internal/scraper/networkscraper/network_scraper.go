// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package networkscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper"

import (
	"context"
	"fmt"
	stdnet "net"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper/internal/metadata"
)

const (
	networkMetricsLen = 4
)

// scraper for Network Metrics
type networkScraper struct {
	settings         scraper.Settings
	config           *Config
	mb               *metadata.MetricsBuilder
	startTime        pcommon.Timestamp
	includeFS        filterset.FilterSet
	excludeFS        filterset.FilterSet
	includeProcessFS filterset.FilterSet
	excludeProcessFS filterset.FilterSet

	// for mocking
	bootTime       func(context.Context) (uint64, error)
	ioCounters     func(context.Context, bool) ([]net.IOCountersStat, error)
	connections    func(context.Context, string) ([]net.ConnectionStat, error)
	conntrack      func(context.Context) ([]net.FilterStat, error)
	processName    func(context.Context, int32) (string, error)
	interfaceAddrs func() ([]stdnet.Addr, error)
}

// newNetworkScraper creates a set of Network related metrics
func newNetworkScraper(_ context.Context, settings scraper.Settings, cfg *Config) (*networkScraper, error) {
	scraper := &networkScraper{
		settings:       settings,
		config:         cfg,
		bootTime:       host.BootTimeWithContext,
		ioCounters:     net.IOCountersWithContext,
		connections:    net.ConnectionsWithContext,
		conntrack:      net.FilterCountersWithContext,
		processName:    getProcessName,
		interfaceAddrs: stdnet.InterfaceAddrs,
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

	if len(cfg.Connections.IncludeProcesses.Names) > 0 {
		scraper.includeProcessFS, err = filterset.CreateFilterSet(
			cfg.Connections.IncludeProcesses.Names,
			&cfg.Connections.IncludeProcesses.Config,
		)
		if err != nil {
			return nil, fmt.Errorf("error creating network connection process include filters: %w", err)
		}
	}

	if len(cfg.Connections.ExcludeProcesses.Names) > 0 {
		scraper.excludeProcessFS, err = filterset.CreateFilterSet(
			cfg.Connections.ExcludeProcesses.Names,
			&cfg.Connections.ExcludeProcesses.Config,
		)
		if err != nil {
			return nil, fmt.Errorf("error creating network connection process exclude filters: %w", err)
		}
	}

	return scraper, nil
}

func getProcessName(ctx context.Context, pid int32) (string, error) {
	if pid <= 0 {
		return "", nil
	}

	proc, err := process.NewProcessWithContext(ctx, pid)
	if err != nil {
		return "", err
	}
	return proc.NameWithContext(ctx)
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
		errors.AddPartial(s.enabledConnectionMetricsLen(), err)
	}

	err = s.recordNetworkConntrackMetrics(ctx)
	if err != nil {
		errors.AddPartial(conntrackMetricsLen, err)
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
	if s.enabledConnectionMetricsLen() == 0 {
		return nil
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	connections, err := s.connections(ctx, "tcp")
	if err != nil {
		return fmt.Errorf("failed to read TCP connections: %w", err)
	}

	if s.config.Metrics.SystemNetworkConnections.Enabled {
		tcpConnectionStatusCounts := getTCPConnectionStatusCounts(connections)
		s.recordNetworkConnectionsMetric(now, tcpConnectionStatusCounts)
	}

	if s.config.Metrics.SystemNetworkConnectionCount.Enabled {
		s.recordNetworkConnectionCountMetric(ctx, now, connections)
	}

	return nil
}

func (s *networkScraper) enabledConnectionMetricsLen() int {
	count := 0
	if s.config.Metrics.SystemNetworkConnections.Enabled {
		count++
	}
	if s.config.Metrics.SystemNetworkConnectionCount.Enabled {
		count++
	}
	return count
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

type networkConnectionKey struct {
	processName   string
	serverAddress string
	serverPort    int64
}

func (s *networkScraper) recordNetworkConnectionCountMetric(ctx context.Context, now pcommon.Timestamp, connections []net.ConnectionStat) {
	listenPorts := map[uint32]struct{}{}
	if s.config.Connections.ExcludeListenPorts {
		listenPorts = buildListenPortSet(connections)
	}

	var localIPs map[string]struct{}
	if s.config.Connections.ExcludeLocalhost {
		localIPs = s.localIPSet()
	}

	pidCache := make(map[int32]string, len(connections))
	counts := make(map[networkConnectionKey]int64)
	for _, connection := range connections {
		if !strings.EqualFold(connection.Status, "ESTABLISHED") {
			continue
		}
		if connection.Raddr.IP == "" || connection.Raddr.Port == 0 {
			continue
		}
		if _, ok := listenPorts[connection.Laddr.Port]; ok {
			continue
		}
		if !s.isConnectionPortAllowed(connection.Raddr.Port) {
			continue
		}
		if s.config.Connections.ExcludeLocalhost && isLocalIP(connection.Raddr.IP, localIPs) {
			continue
		}

		processName := s.getProcessNameCached(ctx, connection.Pid, pidCache)
		if processName == "" || !s.isConnectionProcessAllowed(processName) {
			continue
		}

		key := networkConnectionKey{
			processName:   processName,
			serverAddress: connection.Raddr.IP,
			serverPort:    int64(connection.Raddr.Port),
		}
		counts[key]++
	}

	for key, count := range counts {
		s.mb.RecordSystemNetworkConnectionCountDataPoint(
			now,
			count,
			key.processName,
			key.serverAddress,
			key.serverPort,
		)
	}
}

func buildListenPortSet(connections []net.ConnectionStat) map[uint32]struct{} {
	ports := make(map[uint32]struct{})
	for _, connection := range connections {
		if strings.EqualFold(connection.Status, "LISTEN") {
			ports[connection.Laddr.Port] = struct{}{}
		}
	}
	return ports
}

func (s *networkScraper) getProcessNameCached(ctx context.Context, pid int32, cache map[int32]string) string {
	if name, ok := cache[pid]; ok {
		return name
	}

	name, err := s.processName(ctx, pid)
	if err != nil {
		cache[pid] = ""
		return ""
	}
	cache[pid] = name
	return name
}

func (s *networkScraper) isConnectionProcessAllowed(processName string) bool {
	return (s.includeProcessFS == nil || s.includeProcessFS.Matches(processName)) &&
		(s.excludeProcessFS == nil || !s.excludeProcessFS.Matches(processName))
}

func (s *networkScraper) isConnectionPortAllowed(port uint32) bool {
	if len(s.config.Connections.IncludePorts) > 0 {
		matched := false
		for _, includePort := range s.config.Connections.IncludePorts {
			if includePort == port {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	for _, excludePort := range s.config.Connections.ExcludePorts {
		if excludePort == port {
			return false
		}
	}
	return true
}

func (s *networkScraper) localIPSet() map[string]struct{} {
	ips := map[string]struct{}{
		"127.0.0.1": {},
		"::1":       {},
		"localhost": {},
	}

	addrs, err := s.interfaceAddrs()
	if err != nil {
		return ips
	}
	for _, addr := range addrs {
		switch typedAddr := addr.(type) {
		case *stdnet.IPNet:
			ips[typedAddr.IP.String()] = struct{}{}
		case *stdnet.IPAddr:
			ips[typedAddr.IP.String()] = struct{}{}
		}
	}
	return ips
}

func isLocalIP(ip string, localIPs map[string]struct{}) bool {
	if _, ok := localIPs[ip]; ok {
		return true
	}
	parsed := stdnet.ParseIP(ip)
	return parsed != nil && parsed.IsLoopback()
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
