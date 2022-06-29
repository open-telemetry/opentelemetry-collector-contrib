// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package networkscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper"

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/net"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.opentelemetry.io/collector/service/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper/internal/metadata"
)

const (
	networkMetricsLen     = 4
	connectionsMetricsLen = 1
	// ID for a temporary feature gate"
	removeDirectionAttributeFeatureGateID = "receiver.hostmetricsreceiver.removeDirectionAttributeNetworkMetrics"
)

// scraper for Network Metrics
type scraper struct {
	settings  component.ReceiverCreateSettings
	config    *Config
	mb        *metadata.MetricsBuilder
	startTime pcommon.Timestamp
	includeFS filterset.FilterSet
	excludeFS filterset.FilterSet

	// for mocking
	bootTime    func() (uint64, error)
	ioCounters  func(bool) ([]net.IOCountersStat, error)
	connections func(string) ([]net.ConnectionStat, error)
}

var removeDirectionAttributeFeatureGate = featuregate.Gate{
	ID:      removeDirectionAttributeFeatureGateID,
	Enabled: false,
	Description: "Some network metrics reported by the hostmetricsreceiver are transitioning from being reported " +
		"with a direction attribute to being reported with the direction included in the metric name to adhere to the " +
		"OpenTelemetry specification. You can control whether the hostmetricsreceiver reports metrics with a direction " +
		"attribute using the " + removeDirectionAttributeFeatureGateID + " feature gate. For more details, see: " +
		"https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/hostmetricsreceiver/README.md#feature-gate-configurations",
}

func init() {
	featuregate.GetRegistry().MustRegister(removeDirectionAttributeFeatureGate)
}

// newNetworkScraper creates a set of Network related metrics
func newNetworkScraper(_ context.Context, settings component.ReceiverCreateSettings, cfg *Config) (*scraper, error) {
	scraper := &scraper{settings: settings, config: cfg, bootTime: host.BootTime, ioCounters: net.IOCounters, connections: net.Connections}

	var err error

	if featuregate.GetRegistry().IsEnabled(removeDirectionAttributeFeatureGateID) {
		settings.Logger.Info("The " + removeDirectionAttributeFeatureGateID + " featre gate is enabled. This " +
			"otel collector will report metrics without a direction attribute, which is good for future support")
	} else {
		settings.Logger.Info("WARNING - Breaking Change: " + removeDirectionAttributeFeatureGate.Description)
		settings.Logger.Info("The feature gate " + removeDirectionAttributeFeatureGateID + " is disabled. This " +
			"otel collector will report metrics with a direction attribute, be aware this will not be supported in the future")
	}

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

func (s *scraper) start(context.Context, component.Host) error {
	bootTime, err := s.bootTime()
	if err != nil {
		return err
	}

	s.startTime = pcommon.Timestamp(bootTime * 1e9)
	s.mb = metadata.NewMetricsBuilder(s.config.Metrics, s.settings.BuildInfo, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
	return nil
}

func (s *scraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	var errors scrapererror.ScrapeErrors

	err := s.recordNetworkCounterMetrics()
	if err != nil {
		errors.AddPartial(networkMetricsLen, err)
	}

	err = s.recordNetworkConnectionsMetrics()
	if err != nil {
		errors.AddPartial(connectionsMetricsLen, err)
	}

	return s.mb.Emit(), errors.Combine()
}

func (s *scraper) recordNetworkCounterMetrics() error {
	now := pcommon.NewTimestampFromTime(time.Now())

	// get total stats only
	ioCounters, err := s.ioCounters( /*perNetworkInterfaceController=*/ true)
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

func (s *scraper) recordNetworkPacketsMetric(now pcommon.Timestamp, ioCountersSlice []net.IOCountersStat) {
	for _, ioCounters := range ioCountersSlice {
		if featuregate.GetRegistry().IsEnabled(removeDirectionAttributeFeatureGateID) {
			s.mb.RecordSystemNetworkPacketsTransmitDataPoint(now, int64(ioCounters.PacketsSent), ioCounters.Name)
			s.mb.RecordSystemNetworkPacketsReceiveDataPoint(now, int64(ioCounters.PacketsRecv), ioCounters.Name)
		} else {
			s.mb.RecordSystemNetworkPacketsDataPoint(now, int64(ioCounters.PacketsSent), ioCounters.Name, metadata.AttributeDirectionTransmit)
			s.mb.RecordSystemNetworkPacketsDataPoint(now, int64(ioCounters.PacketsRecv), ioCounters.Name, metadata.AttributeDirectionReceive)
		}
	}
}

func (s *scraper) recordNetworkDroppedPacketsMetric(now pcommon.Timestamp, ioCountersSlice []net.IOCountersStat) {
	for _, ioCounters := range ioCountersSlice {
		if featuregate.GetRegistry().IsEnabled(removeDirectionAttributeFeatureGateID) {
			s.mb.RecordSystemNetworkDroppedTransmitDataPoint(now, int64(ioCounters.Dropout), ioCounters.Name)
			s.mb.RecordSystemNetworkDroppedReceiveDataPoint(now, int64(ioCounters.Dropin), ioCounters.Name)
		} else {
			s.mb.RecordSystemNetworkDroppedDataPoint(now, int64(ioCounters.Dropout), ioCounters.Name, metadata.AttributeDirectionTransmit)
			s.mb.RecordSystemNetworkDroppedDataPoint(now, int64(ioCounters.Dropin), ioCounters.Name, metadata.AttributeDirectionReceive)
		}
	}
}

func (s *scraper) recordNetworkErrorPacketsMetric(now pcommon.Timestamp, ioCountersSlice []net.IOCountersStat) {
	for _, ioCounters := range ioCountersSlice {
		if featuregate.GetRegistry().IsEnabled(removeDirectionAttributeFeatureGateID) {
			s.mb.RecordSystemNetworkErrorsTransmitDataPoint(now, int64(ioCounters.Errout), ioCounters.Name)
			s.mb.RecordSystemNetworkErrorsReceiveDataPoint(now, int64(ioCounters.Errin), ioCounters.Name)
		} else {
			s.mb.RecordSystemNetworkErrorsDataPoint(now, int64(ioCounters.Errout), ioCounters.Name, metadata.AttributeDirectionTransmit)
			s.mb.RecordSystemNetworkErrorsDataPoint(now, int64(ioCounters.Errin), ioCounters.Name, metadata.AttributeDirectionReceive)
		}
	}
}

func (s *scraper) recordNetworkIOMetric(now pcommon.Timestamp, ioCountersSlice []net.IOCountersStat) {
	for _, ioCounters := range ioCountersSlice {
		if featuregate.GetRegistry().IsEnabled(removeDirectionAttributeFeatureGateID) {
			s.mb.RecordSystemNetworkIoTransmitDataPoint(now, int64(ioCounters.BytesSent), ioCounters.Name)
			s.mb.RecordSystemNetworkIoReceiveDataPoint(now, int64(ioCounters.BytesRecv), ioCounters.Name)
		} else {
			s.mb.RecordSystemNetworkIoDataPoint(now, int64(ioCounters.BytesSent), ioCounters.Name, metadata.AttributeDirectionTransmit)
			s.mb.RecordSystemNetworkIoDataPoint(now, int64(ioCounters.BytesRecv), ioCounters.Name, metadata.AttributeDirectionReceive)
		}
	}
}

func (s *scraper) recordNetworkConnectionsMetrics() error {
	now := pcommon.NewTimestampFromTime(time.Now())

	connections, err := s.connections("tcp")
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

func (s *scraper) recordNetworkConnectionsMetric(now pcommon.Timestamp, connectionStateCounts map[string]int64) {
	for connectionState, count := range connectionStateCounts {
		s.mb.RecordSystemNetworkConnectionsDataPoint(now, count, metadata.AttributeProtocolTcp, connectionState)
	}
}

func (s *scraper) filterByInterface(ioCounters []net.IOCountersStat) []net.IOCountersStat {
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

func (s *scraper) includeInterface(interfaceName string) bool {
	return (s.includeFS == nil || s.includeFS.Matches(interfaceName)) &&
		(s.excludeFS == nil || !s.excludeFS.Matches(interfaceName))
}
