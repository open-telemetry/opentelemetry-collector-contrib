package httpjsonreceiver

import (
	"context"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

type Receiver struct {
	cfg      *Config
	consumer consumer.Metrics
	logger   *zap.Logger
	settings receiver.Settings

	client  *http.Client
	scraper *Scraper

	wg     sync.WaitGroup
	stopCh chan struct{}
	ticker *time.Ticker
}

func NewReceiver(
	cfg *Config,
	consumer consumer.Metrics,
	settings receiver.Settings,
) (*Receiver, error) {
	// Validate and apply defaults
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &Receiver{
		cfg:      cfg,
		consumer: consumer,
		logger:   settings.Logger,
		settings: settings,
		stopCh:   make(chan struct{}),
	}, nil
}

func (r *Receiver) Start(ctx context.Context, host component.Host) error {
	r.logger.Info("Starting HTTP JSON receiver",
		zap.Int("endpoints", len(r.cfg.Endpoints)),
		zap.Duration("interval", r.cfg.CollectionInterval))

	// Create HTTP client with timeout
	r.client = &http.Client{
		Timeout: r.cfg.Timeout,
	}

	r.scraper = NewScraper(r.cfg, r.client, r.logger)

	r.wg.Add(1)
	go r.collectLoop(ctx)

	return nil
}

func (r *Receiver) Shutdown(ctx context.Context) error {
	r.logger.Info("Shutting down HTTP JSON receiver")

	if r.ticker != nil {
		r.ticker.Stop()
	}

	close(r.stopCh)
	r.wg.Wait()

	return nil
}

func (r *Receiver) collectLoop(ctx context.Context) {
	defer r.wg.Done()

	if r.cfg.InitialDelay > 0 {
		select {
		case <-time.After(r.cfg.InitialDelay):
		case <-r.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}

	r.ticker = time.NewTicker(r.cfg.CollectionInterval)
	defer r.ticker.Stop()

	r.collect(ctx)

	for {
		select {
		case <-r.ticker.C:
			r.collect(ctx)
		case <-r.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (r *Receiver) collect(ctx context.Context) {
	r.logger.Debug("Starting metric collection")

	metrics, err := r.scraper.Scrape(ctx)
	if err != nil {
		r.logger.Error("Failed to scrape metrics", zap.Error(err))
		return
	}

	if metrics.MetricCount() == 0 {
		r.logger.Debug("No metrics collected")
		return
	}

	if err := r.consumer.ConsumeMetrics(ctx, metrics); err != nil {
		r.logger.Error("Failed to consume metrics", zap.Error(err))
	} else {
		r.logger.Debug("Successfully sent metrics",
			zap.Int("count", metrics.MetricCount()))
	}
}
