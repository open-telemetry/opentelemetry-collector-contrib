// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sync"
	"time"
	"unsafe"

	"github.com/prometheus/client_golang/prometheus"
	commonconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
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
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/apiserver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/sharedpromconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/targetallocator"
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
	apiServerManager       *apiserver.Manager
	registry               *prometheus.Registry
	registerer             prometheus.Registerer
	unregisterMetrics      func()
}

// New creates a new prometheus.Receiver reference.
func newPrometheusReceiver(set receiver.Settings, cfg *Config, next consumer.Metrics) (*pReceiver, error) {
	if err := cfg.PrometheusConfig.Reload(); err != nil {
		return nil, fmt.Errorf("failed to reload Prometheus config: %w", err)
	}

	baseCfg := promconfig.Config(*cfg.PrometheusConfig)
	registry := prometheus.NewRegistry()
	registerer := prometheus.WrapRegistererWith(
		prometheus.Labels{"receiver": set.ID.String()},
		registry,
	)
	sharedCfg := sharedpromconfig.NewConfig(&baseCfg)
	var apiServerManager *apiserver.Manager
	if cfg.APIServer.Enabled {
		apiServerManager = apiserver.NewManager(
			set,
			&cfg.APIServer,
			sharedCfg,
			registry,
			registerer,
		)
	}

	pr := &pReceiver{
		cfg:          cfg,
		consumer:     next,
		settings:     set,
		configLoaded: make(chan struct{}),
		registerer:   registerer,
		registry:     registry,
		targetAllocatorManager: targetallocator.NewManager(
			set,
			cfg.TargetAllocator.Get(),
			sharedCfg,
		),
		apiServerManager: apiServerManager,
	}
	return pr, nil
}

// Start is the method that starts Prometheus scraping. It
// is controlled by having previously defined a Configuration using perhaps New.
func (r *pReceiver) Start(ctx context.Context, host component.Host) error {
	return r.start(ctx, host, prometheusComponentTestOptions{})
}

func (r *pReceiver) start(ctx context.Context, host component.Host, opts prometheusComponentTestOptions) error {
	discoveryCtx, cancel := context.WithCancel(context.Background())
	r.cancelFunc = cancel

	logger := slog.New(zapslog.NewHandler(r.settings.Logger.Core()))

	err := r.initPrometheusComponents(discoveryCtx, logger, host, opts)
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

	if r.apiServerManager != nil {
		err := r.apiServerManager.Start(ctx, host, r.scrapeManager)
		if err != nil {
			r.settings.Logger.Error("Failed to start APIServer", zap.Error(err))
		}
	}

	return nil
}

func (r *pReceiver) initPrometheusComponents(
	ctx context.Context, logger *slog.Logger, host component.Host,
	opts prometheusComponentTestOptions,
) error {
	// Register the metrics needed by service discovery mechanisms.
	sdMetrics, err := discovery.CreateAndRegisterSDMetrics(r.registerer)
	if err != nil {
		return fmt.Errorf("failed to register service discovery metrics: %w", err)
	}

	var discoveryManagerTestOptions []func(*discovery.Manager)
	if opts.discovery.updatert > 0 {
		discoveryManagerTestOptions = append(discoveryManagerTestOptions, discovery.Updatert(opts.discovery.updatert))
	}
	r.discoveryManager = discovery.NewManager(ctx, logger, r.registerer, sdMetrics, discoveryManagerTestOptions...)
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

	store, err := internal.NewAppendable(
		r.consumer,
		r.settings,
		!r.cfg.ignoreMetadata,
		r.cfg.PrometheusConfig.GlobalConfig.ExternalLabels,
		r.cfg.TrimMetricSuffixes,
	)
	if err != nil {
		return err
	}

	scrapeOpts := r.initScrapeOptions(opts.scrape)

	// for testing only
	if r.cfg.skipOffsetting {
		optsValue := reflect.ValueOf(scrapeOpts).Elem()
		field := optsValue.FieldByName("skipJitterOffsetting")
		reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).
			Elem().
			Set(reflect.ValueOf(true))
	}

	scrapeManager, err := scrape.NewManager(scrapeOpts, logger, nil, nil, store, r.registerer)
	if err != nil {
		return err
	}
	r.scrapeManager = scrapeManager

	r.unregisterMetrics = func() {
		sdMetrics.RefreshManager.Unregister()
		for _, sdMetric := range sdMetrics.MechanismMetrics {
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

func (r *pReceiver) initScrapeOptions(o prometheusScrapeTestOptions) *scrape.Options {
	opts := &scrape.Options{
		DiscoveryReloadInterval: model.Duration(o.discoveryReloadInterval),
		PassMetadataInContext:   true,
		HTTPClientOptions: []commonconfig.HTTPClientOption{
			commonconfig.WithUserAgent(r.settings.BuildInfo.Command + "/" + r.settings.BuildInfo.Version),
		},
		EnableStartTimestampZeroIngestion: metadata.ReceiverPrometheusreceiverEnableCreatedTimestampZeroIngestionFeatureGate.IsEnabled(),
		ScrapeOnShutdown:                  r.cfg.ScrapeOnShutdown,
		DiscoveryReloadOnStartup:          r.cfg.DiscoveryReloadOnStartup,
		InitialScrapeOffset:               r.cfg.InitialScrapeOffset,
	}

	return opts
}

// gcInterval returns the longest scrape interval used by a scrape config,
// plus a delta to prevent race conditions.
// This ensures jobs are not garbage collected between scrapes.
func gcInterval(cfg *PromConfig) time.Duration {
	gcInterval := max(time.Duration(cfg.GlobalConfig.ScrapeInterval)+gcIntervalDelta, defaultGCInterval)
	for _, scrapeConfig := range cfg.ScrapeConfigs {
		if time.Duration(scrapeConfig.ScrapeInterval)+gcIntervalDelta > gcInterval {
			gcInterval = time.Duration(scrapeConfig.ScrapeInterval) + gcIntervalDelta
		}
	}
	return gcInterval
}

// Shutdown stops and cancels the underlying Prometheus scrapers.
func (r *pReceiver) Shutdown(ctx context.Context) error {
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
	if r.apiServerManager != nil {
		if err := r.apiServerManager.Shutdown(ctx); err != nil {
			return err
		}
	}
	return nil
}

type prometheusComponentTestOptions struct {
	discovery prometheusDiscoveryTestOptions
	scrape    prometheusScrapeTestOptions
}

type prometheusDiscoveryTestOptions struct {
	// updatert is the interval for updating targets.
	//
	// If zero, the default (5s) from Prometheus is used.
	// This option is primarily for testing.
	updatert time.Duration
}

type prometheusScrapeTestOptions struct {
	// discoveryReloadInterval is the interval for reloading
	// scrape configurations.
	//
	// If zero, the default (5s) from Prometheus is used.
	// This option is primarily for testing.
	discoveryReloadInterval time.Duration
}
