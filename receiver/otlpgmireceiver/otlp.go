// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpgmireceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpgmireceiver"

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	// "go.opentelemetry.io/collector/internal/telemetry" // 内部包，不能使用
	// "go.opentelemetry.io/collector/internal/telemetry/componentattribute" // 内部包，不能使用
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/pprofile/pprofileotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.opentelemetry.io/collector/receiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpgmireceiver/internal/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpgmireceiver/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpgmireceiver/internal/profiles"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpgmireceiver/internal/trace"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

// otlpgmiReceiver is the type that exposes Trace and Metrics reception.
type otlpgmiReceiver struct {
	cfg        *Config
	serverGRPC *grpc.Server
	serverHTTP *http.Server

	nextTraces   consumer.Traces
	nextMetrics  consumer.Metrics
	nextLogs     consumer.Logs
	nextProfiles xconsumer.Profiles
	shutdownWG   sync.WaitGroup

	obsrepGRPC *receiverhelper.ObsReport
	obsrepHTTP *receiverhelper.ObsReport

	settings *receiver.Settings
	licenseValidator *LicenseValidator
}

// newOtlpgmiReceiver just creates the OpenTelemetry receiver services. It is the caller's
// responsibility to invoke the respective Start*Reception methods as well
// as the various Stop*Reception methods to end it.
func newOtlpgmiReceiver(cfg *Config, set *receiver.Settings) (*otlpgmiReceiver, error) {
	// set.TelemetrySettings = telemetry.WithoutAttributes(set.TelemetrySettings, componentattribute.SignalKey) // 内部包，暂时注释
	set.Logger.Debug("created signal-agnostic logger")
	r := &otlpgmiReceiver{
		cfg:          cfg,
		nextTraces:   nil,
		nextMetrics:  nil,
		nextLogs:     nil,
		nextProfiles: nil,
		settings:     set,
		licenseValidator: nil, // 将在 start 方法中根据配置初始化
	}

	var err error
	r.obsrepGRPC, err = receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		Transport:              "grpc",
		ReceiverCreateSettings: *set,
	})
	if err != nil {
		return nil, err
	}
	r.obsrepHTTP, err = receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		Transport:              "http",
		ReceiverCreateSettings: *set,
	})
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (r *otlpgmiReceiver) startGRPCServer(host component.Host) error {
	// If GRPC is not enabled, nothing to start.
	if !r.cfg.GRPC.HasValue() {
		return nil
	}

	grpcCfg := r.cfg.GRPC.Get()
	var err error
	if r.serverGRPC, err = grpcCfg.ToServer(context.Background(), host, r.settings.TelemetrySettings); err != nil {
		return err
	}

	if r.nextTraces != nil {
		ptraceotlp.RegisterGRPCServer(r.serverGRPC, trace.New(r.nextTraces, r.obsrepGRPC))
	}

	if r.nextMetrics != nil {
		pmetricotlp.RegisterGRPCServer(r.serverGRPC, metrics.New(r.nextMetrics, r.obsrepGRPC))
	}

	if r.nextLogs != nil {
		plogotlp.RegisterGRPCServer(r.serverGRPC, logs.New(r.nextLogs, r.obsrepGRPC))
	}

	if r.nextProfiles != nil {
		pprofileotlp.RegisterGRPCServer(r.serverGRPC, profiles.New(r.nextProfiles))
	}

	r.settings.Logger.Info("Starting GRPC server", zap.String("endpoint", grpcCfg.NetAddr.Endpoint))
	var gln net.Listener
	if gln, err = grpcCfg.NetAddr.Listen(context.Background()); err != nil {
		return err
	}

	r.shutdownWG.Add(1)
	go func() {
		defer r.shutdownWG.Done()

		if errGrpc := r.serverGRPC.Serve(gln); errGrpc != nil && !errors.Is(errGrpc, grpc.ErrServerStopped) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(errGrpc))
		}
	}()
	return nil
}

func (r *otlpgmiReceiver) startHTTPServer(ctx context.Context, host component.Host) error {
	// If HTTP is not enabled, nothing to start.
	if !r.cfg.HTTP.HasValue() {
		return nil
	}

	// Initialize license validator if license configuration is provided
	if r.cfg.License.HasValue() {
		licenseCfg := r.cfg.License.Get()
		r.licenseValidator = NewLicenseValidator(licenseCfg.ValidationURL, licenseCfg.CacheTimeout)
		r.settings.Logger.Info("License validation enabled", 
			zap.String("validation_url", licenseCfg.ValidationURL),
			zap.Int("cache_timeout", licenseCfg.CacheTimeout))
	} else {
		r.settings.Logger.Info("License validation disabled")
	}

	httpCfg := r.cfg.HTTP.Get()
	httpMux := http.NewServeMux()
	
	// Use dynamic routing for license key support
	if r.nextTraces != nil {
		httpTracesReceiver := trace.New(r.nextTraces, r.obsrepHTTP)
		httpMux.HandleFunc("/", func(resp http.ResponseWriter, req *http.Request) {
			// Check if this is a traces request with license key
			if strings.HasSuffix(req.URL.Path, "/v1/traces") && strings.Count(req.URL.Path, "/") == 3 {
				handleTraces(resp, req, httpTracesReceiver, r.licenseValidator)
				return
			}
			// Check if this is a metrics request with license key
			if strings.HasSuffix(req.URL.Path, "/v1/metrics") && strings.Count(req.URL.Path, "/") == 3 {
				if r.nextMetrics != nil {
					httpMetricsReceiver := metrics.New(r.nextMetrics, r.obsrepHTTP)
					handleMetrics(resp, req, httpMetricsReceiver, r.licenseValidator)
				}
				return
			}
			// Check if this is a logs request with license key
			if strings.HasSuffix(req.URL.Path, "/v1/logs") && strings.Count(req.URL.Path, "/") == 3 {
				if r.nextLogs != nil {
					httpLogsReceiver := logs.New(r.nextLogs, r.obsrepHTTP)
					handleLogs(resp, req, httpLogsReceiver, r.licenseValidator)
				}
				return
			}
			// Check if this is a profiles request with license key
			if strings.HasSuffix(req.URL.Path, "/v1/profiles") && strings.Count(req.URL.Path, "/") == 3 {
				if r.nextProfiles != nil {
					httpProfilesReceiver := profiles.New(r.nextProfiles)
					handleProfiles(resp, req, httpProfilesReceiver, r.licenseValidator)
				}
				return
			}
			// No matching route found
			http.NotFound(resp, req)
		})
	}

	var err error
	if r.serverHTTP, err = httpCfg.ServerConfig.ToServer(ctx, host, r.settings.TelemetrySettings, httpMux, confighttp.WithErrorHandler(errorHandler)); err != nil {
		return err
	}

	r.settings.Logger.Info("Starting HTTP server", zap.String("endpoint", httpCfg.ServerConfig.Endpoint))
	var hln net.Listener
	if hln, err = httpCfg.ServerConfig.ToListener(ctx); err != nil {
		return err
	}

	r.shutdownWG.Add(1)
	go func() {
		defer r.shutdownWG.Done()

		if errHTTP := r.serverHTTP.Serve(hln); errHTTP != nil && !errors.Is(errHTTP, http.ErrServerClosed) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(errHTTP))
		}
	}()
	return nil
}

// Start runs the trace receiver on the gRPC server. Currently
// it also enables the metrics receiver too.
func (r *otlpgmiReceiver) Start(ctx context.Context, host component.Host) error {
	if err := r.startGRPCServer(host); err != nil {
		return err
	}
	if err := r.startHTTPServer(ctx, host); err != nil {
		// It's possible that a valid GRPC server configuration was specified,
		// but an invalid HTTP configuration. If that's the case, the successfully
		// started GRPC server must be shutdown to ensure no goroutines are leaked.
		return errors.Join(err, r.Shutdown(ctx))
	}

	return nil
}

// Shutdown is a method to turn off receiving.
func (r *otlpgmiReceiver) Shutdown(ctx context.Context) error {
	var err error

	if r.serverHTTP != nil {
		err = r.serverHTTP.Shutdown(ctx)
	}

	if r.serverGRPC != nil {
		r.serverGRPC.GracefulStop()
	}

	r.shutdownWG.Wait()
	return err
}

func (r *otlpgmiReceiver) registerTraceConsumer(tc consumer.Traces) {
	r.nextTraces = tc
}

func (r *otlpgmiReceiver) registerMetricsConsumer(mc consumer.Metrics) {
	r.nextMetrics = mc
}

func (r *otlpgmiReceiver) registerLogsConsumer(lc consumer.Logs) {
	r.nextLogs = lc
}

func (r *otlpgmiReceiver) registerProfilesConsumer(tc xconsumer.Profiles) {
	r.nextProfiles = tc
}
