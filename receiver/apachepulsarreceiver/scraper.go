package apachepulsarreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

// apachePulsarScraper handles the scraping of Apache Pulsar metrics
type apachePulsarScraper struct {
	logger   *zap.Logger
	cfg      *Config
	settings receiver.CreateSettings
}

func newScraper(logger *zap.Logger, cfg *Config, settings receiver.CreateSettings) *apachePulsarScraper {
	return &apachePulsarScraper{logger, cfg, settings}
}

func (p *apachePulsarScraper) start(_ context.Context, _ component.Host) (err error) {
	return nil
}

func (p *apachePulsarScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	md := pmetric.NewMetrics()

	return md, nil
}
