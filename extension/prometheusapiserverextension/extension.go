// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusapiserverextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/prometheusapiserverextension"

import (
	"context"
	"fmt"
	stdlog "log"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/log/level"
	"github.com/mwitkow/go-conntrack"
	"golang.org/x/net/netutil"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/route"
	"github.com/prometheus/common/version"
	toolkit_web "github.com/prometheus/exporter-toolkit/web"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/util/httputil"
	"github.com/prometheus/prometheus/web"
	api_v1 "github.com/prometheus/prometheus/web/api/v1"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type prometheusAPIServerExtension struct {
	config              *Config
	settings            extension.CreateSettings
	ctx                 context.Context
	httpServer          *http.Server
	prometheusReceivers map[string]*prometheusReceiver
}

type prometheusReceiver struct {
	name             string
	port             uint64
	prometheusConfig *config.Config
	scrapeManager    *scrape.Manager
	registerer       prometheus.Registerer
}

// Use same settings as Prometheus web server
const (
	defaultPort        = 9090
	maxConnections     = 512
	readTimeoutMinutes = 10
)

func (e *prometheusAPIServerExtension) Start(_ context.Context, host component.Host) error {
	e.ctx = context.Background()

	return nil
}

func (e *prometheusAPIServerExtension) RegisterPrometheusReceiverComponents(receiverName string, port uint64,
	prometheusConfig *config.Config, scrapeManager *scrape.Manager, registerer prometheus.Registerer) error {

	prometheusReceiver := &prometheusReceiver{
		name:             receiverName,
		port:             port,
		prometheusConfig: prometheusConfig,
		scrapeManager:    scrapeManager,
		registerer:       registerer,
	}
	e.prometheusReceivers[receiverName] = prometheusReceiver

	o := &web.Options{
		ScrapeManager: prometheusReceiver.scrapeManager,
		Context:       e.ctx,
		ListenAddress: fmt.Sprintf(":%d", prometheusReceiver.port),
		ExternalURL: &url.URL{
			Scheme: "http",
			Host:   fmt.Sprintf("localhost:%d", prometheusReceiver.port),
			Path:   "",
		},
		RoutePrefix: "/",
		ReadTimeout: time.Minute * readTimeoutMinutes,
		PageTitle:   "Prometheus Receiver",
		Version: &web.PrometheusVersion{
			Version:   version.Version,
			Revision:  version.Revision,
			Branch:    version.Branch,
			BuildUser: version.BuildUser,
			BuildDate: version.BuildDate,
			GoVersion: version.GoVersion,
		},
		Flags:          make(map[string]string),
		MaxConnections: maxConnections,
		IsAgent:        true,
		Gatherer:       prometheus.DefaultGatherer,
		Registerer:     prometheusReceiver.registerer,
	}

	// Creates the API object in the same way as the Prometheus web package: https://github.com/prometheus/prometheus/blob/6150e1ca0ede508e56414363cc9062ef522db518/web/web.go#L314-L354
	// Anything not defined by the options above will be nil, such as o.QueryEngine, o.Storage, etc. IsAgent=true, so these being nil is expected by Prometheus.
	factorySPr := func(_ context.Context) api_v1.ScrapePoolsRetriever { return prometheusReceiver.scrapeManager }
	factoryTr := func(_ context.Context) api_v1.TargetRetriever { return prometheusReceiver.scrapeManager }
	factoryAr := func(_ context.Context) api_v1.AlertmanagerRetriever { return nil }
	FactoryRr := func(_ context.Context) api_v1.RulesRetriever { return nil }
	var app storage.Appendable
	logger := log.NewNopLogger()

	apiV1 := api_v1.NewAPI(o.QueryEngine, o.Storage, app, o.ExemplarStorage, factorySPr, factoryTr, factoryAr,
		func() config.Config {
			return *prometheusReceiver.prometheusConfig
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
	listener, err := net.Listen("tcp", o.ListenAddress)
	if err != nil {
		return err
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

	errlog := stdlog.New(log.NewStdlibAdapter(level.Error(logger)), "", 0)
	spanNameFormatter := otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
		return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
	})
	e.httpServer = &http.Server{
		Handler:     otelhttp.NewHandler(mux, "", spanNameFormatter),
		ErrorLog:    errlog,
		ReadTimeout: o.ReadTimeout,
	}
	webconfig := ""

	go func() {
		toolkit_web.Serve(listener, e.httpServer, &toolkit_web.FlagConfig{WebConfigFile: &webconfig}, logger)
	}()

	return nil
}

func (e *prometheusAPIServerExtension) UpdatePrometheusConfig(receiverName string, prometheusConfig *config.Config) {
	e.prometheusReceivers[receiverName].prometheusConfig = prometheusConfig
}

func (e *prometheusAPIServerExtension) Shutdown(ctx context.Context) error {
	e.httpServer.Shutdown(ctx)

	fmt.Println("shutting down prometheusAPIServerExtension")
	return nil
}

func setPathWithPrefix(prefix string) func(handlerName string, handler http.HandlerFunc) http.HandlerFunc {
	return func(handlerName string, handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			handler(w, r.WithContext(httputil.ContextWithPath(r.Context(), prefix+r.URL.Path)))
		}
	}
}
