// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"
	"unsafe"

	grafanaRegexp "github.com/grafana/regexp"
	"github.com/mwitkow/go-conntrack"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	commonconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/route"
	"github.com/prometheus/common/version"
	toolkit_web "github.com/prometheus/exporter-toolkit/web"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/util/httputil"
	"github.com/prometheus/prometheus/web"
	api_v1 "github.com/prometheus/prometheus/web/api/v1"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
	"golang.org/x/net/netutil"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/targetallocator"
)

const (
	defaultGCInterval = 2 * time.Minute
	gcIntervalDelta   = 1 * time.Minute

	// Use same settings as Prometheus web server
	maxConnections     = 512
	readTimeoutMinutes = 10
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
	apiServer              *http.Server
	registry               *prometheus.Registry
	registerer             prometheus.Registerer
	unregisterMetrics      func()
	skipOffsetting         bool // for testing only
}

// New creates a new prometheus.Receiver reference.
func newPrometheusReceiver(set receiver.Settings, cfg *Config, next consumer.Metrics) *pReceiver {
	baseCfg := promconfig.Config(*cfg.PrometheusConfig)
	registry := prometheus.NewRegistry()
	registerer := prometheus.WrapRegistererWith(
		prometheus.Labels{"receiver": set.ID.String()},
		registry)
	pr := &pReceiver{
		cfg:          cfg,
		consumer:     next,
		settings:     set,
		configLoaded: make(chan struct{}),
		registerer:   registerer,
		registry:     registry,
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

	if r.cfg.APIServer != nil && r.cfg.APIServer.Enabled {
		err = r.initAPIServer(discoveryCtx, host)
		if err != nil {
			r.settings.Logger.Error("Failed to initAPIServer", zap.Error(err))
		}
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

func (r *pReceiver) initAPIServer(ctx context.Context, host component.Host) error {
	r.settings.Logger.Info("Starting Prometheus API server")

	// If allowed CORS origins are provided in the receiver config, combine them into a single regex since the Prometheus API server requires this format.
	var corsOriginRegexp *grafanaRegexp.Regexp
	if r.cfg.APIServer.ServerConfig.CORS != nil && len(r.cfg.APIServer.ServerConfig.CORS.AllowedOrigins) > 0 {
		var combinedOriginsBuilder strings.Builder
		combinedOriginsBuilder.WriteString(r.cfg.APIServer.ServerConfig.CORS.AllowedOrigins[0])
		for _, origin := range r.cfg.APIServer.ServerConfig.CORS.AllowedOrigins[1:] {
			combinedOriginsBuilder.WriteString("|")
			combinedOriginsBuilder.WriteString(origin)
		}
		combinedRegexp, err := grafanaRegexp.Compile(combinedOriginsBuilder.String())
		if err != nil {
			return fmt.Errorf("failed to compile combined CORS allowed origins into regex: %s", err.Error())
		}
		corsOriginRegexp = combinedRegexp
	}

	// If read timeout is not set in the receiver config, use the default Prometheus value.
	readTimeout := r.cfg.APIServer.ServerConfig.ReadTimeout
	if readTimeout == 0 {
		readTimeout = time.Duration(readTimeoutMinutes) * time.Minute
	}

	o := &web.Options{
		ScrapeManager:   r.scrapeManager,
		Context:         ctx,
		ListenAddresses: []string{r.cfg.APIServer.ServerConfig.Endpoint},
		ExternalURL: &url.URL{
			Scheme: "http",
			Host:   r.cfg.APIServer.ServerConfig.Endpoint,
			Path:   "",
		},
		RoutePrefix:    "/",
		ReadTimeout:    readTimeout,
		PageTitle:      "Prometheus Receiver",
		Flags:          make(map[string]string),
		MaxConnections: maxConnections,
		IsAgent:        true,
		Registerer:     r.registerer,
		Gatherer:       r.registry,
		CORSOrigin:     corsOriginRegexp,
	}

	// Creates the API object in the same way as the Prometheus web package: https://github.com/prometheus/prometheus/blob/6150e1ca0ede508e56414363cc9062ef522db518/web/web.go#L314-L354
	// Anything not defined by the options above will be nil, such as o.QueryEngine, o.Storage, etc. IsAgent=true, so these being nil is expected by Prometheus.
	factorySPr := func(_ context.Context) api_v1.ScrapePoolsRetriever { return o.ScrapeManager }
	factoryTr := func(_ context.Context) api_v1.TargetRetriever { return o.ScrapeManager }
	factoryAr := func(_ context.Context) api_v1.AlertmanagerRetriever { return nil }
	factoryRr := func(_ context.Context) api_v1.RulesRetriever { return nil }
	var app storage.Appendable
	logger := promslog.NewNopLogger()

	apiV1 := api_v1.NewAPI(o.QueryEngine, o.Storage, app, o.ExemplarStorage, factorySPr, factoryTr, factoryAr,

		// This ensures that any changes to the config made, even by the target allocator, are reflected in the API.
		func() promconfig.Config {
			return *(*promconfig.Config)(r.cfg.PrometheusConfig)
		},
		o.Flags, // nil
		api_v1.GlobalURLOptions{
			ListenAddress: o.ListenAddresses[0],
			Host:          o.ExternalURL.Host,
			Scheme:        o.ExternalURL.Scheme,
		},
		func(f http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				f(w, r)
			}
		},
		o.LocalStorage,   // nil
		o.TSDBDir,        // nil
		o.EnableAdminAPI, // nil
		logger,
		factoryRr,
		o.RemoteReadSampleLimit,      // nil
		o.RemoteReadConcurrencyLimit, // nil
		o.RemoteReadBytesInFrame,     // nil
		o.IsAgent,
		o.CORSOrigin,
		func() (api_v1.RuntimeInfo, error) {
			status := api_v1.RuntimeInfo{
				GoroutineCount: runtime.NumGoroutine(),
				GOMAXPROCS:     runtime.GOMAXPROCS(0),
				GOMEMLIMIT:     debug.SetMemoryLimit(-1),
				GOGC:           os.Getenv("GOGC"),
				GODEBUG:        os.Getenv("GODEBUG"),
			}

			return status, nil
		},
		&web.PrometheusVersion{
			Version:   version.Version,
			Revision:  version.Revision,
			Branch:    version.Branch,
			BuildUser: version.BuildUser,
			BuildDate: version.BuildDate,
			GoVersion: version.GoVersion,
		},
		o.NotificationsGetter,
		o.NotificationsSub,
		o.Gatherer,
		o.Registerer,
		nil,
		o.EnableRemoteWriteReceiver,
		o.AcceptRemoteWriteProtoMsgs,
		o.EnableOTLPWriteReceiver,
	)

	// Create listener and monitor with conntrack in the same way as the Prometheus web package: https://github.com/prometheus/prometheus/blob/6150e1ca0ede508e56414363cc9062ef522db518/web/web.go#L564-L579
	listener, err := r.cfg.APIServer.ServerConfig.ToListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to create listener: %s", err.Error())
	}
	listener = netutil.LimitListener(listener, o.MaxConnections)
	listener = conntrack.NewListener(listener,
		conntrack.TrackWithName("http"),
		conntrack.TrackWithTracing())

	// Run the API server in the same way as the Prometheus web package: https://github.com/prometheus/prometheus/blob/6150e1ca0ede508e56414363cc9062ef522db518/web/web.go#L582-L630
	mux := http.NewServeMux()
	promHandler := promhttp.HandlerFor(o.Gatherer, promhttp.HandlerOpts{Registry: o.Registerer})
	mux.Handle("/metrics", promHandler)

	// This is the path the web package uses, but the router above with no prefix can also be Registered by apiV1 instead.
	apiPath := "/api"
	if o.RoutePrefix != "/" {
		apiPath = o.RoutePrefix + apiPath
		logger.Info("Router prefix", "prefix", o.RoutePrefix)
	}
	av1 := route.New().
		WithInstrumentation(setPathWithPrefix(apiPath + "/v1"))
	apiV1.Register(av1)
	mux.Handle(apiPath+"/v1/", http.StripPrefix(apiPath+"/v1", av1))

	spanNameFormatter := otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
		return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
	})
	r.apiServer, err = r.cfg.APIServer.ServerConfig.ToServer(ctx, host, r.settings.TelemetrySettings, otelhttp.NewHandler(mux, "", spanNameFormatter))
	if err != nil {
		return err
	}
	webconfig := ""

	go func() {
		if err := toolkit_web.Serve(listener, r.apiServer, &toolkit_web.FlagConfig{WebConfigFile: &webconfig}, logger); err != nil {
			r.settings.Logger.Error("API server failed", zap.Error(err))
		}
	}()

	return nil
}

// Helper function from the Prometheus web package: https://github.com/prometheus/prometheus/blob/6150e1ca0ede508e56414363cc9062ef522db518/web/web.go#L582-L630
func setPathWithPrefix(prefix string) func(handlerName string, handler http.HandlerFunc) http.HandlerFunc {
	return func(_ string, handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			handler(w, r.WithContext(httputil.ContextWithPath(r.Context(), prefix+r.URL.Path)))
		}
	}
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
	if r.apiServer != nil {
		err := r.apiServer.Shutdown(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}
