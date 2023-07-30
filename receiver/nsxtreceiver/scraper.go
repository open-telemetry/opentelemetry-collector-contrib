// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nsxtreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxtreceiver"

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxtreceiver/internal/metadata"
	dm "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxtreceiver/internal/model"
)

type scraper struct {
	config   *Config
	settings component.TelemetrySettings
	host     component.Host
	client   Client
	rb       *metadata.ResourceBuilder
	mb       *metadata.MetricsBuilder
}

func newScraper(cfg *Config, settings receiver.CreateSettings) *scraper {
	return &scraper{
		config:   cfg,
		settings: settings.TelemetrySettings,
		rb:       metadata.NewResourceBuilder(cfg.ResourceAttributes),
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
	}
}

func (s *scraper) start(_ context.Context, host component.Host) error {
	s.host = host
	client, err := newClient(s.config, s.settings, s.host, s.settings.Logger)
	if err != nil {
		return fmt.Errorf("unable to construct http client: %w", err)
	}
	s.client = client
	return nil
}

type nodeClass int

const (
	transportClass nodeClass = iota
	managerClass
)

func (s *scraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	r, err := s.retrieve(ctx)
	if err != nil {
		return pmetric.NewMetrics(), err
	}

	colTime := pcommon.NewTimestampFromTime(time.Now())
	s.process(r, colTime)
	return s.mb.Emit(), nil
}

type nodeInfo struct {
	nodeProps  dm.NodeProperties
	nodeType   string
	interfaces []interfaceInformation
	stats      *dm.NodeStatus
}

type interfaceInformation struct {
	iFace dm.NetworkInterface
	stats *dm.NetworkInterfaceStats
}

func (s *scraper) retrieve(ctx context.Context) ([]*nodeInfo, error) {
	var r []*nodeInfo
	errs := &scrapererror.ScrapeErrors{}

	tNodes, err := s.client.TransportNodes(ctx)
	if err != nil {
		return r, err
	}

	cNodes, err := s.client.ClusterNodes(ctx)
	if err != nil {
		return r, err
	}

	wg := &sync.WaitGroup{}
	for _, n := range tNodes {
		nodeInfo := &nodeInfo{
			nodeProps: n.NodeProperties,
			nodeType:  "transport",
		}
		wg.Add(2)
		go s.retrieveInterfaces(ctx, n.NodeProperties, nodeInfo, transportClass, wg, errs)
		go s.retrieveNodeStats(ctx, n.NodeProperties, nodeInfo, transportClass, wg, errs)

		r = append(r, nodeInfo)
	}

	for _, n := range cNodes {
		// no useful stats are recorded for controller nodes
		if clusterNodeType(n) != "manager" {
			continue
		}

		nodeInfo := &nodeInfo{
			nodeProps: n.NodeProperties,
			nodeType:  "manager",
		}

		wg.Add(2)
		go s.retrieveInterfaces(ctx, n.NodeProperties, nodeInfo, managerClass, wg, errs)
		go s.retrieveNodeStats(ctx, n.NodeProperties, nodeInfo, managerClass, wg, errs)

		r = append(r, nodeInfo)
	}

	wg.Wait()

	return r, errs.Combine()
}

func (s *scraper) retrieveInterfaces(
	ctx context.Context,
	nodeProps dm.NodeProperties,
	nodeInfo *nodeInfo,
	nodeClass nodeClass,
	wg *sync.WaitGroup,
	errs *scrapererror.ScrapeErrors,
) {
	defer wg.Done()
	interfaces, err := s.client.Interfaces(ctx, nodeProps.ID, nodeClass)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	nodeInfo.interfaces = []interfaceInformation{}
	for _, i := range interfaces {
		interfaceInfo := interfaceInformation{
			iFace: i,
		}
		stats, err := s.client.InterfaceStatus(ctx, nodeProps.ID, i.InterfaceId, nodeClass)
		if err != nil {
			errs.AddPartial(1, err)
		}
		interfaceInfo.stats = stats
		nodeInfo.interfaces = append(nodeInfo.interfaces, interfaceInfo)
	}
}

func (s *scraper) retrieveNodeStats(
	ctx context.Context,
	nodeProps dm.NodeProperties,
	nodeInfo *nodeInfo,
	nodeClass nodeClass,
	wg *sync.WaitGroup,
	errs *scrapererror.ScrapeErrors,
) {
	defer wg.Done()
	ns, err := s.client.NodeStatus(ctx, nodeProps.ID, nodeClass)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	nodeInfo.stats = ns
}

func (s *scraper) process(
	nodes []*nodeInfo,
	colTime pcommon.Timestamp,
) {
	for _, n := range nodes {
		for _, i := range n.interfaces {
			s.recordNodeInterface(colTime, n.nodeProps, i)
		}
		s.recordNode(colTime, n)
	}
}

func (s *scraper) recordNodeInterface(colTime pcommon.Timestamp, nodeProps dm.NodeProperties, i interfaceInformation) {
	s.mb.RecordNsxtNodeNetworkPacketCountDataPoint(colTime, i.stats.RxDropped, metadata.AttributeDirectionReceived, metadata.AttributePacketTypeDropped)
	s.mb.RecordNsxtNodeNetworkPacketCountDataPoint(colTime, i.stats.RxErrors, metadata.AttributeDirectionReceived, metadata.AttributePacketTypeErrored)
	successRxPackets := i.stats.RxPackets - i.stats.RxDropped - i.stats.RxErrors
	s.mb.RecordNsxtNodeNetworkPacketCountDataPoint(colTime, successRxPackets, metadata.AttributeDirectionReceived, metadata.AttributePacketTypeSuccess)

	s.mb.RecordNsxtNodeNetworkPacketCountDataPoint(colTime, i.stats.TxDropped, metadata.AttributeDirectionTransmitted, metadata.AttributePacketTypeDropped)
	s.mb.RecordNsxtNodeNetworkPacketCountDataPoint(colTime, i.stats.TxErrors, metadata.AttributeDirectionTransmitted, metadata.AttributePacketTypeErrored)
	successTxPackets := i.stats.TxPackets - i.stats.TxDropped - i.stats.TxErrors
	s.mb.RecordNsxtNodeNetworkPacketCountDataPoint(colTime, successTxPackets, metadata.AttributeDirectionTransmitted, metadata.AttributePacketTypeSuccess)

	s.mb.RecordNsxtNodeNetworkIoDataPoint(colTime, i.stats.RxBytes, metadata.AttributeDirectionReceived)
	s.mb.RecordNsxtNodeNetworkIoDataPoint(colTime, i.stats.TxBytes, metadata.AttributeDirectionTransmitted)

	s.rb.SetDeviceID(i.iFace.InterfaceId)
	s.rb.SetNsxtNodeName(nodeProps.Name)
	s.rb.SetNsxtNodeType(nodeProps.ResourceType)
	s.rb.SetNsxtNodeID(nodeProps.ID)
	s.mb.EmitForResource(metadata.WithResource(s.rb.Emit()))
}

func (s *scraper) recordNode(
	colTime pcommon.Timestamp,
	info *nodeInfo,
) {
	if info.stats == nil {
		return
	}

	ss := info.stats.SystemStatus
	s.mb.RecordNsxtNodeCPUUtilizationDataPoint(colTime, ss.CPUUsage.AvgCPUCoreUsageDpdk, metadata.AttributeClassDatapath)
	s.mb.RecordNsxtNodeCPUUtilizationDataPoint(colTime, ss.CPUUsage.AvgCPUCoreUsageNonDpdk, metadata.AttributeClassServices)
	s.mb.RecordNsxtNodeMemoryUsageDataPoint(colTime, int64(ss.MemUsed))
	s.mb.RecordNsxtNodeMemoryCacheUsageDataPoint(colTime, int64(ss.MemCache))

	s.mb.RecordNsxtNodeFilesystemUsageDataPoint(colTime, int64(ss.DiskSpaceUsed), metadata.AttributeDiskStateUsed)
	availableStorage := ss.DiskSpaceTotal - ss.DiskSpaceUsed
	s.mb.RecordNsxtNodeFilesystemUsageDataPoint(colTime, int64(availableStorage), metadata.AttributeDiskStateAvailable)
	// ensure division by zero is safeguarded
	s.mb.RecordNsxtNodeFilesystemUtilizationDataPoint(colTime, float64(ss.DiskSpaceUsed)/math.Max(float64(ss.DiskSpaceTotal), 1))

	s.rb.SetNsxtNodeName(info.nodeProps.Name)
	s.rb.SetNsxtNodeID(info.nodeProps.ID)
	s.rb.SetNsxtNodeType(info.nodeType)
	s.mb.EmitForResource(metadata.WithResource(s.rb.Emit()))
}

func clusterNodeType(node dm.ClusterNode) string {
	if node.ControllerRole != nil {
		return "controller"
	}
	return "manager"
}
