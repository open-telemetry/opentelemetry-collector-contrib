// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
	"unsafe"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/mwitkow/go-conntrack"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	commonconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/route"
	toolkit_web "github.com/prometheus/exporter-toolkit/web"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	promHTTP "github.com/prometheus/prometheus/discovery/http"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/util/httputil"
	"github.com/prometheus/prometheus/web"
	api_v1 "github.com/prometheus/prometheus/web/api/v1"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"
	"golang.org/x/net/netutil"
	"gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"
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
	cfg                 *Config
	consumer            consumer.Metrics
	cancelFunc          context.CancelFunc
	targetAllocatorStop chan struct{}
	configLoaded        chan struct{}
	loadConfigOnce      sync.Once

	settings          receiver.Settings
	scrapeManager     *scrape.Manager
	discoveryManager  *discovery.Manager
	httpClient        *http.Client
	registerer        prometheus.Registerer
	unregisterMetrics func()
	skipOffsetting    bool // for testing only
	apiServer         *http.Server
	registry          *prometheus.Registry
}

// New creates a new prometheus.Receiver reference.
func newPrometheusReceiver(set receiver.Settings, cfg *Config, next consumer.Metrics) *pReceiver {
	registry := prometheus.NewRegistry()
	registerer := prometheus.WrapRegistererWith(
		prometheus.Labels{"receiver": set.ID.String()},
		registry)
	pr := &pReceiver{
		cfg:                 cfg,
		consumer:            next,
		settings:            set,
		configLoaded:        make(chan struct{}),
		targetAllocatorStop: make(chan struct{}),
		registerer:          registerer,
		registry:            registry,
	}
	return pr
}

// Start is the method that starts Prometheus scraping. It
// is controlled by having previously defined a Configuration using perhaps New.
func (r *pReceiver) Start(ctx context.Context, host component.Host) error {
	discoveryCtx, cancel := context.WithCancel(context.Background())
	r.cancelFunc = cancel

	logger := internal.NewZapToGokitLogAdapter(r.settings.Logger)

	// add scrape configs defined by the collector configs
	baseCfg := r.cfg.PrometheusConfig

	err := r.initPrometheusComponents(discoveryCtx, logger)
	if err != nil {
		r.settings.Logger.Error("Failed to initPrometheusComponents Prometheus components", zap.Error(err))
		return err
	}

	err = r.applyCfg(baseCfg)
	if err != nil {
		r.settings.Logger.Error("Failed to apply new scrape configuration", zap.Error(err))
		return err
	}

	if r.cfg.APIServer != nil && r.cfg.APIServer.Enabled {
		r.initAPIServer(discoveryCtx, host)
	}

	allocConf := r.cfg.TargetAllocator
	if allocConf != nil {
		r.httpClient, err = r.cfg.TargetAllocator.ToClient(ctx, host, r.settings.TelemetrySettings)
		if err != nil {
			r.settings.Logger.Error("Failed to create http client", zap.Error(err))
			return err
		}
		err = r.startTargetAllocator(allocConf, baseCfg)
		if err != nil {
			return err
		}
	}

	r.loadConfigOnce.Do(func() {
		close(r.configLoaded)
	})

	return nil
}

func (r *pReceiver) startTargetAllocator(allocConf *TargetAllocator, baseCfg *PromConfig) error {
	r.settings.Logger.Info("Starting target allocator discovery")
	// immediately sync jobs, not waiting for the first tick
	savedHash, err := r.syncTargetAllocator(uint64(0), allocConf, baseCfg)
	if err != nil {
		return err
	}
	go func() {
		targetAllocatorIntervalTicker := time.NewTicker(allocConf.Interval)
		for {
			select {
			case <-targetAllocatorIntervalTicker.C:
				hash, newErr := r.syncTargetAllocator(savedHash, allocConf, baseCfg)
				if newErr != nil {
					r.settings.Logger.Error(newErr.Error())
					continue
				}
				savedHash = hash
			case <-r.targetAllocatorStop:
				targetAllocatorIntervalTicker.Stop()
				r.settings.Logger.Info("Stopping target allocator")
				return
			}
		}
	}()
	return nil
}

// syncTargetAllocator request jobs from targetAllocator and update underlying receiver, if the response does not match the provided compareHash.
// baseDiscoveryCfg can be used to provide additional ScrapeConfigs which will be added to the retrieved jobs.
func (r *pReceiver) syncTargetAllocator(compareHash uint64, allocConf *TargetAllocator, baseCfg *PromConfig) (uint64, error) {
	r.settings.Logger.Debug("Syncing target allocator jobs")
	scrapeConfigsResponse, err := r.getScrapeConfigsResponse(allocConf.Endpoint)
	if err != nil {
		r.settings.Logger.Error("Failed to retrieve job list", zap.Error(err))
		return 0, err
	}

	hash, err := hashstructure.Hash(scrapeConfigsResponse, hashstructure.FormatV2, nil)
	if err != nil {
		r.settings.Logger.Error("Failed to hash job list", zap.Error(err))
		return 0, err
	}
	if hash == compareHash {
		// no update needed
		return hash, nil
	}

	// Clear out the current configurations
	baseCfg.ScrapeConfigs = []*config.ScrapeConfig{}

	for jobName, scrapeConfig := range scrapeConfigsResponse {
		var httpSD promHTTP.SDConfig
		if allocConf.HTTPSDConfig == nil {
			httpSD = promHTTP.SDConfig{
				RefreshInterval: model.Duration(30 * time.Second),
			}
		} else {
			httpSD = promHTTP.SDConfig(*allocConf.HTTPSDConfig)
		}
		escapedJob := url.QueryEscape(jobName)
		httpSD.URL = fmt.Sprintf("%s/jobs/%s/targets?collector_id=%s", allocConf.Endpoint, escapedJob, allocConf.CollectorID)
		httpSD.HTTPClientConfig.FollowRedirects = false
		scrapeConfig.ServiceDiscoveryConfigs = discovery.Configs{
			&httpSD,
		}

		if allocConf.HTTPScrapeConfig != nil {
			scrapeConfig.HTTPClientConfig = commonconfig.HTTPClientConfig(*allocConf.HTTPScrapeConfig)
		}

		baseCfg.ScrapeConfigs = append(baseCfg.ScrapeConfigs, scrapeConfig)
	}

	err = r.applyCfg(baseCfg)
	if err != nil {
		r.settings.Logger.Error("Failed to apply new scrape configuration", zap.Error(err))
		return 0, err
	}

	return hash, nil
}

// instantiateShard inserts the SHARD environment variable in the returned configuration
func (r *pReceiver) instantiateShard(body []byte) []byte {
	shard, ok := os.LookupEnv("SHARD")
	if !ok {
		shard = "0"
	}
	return bytes.ReplaceAll(body, []byte("$(SHARD)"), []byte(shard))
}

func (r *pReceiver) getScrapeConfigsResponse(baseURL string) (map[string]*config.ScrapeConfig, error) {
	scrapeConfigsURL := fmt.Sprintf("%s/scrape_configs", baseURL)
	_, err := url.Parse(scrapeConfigsURL) // check if valid
	if err != nil {
		return nil, err
	}

	resp, err := r.httpClient.Get(scrapeConfigsURL)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	jobToScrapeConfig := map[string]*config.ScrapeConfig{}
	envReplacedBody := r.instantiateShard(body)
	err = yaml.Unmarshal(envReplacedBody, &jobToScrapeConfig)
	if err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return jobToScrapeConfig, nil
}

func (r *pReceiver) applyCfg(cfg *PromConfig) error {
	if !enableNativeHistogramsGate.IsEnabled() {
		// Enforce scraping classic histograms to avoid dropping them.
		for _, scrapeConfig := range cfg.ScrapeConfigs {
			scrapeConfig.ScrapeClassicHistograms = true
		}
	}

	if err := r.scrapeManager.ApplyConfig((*config.Config)(cfg)); err != nil {
		return err
	}

	discoveryCfg := make(map[string]discovery.Configs)
	for _, scrapeConfig := range cfg.ScrapeConfigs {
		discoveryCfg[scrapeConfig.JobName] = scrapeConfig.ServiceDiscoveryConfigs
		r.settings.Logger.Info("Scrape job added", zap.String("jobName", scrapeConfig.JobName))
	}
	return r.discoveryManager.ApplyConfig(discoveryCfg)
}

func (r *pReceiver) initPrometheusComponents(ctx context.Context, logger log.Logger) error {
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
		return fmt.Errorf("failed to create discovery manager")
	}

	go func() {
		r.settings.Logger.Info("Starting discovery manager")
		if err = r.discoveryManager.Run(); err != nil && !errors.Is(err, context.Canceled) {
			r.settings.Logger.Error("Discovery manager failed", zap.Error(err))
			r.settings.TelemetrySettings.ReportStatus(component.NewFatalErrorEvent(err))
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
	}

	// for testing only
	if r.skipOffsetting {
		optsValue := reflect.ValueOf(opts).Elem()
		field := optsValue.FieldByName("skipOffsetting")
		reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).
			Elem().
			Set(reflect.ValueOf(true))
	}

	scrapeManager, err := scrape.NewManager(opts, logger, store, r.registerer)
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
			r.settings.TelemetrySettings.ReportStatus(component.NewFatalErrorEvent(err))
		}
	}()

	return nil
}

func (r *pReceiver) initAPIServer(ctx context.Context, host component.Host) error {
	r.settings.Logger.Info("Starting Prometheus API server")

	o := &web.Options{
		ScrapeManager: r.scrapeManager,
		Context:       ctx,
		ListenAddress: r.cfg.APIServer.ServerConfig.Endpoint,
		ExternalURL: &url.URL{
			Scheme: "http",
			Host:   r.cfg.APIServer.ServerConfig.Endpoint,
			Path:   "",
		},
		RoutePrefix:    "/",
		ReadTimeout:    time.Minute * readTimeoutMinutes,
		PageTitle:      "Prometheus Receiver",
		Flags:          make(map[string]string),
		MaxConnections: maxConnections,
		IsAgent:        true,
		Registerer:     r.registry,
		Gatherer:       r.registry,
	}

	// Creates the API object in the same way as the Prometheus web package: https://github.com/prometheus/prometheus/blob/6150e1ca0ede508e56414363cc9062ef522db518/web/web.go#L314-L354
	// Anything not defined by the options above will be nil, such as o.QueryEngine, o.Storage, etc. IsAgent=true, so these being nil is expected by Prometheus.
	factorySPr := func(_ context.Context) api_v1.ScrapePoolsRetriever { return o.ScrapeManager }
	factoryTr := func(_ context.Context) api_v1.TargetRetriever { return o.ScrapeManager }
	factoryAr := func(_ context.Context) api_v1.AlertmanagerRetriever { return nil }
	FactoryRr := func(_ context.Context) api_v1.RulesRetriever { return nil }
	var app storage.Appendable
	logger := log.NewNopLogger()

	apiV1 := api_v1.NewAPI(o.QueryEngine, o.Storage, app, o.ExemplarStorage, factorySPr, factoryTr, factoryAr,
		func() config.Config {
			return *(*config.Config)(r.cfg.PrometheusConfig)
		},
		o.Flags,
		api_v1.GlobalURLOptions{
			ListenAddress: o.ListenAddress,
			Host:          o.ExternalURL.Host,
			Scheme:        o.ExternalURL.Scheme,
		},
		func(f http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				f(w, r)
			}
		},
		o.LocalStorage,
		o.TSDBDir,
		o.EnableAdminAPI,
		logger,
		FactoryRr,
		o.RemoteReadSampleLimit,
		o.RemoteReadConcurrencyLimit,
		o.RemoteReadBytesInFrame,
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
		nil,
		o.Gatherer,
		o.Registerer,
		nil,
		o.EnableRemoteWriteReceiver,
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
		level.Info(logger).Log("msg", "Router prefix", "prefix", o.RoutePrefix)
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
		toolkit_web.Serve(listener, r.apiServer, &toolkit_web.FlagConfig{WebConfigFile: &webconfig}, logger)
	}()

	return nil
}

func setPathWithPrefix(prefix string) func(handlerName string, handler http.HandlerFunc) http.HandlerFunc {
	return func(handlerName string, handler http.HandlerFunc) http.HandlerFunc {
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
	close(r.targetAllocatorStop)
	if r.unregisterMetrics != nil {
		r.unregisterMetrics()
	}
	if r.apiServer != nil {
		r.apiServer.Shutdown(ctx)
	}
	return nil
}
