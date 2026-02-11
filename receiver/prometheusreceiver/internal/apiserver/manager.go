// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apiserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/apiserver"

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/mwitkow/go-conntrack"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/route"
	"github.com/prometheus/common/version"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/util/httputil"
	"github.com/prometheus/prometheus/web"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"
	"golang.org/x/net/netutil"

	grafanaRegexp "github.com/grafana/regexp"
	toolkit_web "github.com/prometheus/exporter-toolkit/web"
	api_v1 "github.com/prometheus/prometheus/web/api/v1"
)

// Use same settings as Prometheus web server
const (
	maxConnections     = 512
	readTimeoutMinutes = 10
)

type Manager struct {
	settings      receiver.Settings
	shutdown      chan struct{}
	cfg           *Config
	promCfg       *promconfig.Config
	scrapeManager *scrape.Manager
	registry      *prometheus.Registry
	registerer    prometheus.Registerer
	server        *http.Server
	cfgLock       *sync.RWMutex
}

func NewManager(set receiver.Settings, cfg *Config, promCfg *promconfig.Config, registry *prometheus.Registry, registerer prometheus.Registerer, cfgLock *sync.RWMutex) *Manager {
	if cfg == nil {
		return nil
	}
	if cfgLock == nil {
		cfgLock = &sync.RWMutex{}
	}
	return &Manager{
		shutdown:   make(chan struct{}),
		settings:   set,
		cfg:        cfg,
		promCfg:    promCfg,
		registry:   registry,
		registerer: registerer,
		cfgLock:    cfgLock,
	}
}

func (m *Manager) Start(ctx context.Context, host component.Host, scrapeManager *scrape.Manager) error {
	if m.cfg == nil {
		return nil
	}

	m.settings.Logger.Info("Starting Prometheus API server")
	m.scrapeManager = scrapeManager

	// If allowed CORS origins are provided in the receiver config, combine them into a single regex since the Prometheus API server requires this format.
	var corsOriginRegexp *grafanaRegexp.Regexp
	corsConfig := m.cfg.ServerConfig.CORS.Get()
	if corsConfig != nil && len(corsConfig.AllowedOrigins) > 0 {
		var combinedOriginsBuilder strings.Builder
		combinedOriginsBuilder.WriteString(corsConfig.AllowedOrigins[0])
		for _, origin := range corsConfig.AllowedOrigins[1:] {
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
	readTimeout := m.cfg.ServerConfig.ReadTimeout
	if readTimeout == 0 {
		readTimeout = time.Duration(readTimeoutMinutes) * time.Minute
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
		ReadTimeout:    readTimeout,
		PageTitle:      "Prometheus Receiver",
		Flags:          make(map[string]string),
		MaxConnections: maxConnections,
		IsAgent:        true,
		Registerer:     m.registerer,
		Gatherer:       m.registry,
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

	apiV1 := api_v1.NewAPI(
		o.QueryEngine,
		o.Storage,
		app,
		o.ExemplarStorage,
		factorySPr,
		factoryTr,
		factoryAr,
		// This ensures that any changes to the config made, even by the target allocator, are reflected in the API.
		func() promconfig.Config {
			m.cfgLock.RLock()
			defer m.cfgLock.RUnlock()
			return *m.promCfg
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
		o.ConvertOTLPDelta,
		o.NativeOTLPDeltaIngestion,
		o.STZeroIngestionEnabled,
		5*time.Minute, // LookbackDelta - Using the default value of 5 minutes
		o.EnableTypeAndUnitLabels,
		o.AppendMetadata,
		nil,
		o.FeatureRegistry,
	)

	// Create listener and monitor with conntrack in the same way as the Prometheus web package: https://github.com/prometheus/prometheus/blob/6150e1ca0ede508e56414363cc9062ef522db518/web/web.go#L564-L579
	listener, err := m.cfg.ServerConfig.ToListener(ctx)
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
	m.server, err = m.cfg.ServerConfig.ToServer(ctx, host.GetExtensions(), m.settings.TelemetrySettings, otelhttp.NewHandler(mux, "", spanNameFormatter))
	if err != nil {
		return err
	}
	webconfig := ""

	go func() {
		if err := toolkit_web.Serve(listener, m.server, &toolkit_web.FlagConfig{WebConfigFile: &webconfig}, logger); err != nil {
			m.settings.Logger.Error("API server failed", zap.Error(err))
		}
	}()

	return nil
}

// ApplyConfig updates the config field of the Manager struct.
func (m *Manager) ApplyConfig(cfg *promconfig.Config) error {
	m.cfgLock.Lock()
	defer m.cfgLock.Unlock()

	m.promCfg = cfg

	return nil
}

func (m *Manager) GetConfig() *promconfig.Config {
	m.cfgLock.RLock()
	defer m.cfgLock.RUnlock()

	return m.promCfg
}

func (m *Manager) Shutdown(ctx context.Context) error {
	close(m.shutdown)
	if m.server != nil {
		return m.server.Shutdown(ctx)
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
