// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apiserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/apiserver"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/grafana/regexp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/route"
	"github.com/prometheus/common/version"
	toolkit_web "github.com/prometheus/exporter-toolkit/web"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/util/httputil"
	"github.com/prometheus/prometheus/web"
	api_v1 "github.com/prometheus/prometheus/web/api/v1"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"
	"golang.org/x/net/netutil"
	"golang.org/x/sync/errgroup"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/sharedpromconfig"
)

// defaultLookbackDelta is the Prometheus default. PromQL queries are not
// supported by this receiver so the value has no practical effect; it is
// only passed to satisfy the API v1 constructor signature.
const defaultLookbackDelta = 5 * time.Minute

type Manager struct {
	settings      receiver.Settings
	shutdown      chan struct{}
	shutdownOnce  sync.Once
	cfg           *Config
	promCfg       *sharedpromconfig.Config
	scrapeManager *scrape.Manager
	registry      *prometheus.Registry
	registerer    prometheus.Registerer
	server        *http.Server
	serverGroup   *errgroup.Group
}

func NewManager(set receiver.Settings, cfg *Config, promCfg *sharedpromconfig.Config, registry *prometheus.Registry, registerer prometheus.Registerer) *Manager {
	return &Manager{
		shutdown:   make(chan struct{}),
		settings:   set,
		cfg:        cfg,
		promCfg:    promCfg,
		registry:   registry,
		registerer: registerer,
	}
}

func (m *Manager) Start(ctx context.Context, host component.Host, scrapeManager *scrape.Manager) error {
	m.settings.Logger.Info("Starting Prometheus API server")
	m.scrapeManager = scrapeManager

	// If allowed CORS origins are provided in the receiver config, combine them into a single regex since the Prometheus API server requires this format.
	var corsOriginRegexp *regexp.Regexp
	corsConfig := m.cfg.ServerConfig.CORS.Get()
	if corsConfig != nil && len(corsConfig.AllowedOrigins) > 0 {
		combinedRegexp, err := regexp.Compile(strings.Join(corsConfig.AllowedOrigins, "|"))
		if err != nil {
			return fmt.Errorf("failed to compile combined CORS allowed origins into regex: %w", err)
		}
		corsOriginRegexp = combinedRegexp
	}

	// Set the options to keep similar code to the Prometheus repo.
	o := &web.Options{
		ScrapeManager:   m.scrapeManager,
		Context:         ctx,
		ListenAddresses: []string{m.cfg.ServerConfig.NetAddr.Endpoint},
		ExternalURL: &url.URL{
			Scheme: "http",
			Host:   m.cfg.ServerConfig.NetAddr.Endpoint,
			Path:   "",
		},
		RoutePrefix:    "/",
		ReadTimeout:    m.cfg.ServerConfig.ReadTimeout,
		PageTitle:      "Prometheus Receiver",
		MaxConnections: m.cfg.MaxConnections,
		IsAgent:        true,
		Registerer:     m.registerer,
		Gatherer:       m.registry,
		CORSOrigin:     corsOriginRegexp,
		Flags:          map[string]string{},
	}

	// Creates the API object in the same way as the Prometheus web package: https://github.com/prometheus/prometheus/blob/6150e1ca0ede508e56414363cc9062ef522db518/web/web.go#L314-L354
	// Anything not defined by the options above will be nil, such as o.QueryEngine, o.Storage, etc. IsAgent=true, so these being nil is expected by Prometheus.
	factorySPr := func(_ context.Context) api_v1.ScrapePoolsRetriever { return o.ScrapeManager }
	factoryTr := func(_ context.Context) api_v1.TargetRetriever { return o.ScrapeManager }
	factoryAr := func(_ context.Context) api_v1.AlertmanagerRetriever { return nil }
	factoryRr := func(_ context.Context) api_v1.RulesRetriever { return nil }
	var app storage.Appendable
	var appV2 storage.AppendableV2
	logger := promslog.NewNopLogger()

	apiV1 := api_v1.NewAPI(
		o.QueryEngine,
		o.Storage,
		app,
		appV2,
		o.ExemplarStorage,
		factorySPr,
		factoryTr,
		factoryAr,
		// This ensures that any changes to the config made, even by the target allocator, are reflected in the API.
		m.promCfg.Get,
		o.Flags,
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
		&api_v1.PrometheusVersion{
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
		nil, // StatsRenderer
		o.EnableRemoteWriteReceiver,
		o.AcceptRemoteWriteProtoMsgs,
		o.EnableOTLPWriteReceiver,
		o.ConvertOTLPDelta,
		o.NativeOTLPDeltaIngestion,
		o.STZeroIngestionEnabled,
		defaultLookbackDelta,
		o.EnableTypeAndUnitLabels,
		false, // appendMetadata from remote write
		nil,   // OverrideErrorCode
		nil,   // FeatureRegistry
		api_v1.OpenAPIOptions{},
		parser.NewParser(parser.Options{}),
	)

	// Create listener in the same way as the Prometheus web package: https://github.com/prometheus/prometheus/blob/6150e1ca0ede508e56414363cc9062ef522db518/web/web.go#L564-L579
	listener, err := m.cfg.ServerConfig.ToListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	listener = netutil.LimitListener(listener, o.MaxConnections)

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
	m.server, err = m.cfg.ServerConfig.ToServer(ctx, host.GetExtensions(), m.settings.TelemetrySettings, otelhttp.NewHandler(mux, "", spanNameFormatter))
	if err != nil {
		return err
	}
	webconfig := ""

	m.serverGroup = &errgroup.Group{}
	m.serverGroup.Go(func() error {
		if err := toolkit_web.Serve(listener, m.server, &toolkit_web.FlagConfig{WebConfigFile: &webconfig}, logger); err != nil && !errors.Is(err, http.ErrServerClosed) {
			m.settings.Logger.Error("API server failed", zap.Error(err))
			return err
		}
		return nil
	})

	return nil
}

func (m *Manager) Shutdown(ctx context.Context) error {
	m.shutdownOnce.Do(func() {
		close(m.shutdown)
	})

	if m.server == nil {
		return nil
	}

	if err := m.server.Shutdown(ctx); err != nil {
		return err
	}

	if m.serverGroup != nil {
		errCh := make(chan error, 1)
		go func() { errCh <- m.serverGroup.Wait() }()
		select {
		case err := <-errCh:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}

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
