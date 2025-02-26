// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"regexp"
	"sync"
	"time"
	"unsafe"

	"github.com/prometheus/client_golang/prometheus"
	commonconfig "github.com/prometheus/common/config"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/scrape"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/targetallocator"
)

const (
	defaultGCInterval = 2 * time.Minute
	gcIntervalDelta   = 1 * time.Minute
)

// pReceiver is the type that provides Prometheus scraper/receiver functionality.
type pReceiver struct {
	cfg            *Config
	consumer       consumer.Metrics
	cancelFunc     context.CancelFunc
	configLoaded   chan struct{}
	loadConfigOnce sync.Once

	settings               receiver.Settings
	scrapeManager          *scrape.Manager
	discoveryManager       *discovery.Manager
	targetAllocatorManager *targetallocator.Manager
	registerer             prometheus.Registerer
	unregisterMetrics      func()
	skipOffsetting         bool // for testing only
}

// New creates a new prometheus.Receiver reference.
func newPrometheusReceiver(set receiver.Settings, cfg *Config, next consumer.Metrics) *pReceiver {
	baseCfg := promconfig.Config(*cfg.PrometheusConfig)
	pr := &pReceiver{
		cfg:          cfg,
		consumer:     next,
		settings:     set,
		configLoaded: make(chan struct{}),
		registerer: prometheus.WrapRegistererWith(
			prometheus.Labels{"receiver": set.ID.String()},
			prometheus.DefaultRegisterer),
		targetAllocatorManager: targetallocator.NewManager(
			set,
			cfg.TargetAllocator,
			&baseCfg,
			enableNativeHistogramsGate.IsEnabled(),
		),
	}
	return pr
}

// Start is the method that starts Prometheus scraping. It
// is controlled by having previously defined a Configuration using perhaps New.
func (r *pReceiver) Start(ctx context.Context, host component.Host) error {
	discoveryCtx, cancel := context.WithCancel(context.Background())
	r.cancelFunc = cancel

	logger := slog.New(zapslog.NewHandler(r.settings.Logger.Core()))

	err := r.initPrometheusComponents(discoveryCtx, logger, host)
	if err != nil {
		r.settings.Logger.Error("Failed to initPrometheusComponents Prometheus components", zap.Error(err))
		return err
	}

	err = r.targetAllocatorManager.Start(ctx, host, r.scrapeManager, r.discoveryManager)
	if err != nil {
		return err
	}

	r.loadConfigOnce.Do(func() {
		close(r.configLoaded)
	})

	return nil
}

func (r *pReceiver) initPrometheusComponents(ctx context.Context, logger *slog.Logger, host component.Host) error {
	// Some SD mechanisms use the "refresh" package, which has its own metrics.
	refreshSdMetrics := discovery.NewRefreshMetrics(r.registerer)

	// Register the metrics specific for each SD mechanism, and the ones for the refresh package.
	sdMetrics, err := discovery.RegisterSDMetrics(r.registerer, refreshSdMetrics)
	if err != nil {
		return fmt.Errorf("failed to register service discovery metrics: %w", err)
	}
	r.discoveryManager = discovery.NewManager(ctx, logger, r.registerer, sdMetrics)
	if r.discoveryManager == nil {
		// NewManager can sometimes return nil if it encountered an error, but
		// the error message is logged separately.
		return errors.New("failed to create discovery manager")
	}

	go func() {
		r.settings.Logger.Info("Starting discovery manager")
		if err = r.discoveryManager.Run(); err != nil && !errors.Is(err, context.Canceled) {
			r.settings.Logger.Error("Discovery manager failed", zap.Error(err))
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
		}
	}()

	var startTimeMetricRegex *regexp.Regexp
	if r.cfg.StartTimeMetricRegex != "" {
		startTimeMetricRegex, err = regexp.Compile(r.cfg.StartTimeMetricRegex)
		if err != nil {
			return err
		}
	}

	store, err := internal.NewAppendable(
		r.consumer,
		r.settings,
		gcInterval(r.cfg.PrometheusConfig),
		r.cfg.UseStartTimeMetric,
		startTimeMetricRegex,
		useCreatedMetricGate.IsEnabled(),
		enableNativeHistogramsGate.IsEnabled(),
		r.cfg.PrometheusConfig.GlobalConfig.ExternalLabels,
		r.cfg.TrimMetricSuffixes,
	)
	if err != nil {
		return err
	}

	opts := &scrape.Options{
		PassMetadataInContext: true,
		ExtraMetrics:          r.cfg.ReportExtraScrapeMetrics,
		HTTPClientOptions: []commonconfig.HTTPClientOption{
			commonconfig.WithUserAgent(r.settings.BuildInfo.Command + "/" + r.settings.BuildInfo.Version),
		},
		EnableCreatedTimestampZeroIngestion: true,
	}

	if enableNativeHistogramsGate.IsEnabled() {
		opts.EnableNativeHistogramsIngestion = true
	}

	// for testing only
	if r.skipOffsetting {
		optsValue := reflect.ValueOf(opts).Elem()
		field := optsValue.FieldByName("skipOffsetting")
		reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).
			Elem().
			Set(reflect.ValueOf(true))
	}

	scrapeManager, err := scrape.NewManager(opts, logger, nil, store, r.registerer)
	if err != nil {
		return err
	}
	r.scrapeManager = scrapeManager

	r.unregisterMetrics = func() {
		refreshSdMetrics.Unregister()
		for _, sdMetric := range sdMetrics {
			sdMetric.Unregister()
		}
		r.discoveryManager.UnregisterMetrics()
		r.scrapeManager.UnregisterMetrics()
	}

	go func() {
		// The scrape manager needs to wait for the configuration to be loaded before beginning
		<-r.configLoaded
		r.settings.Logger.Info("Starting scrape manager")
		if err := r.scrapeManager.Run(r.discoveryManager.SyncCh()); err != nil {
			r.settings.Logger.Error("Scrape manager failed", zap.Error(err))
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
		}
	}()
	return nil
}

// gcInterval returns the longest scrape interval used by a scrape config,
// plus a delta to prevent race conditions.
// This ensures jobs are not garbage collected between scrapes.
func gcInterval(cfg *PromConfig) time.Duration {
	gcInterval := defaultGCInterval
	if time.Duration(cfg.GlobalConfig.ScrapeInterval)+gcIntervalDelta > gcInterval {
		gcInterval = time.Duration(cfg.GlobalConfig.ScrapeInterval) + gcIntervalDelta
	}
	for _, scrapeConfig := range cfg.ScrapeConfigs {
		if time.Duration(scrapeConfig.ScrapeInterval)+gcIntervalDelta > gcInterval {
			gcInterval = time.Duration(scrapeConfig.ScrapeInterval) + gcIntervalDelta
		}
	}
	return gcInterval
}

// Shutdown stops and cancels the underlying Prometheus scrapers.
func (r *pReceiver) Shutdown(context.Context) error {
	if r.cancelFunc != nil {
		r.cancelFunc()
	}
	if r.scrapeManager != nil {
		r.scrapeManager.Stop()
	}
	if r.targetAllocatorManager != nil {
		r.targetAllocatorManager.Shutdown()
	}
	if r.unregisterMetrics != nil {
		r.unregisterMetrics()
	}
	return nil
}
