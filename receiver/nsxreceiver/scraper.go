package nsxreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxreceiver"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxreceiver/internal/metadata"
	dm "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxreceiver/internal/model"
)

type scraper struct {
	config   *Config
	settings component.TelemetrySettings
	client   *nsxClient
	mb       *metadata.MetricsBuilder
	logger   *zap.Logger
}

func newScraper(cfg *Config, settings component.TelemetrySettings) *scraper {
	return &scraper{
		config:   cfg,
		settings: settings,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsConfig.Settings),
		logger:   settings.Logger,
	}
}

func (s *scraper) start(ctx context.Context, host component.Host) error {
	client, err := newClient(s.config, s.settings, host, s.logger.Named("client"))
	if err != nil {
		return err
	}
	s.client = client
	return nil
}

func (s *scraper) scrape(ctx context.Context) (pdata.Metrics, error) {

	colTime := pdata.NewTimestampFromTime(time.Now())
	errs := &scrapererror.ScrapeErrors{}
	r := s.retrieve(ctx, errs)

	s.process(r, colTime, errs)
	return s.mb.Emit(), errs.Combine()
}

type retrieval struct {
	nodes []nodeInfo
}

type nodeInfo struct {
	node       dm.TransportNode
	interfaces []dm.NetworkInterface
	stats      dm.NodeStatus
}

func (s *scraper) retrieve(ctx context.Context, errs *scrapererror.ScrapeErrors) *retrieval {
	r := &retrieval{}

	nodes, err := s.retrieveNodes(ctx, r, errs)

	wg := &sync.WaitGroup{}
	for _, n := range r.nodes {
		go s.retrieveInterfaces(ctx, n, wg, errs)
		go s.retrieveNodeStats(ctx, n, wg, errs)
	}
	wg.Wait()

	return r
}

func (s *scraper) retrieveNodes(
	ctx context.Context,
	r *retrieval,
	errs *scrapererror.ScrapeErrors,
) ([]dm.TransportNode, error) {
	nodes, err := s.client.TransportNodes(ctx)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	r.nodes = nodes
}

func (s *scraper) retrieveInterfaces(
	ctx context.Context,
	node dm.TransportNode,
	wg *sync.WaitGroup,
	errs *scrapererror.ScrapeErrors,
) {
	defer wg.Done()
	interfaces, err := s.client.Interfaces(ctx, node.ID)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	interfaces
}

func (s *scraper) retrieveNodeStats(
	ctx context.Context,
	node dm.TransportNode,
	wg *sync.WaitGroup,
	errs *scrapererror.ScrapeErrors,
) {
	defer wg.Done()
	ns, err := s.client.NodeStatus(ctx, node.ID)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	node.Stats = ns
}

func (s *scraper) process(retrieval *retrieval, colTime pdata.Timestamp, errs *scrapererror.ScrapeErrors) {
	for _, n := range retrieval.nodes {
		for _, i := range n.Interfaces {
			s.recordNodeInterface(n, i)
		}
		s.recordNode(colTime, n)
	}
}

func (s *scraper) recordNodeInterface(node dm.TransportNode, i dm.NetworkInterface) {

}

func (s *scraper) recordNode(colTime pdata.Timestamp, node dm.TransportNode) {
	s.logger.Info(fmt.Sprintf("Node ID: %s", node.ID))
	ss := node.Stats.SystemStatus
	s.mb.RecordNsxNodeCPUUtilizationDataPoint(colTime, ss.CPUUsage.AvgCPUCoreUsageDpdk, metadata.AttributeCPUProcessClass.Datapath)
	s.mb.RecordNsxNodeCPUUtilizationDataPoint(colTime, ss.CPUUsage.AvgCPUCoreUsageNonDpdk, metadata.AttributeCPUProcessClass.Services)
	s.mb.RecordNodeMemoryUsageDataPoint(colTime, int64(ss.MemUsed))
	s.mb.RecordNodeCacheMemoryUsageDataPoint(colTime, int64(ss.MemUsed))

	s.mb.EmitForResource(
		metadata.WithNsxNodeName(node.Name),
		metadata.WithNsxNodeType(nodeType(node)),
	)
}

func nodeType(node dm.TransportNode) string {
	switch {
	case node.ResourceType == "TransportNode":
		return "transport"
	case node.ManagerRole != nil:
		return "manager"
	case node.ControllerRole != nil:
		return "controller"
	default:
		return ""
	}
}
