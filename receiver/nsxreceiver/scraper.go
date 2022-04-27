package nsxreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxreceiver"

import (
	"context"
	"sync"

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
	client, err := newClient(s.config, s.settings, host)
	if err != nil {
		return err
	}
	s.client = client
	return nil
}

func (s *scraper) scrape(ctx context.Context) (pdata.Metrics, error) {

	errs := &scrapererror.ScrapeErrors{}
	r := s.retrieve(ctx, errs)
	s.process(r)
	return pdata.NewMetrics(), nil
}

type retrieval struct {
	nodes   []dm.NodeStat
	routers []dm.NetworkInterface
	sync.RWMutex
}

func (s *scraper) retrieve(ctx context.Context, errs *scrapererror.ScrapeErrors) *retrieval {
	r := &retrieval{}

	wg := &sync.WaitGroup{}

	wg.Add(2)
	go s.retrieveNodes(ctx, r, wg, errs)
	go s.retrieveRouters(ctx, r, wg, errs)
	wg.Wait()

	return r
}

func (s *scraper) retrieveNodes(
	ctx context.Context,
	r *retrieval,
	wg *sync.WaitGroup,
	errs *scrapererror.ScrapeErrors,
) {
	defer wg.Done()

	nodes, err := s.client.Nodes(ctx)
	if err != nil {
		s.logger.Error("unable to retrieve nodes", zap.Field{Key: "error", String: err.Error()})
	}
	r.Lock()
	r.nodes = nodes
	r.Unlock()

	for _, n := range nodes {
		interfaces, err := s.client.Interfaces(ctx, n.ID)
		if err != nil {
			errs.AddPartial(1, err)
			continue
		}
		n.Interfaces = interfaces
	}
}

func (s *scraper) retrieveRouters(
	ctx context.Context,
	r *retrieval,
	wg *sync.WaitGroup,
	errs *scrapererror.ScrapeErrors,
) {
	defer wg.Done()
}

func (s *scraper) process(retrieval *retrieval) {
	for _, n := range retrieval.nodes {
		for _, i := range n.Interfaces {
			s.recordNodeInterface(n, i)
		}
	}
}

func (s *scraper) recordNodeInterface(node dm.NodeStat, i dm.NetworkInterface) {

}
