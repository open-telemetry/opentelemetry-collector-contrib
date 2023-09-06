// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowexporter // import "github.com/open-telemetry/otel-arrow/collector/exporter/otelarrowexporter"

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	arrowPkg "github.com/apache/arrow/go/v12/arrow"
	arrowRecord "github.com/open-telemetry/otel-arrow/pkg/otel/arrow_record"
	"go.uber.org/multierr"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/open-telemetry/otel-arrow/collector/exporter/otelarrowexporter/internal/arrow"
	"github.com/open-telemetry/otel-arrow/collector/netstats"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

type baseExporter struct {
	// Input configuration.
	config *Config

	// gRPC clients and connection.
	traceExporter  ptraceotlp.GRPCClient
	metricExporter pmetricotlp.GRPCClient
	logExporter    plogotlp.GRPCClient
	clientConn     *grpc.ClientConn
	metadata       metadata.MD
	callOptions    []grpc.CallOption
	settings       exporter.CreateSettings
	netStats       *netstats.NetworkReporter

	// Default user-agent header.
	userAgent string

	// OTLP+Arrow optional state
	arrow *arrow.Exporter
	// streamClientFunc is the stream constructor, depends on EnableMixedTelemetry.
	streamClientFactory streamClientFactory
}

type streamClientFactory func(cfg *Config, conn *grpc.ClientConn) func(ctx context.Context, opts ...grpc.CallOption) (arrow.AnyStreamClient, error)

// Crete new exporter and start it. The exporter will begin connecting but
// this function may return before the connection is established.
func newExporter(cfg component.Config, set exporter.CreateSettings, streamClientFactory streamClientFactory) (*baseExporter, error) {
	oCfg := cfg.(*Config)

	if oCfg.Endpoint == "" {
		return nil, errors.New("OTLP exporter config requires an Endpoint")
	}

	netStats, err := netstats.NewExporterNetworkReporter(set)
	if err != nil {
		return nil, err
	}
	userAgent := fmt.Sprintf("%s/%s (%s/%s)",
		set.BuildInfo.Description, set.BuildInfo.Version, runtime.GOOS, runtime.GOARCH)

	if !oCfg.Arrow.Disabled {
		userAgent += fmt.Sprintf(" ApacheArrow/%s (NumStreams/%d)", arrowPkg.PkgVersion, oCfg.Arrow.NumStreams)
	}

	return &baseExporter{
		config:              oCfg,
		settings:            set,
		userAgent:           userAgent,
		netStats:            netStats,
		streamClientFactory: streamClientFactory,
	}, nil
}

// start actually creates the gRPC connection. The client construction is deferred till this point as this
// is the only place we get hold of Extensions which are required to construct auth round tripper.
func (e *baseExporter) start(ctx context.Context, host component.Host) (err error) {
	dialOpts := []grpc.DialOption{
		grpc.WithUserAgent(e.userAgent),
	}
	if e.netStats != nil {
		dialOpts = append(dialOpts, grpc.WithStatsHandler(e.netStats))
	}
	dialOpts = append(dialOpts, e.config.UserDialOptions...)
	if e.clientConn, err = e.config.GRPCClientSettings.ToClientConn(ctx, host, e.settings.TelemetrySettings, dialOpts...); err != nil {
		return err
	}
	e.traceExporter = ptraceotlp.NewGRPCClient(e.clientConn)
	e.metricExporter = pmetricotlp.NewGRPCClient(e.clientConn)
	e.logExporter = plogotlp.NewGRPCClient(e.clientConn)
	headers := map[string]string{}
	for k, v := range e.config.GRPCClientSettings.Headers {
		headers[k] = string(v)
	}
	e.metadata = metadata.New(headers)
	e.callOptions = []grpc.CallOption{
		grpc.WaitForReady(e.config.GRPCClientSettings.WaitForReady),
	}

	if !e.config.Arrow.Disabled {
		// Note this sets static outgoing context for all future stream requests.
		ctx := e.enhanceContext(context.Background())

		var perRPCCreds credentials.PerRPCCredentials
		if e.config.GRPCClientSettings.Auth != nil {
			// Get the auth extension, we'll use it to enrich the request context.
			authClient, err := e.config.GRPCClientSettings.Auth.GetClientAuthenticator(host.GetExtensions())
			if err != nil {
				return err
			}

			perRPCCreds, err = authClient.PerRPCCredentials()
			if err != nil {
				return err
			}
		}

		arrowOpts := e.config.Arrow.ToArrowProducerOptions()

		e.arrow = arrow.NewExporter(e.config.Arrow.MaxStreamLifetime, e.config.Arrow.NumStreams, e.config.Arrow.DisableDowngrade, e.settings.TelemetrySettings, e.callOptions, func() arrowRecord.ProducerAPI {
			return arrowRecord.NewProducerWithOptions(arrowOpts...)
		}, e.streamClientFactory(e.config, e.clientConn), perRPCCreds)

		if err := e.arrow.Start(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (e *baseExporter) shutdown(ctx context.Context) error {
	var err error
	if e.arrow != nil {
		err = multierr.Append(err, e.arrow.Shutdown(ctx))
	}
	if e.clientConn != nil {
		err = multierr.Append(err, e.clientConn.Close())
	}
	return err
}

// arrowSendAndWait gets an available stream and tries to send using
// Arrow if it is configured.  A (false, nil) result indicates for the
// caller to fall back to ordinary OTLP.
//
// Note that ctx is has not had enhanceContext() called, meaning it
// will have outgoing gRPC metadata only when an upstream processor or
// receiver placed it there.
func (e *baseExporter) arrowSendAndWait(ctx context.Context, data interface{}) (sent bool, _ error) {
	if e.arrow == nil {
		return false, nil
	}
	sent, err := e.arrow.SendAndWait(ctx, data)
	if err != nil {
		return sent, processError(err)
	}
	return sent, nil
}

func (e *baseExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {
	if sent, err := e.arrowSendAndWait(ctx, td); err != nil {
		return err
	} else if sent {
		return nil
	}
	req := ptraceotlp.NewExportRequestFromTraces(td)
	resp, respErr := e.traceExporter.Export(e.enhanceContext(ctx), req, e.callOptions...)
	if err := processError(respErr); err != nil {
		return err
	}
	partialSuccess := resp.PartialSuccess()
	if !(partialSuccess.ErrorMessage() == "" && partialSuccess.RejectedSpans() == 0) {
		return consumererror.NewPermanent(fmt.Errorf("OTLP partial success: \"%s\" (%d rejected)", resp.PartialSuccess().ErrorMessage(), resp.PartialSuccess().RejectedSpans()))
	}
	return nil
}

func (e *baseExporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	if sent, err := e.arrowSendAndWait(ctx, md); err != nil {
		return err
	} else if sent {
		return nil
	}
	req := pmetricotlp.NewExportRequestFromMetrics(md)
	resp, respErr := e.metricExporter.Export(e.enhanceContext(ctx), req, e.callOptions...)
	if err := processError(respErr); err != nil {
		return err
	}
	partialSuccess := resp.PartialSuccess()
	if !(partialSuccess.ErrorMessage() == "" && partialSuccess.RejectedDataPoints() == 0) {
		return consumererror.NewPermanent(fmt.Errorf("OTLP partial success: \"%s\" (%d rejected)", resp.PartialSuccess().ErrorMessage(), resp.PartialSuccess().RejectedDataPoints()))
	}
	return nil
}

func (e *baseExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	if sent, err := e.arrowSendAndWait(ctx, ld); err != nil {
		return err
	} else if sent {
		return nil
	}
	req := plogotlp.NewExportRequestFromLogs(ld)
	resp, respErr := e.logExporter.Export(e.enhanceContext(ctx), req, e.callOptions...)
	if err := processError(respErr); err != nil {
		return err
	}
	partialSuccess := resp.PartialSuccess()
	if !(partialSuccess.ErrorMessage() == "" && partialSuccess.RejectedLogRecords() == 0) {
		return consumererror.NewPermanent(fmt.Errorf("OTLP partial success: \"%s\" (%d rejected)", resp.PartialSuccess().ErrorMessage(), resp.PartialSuccess().RejectedLogRecords()))
	}
	return nil
}

func (e *baseExporter) enhanceContext(ctx context.Context) context.Context {
	if e.metadata.Len() > 0 {
		return metadata.NewOutgoingContext(ctx, e.metadata)
	}

	return ctx
}

func processError(err error) error {
	if err == nil {
		// Request is successful, we are done.
		return nil
	}

	// We have an error, check gRPC status code.
	st := status.Convert(err)
	if st.Code() == codes.OK {
		// Not really an error, still success.
		return nil
	}

	// Now, this is this a real error.

	retryInfo := getRetryInfo(st)

	if !shouldRetry(st.Code(), retryInfo) {
		// It is not a retryable error, we should not retry.
		return consumererror.NewPermanent(err)
	}

	// Check if server returned throttling information.
	throttleDuration := getThrottleDuration(retryInfo)
	if throttleDuration != 0 {
		// We are throttled. Wait before retrying as requested by the server.
		return exporterhelper.NewThrottleRetry(err, throttleDuration)
	}

	// Need to retry.

	return err
}

func shouldRetry(code codes.Code, retryInfo *errdetails.RetryInfo) bool {
	switch code {
	case codes.Canceled,
		codes.DeadlineExceeded,
		codes.Aborted,
		codes.OutOfRange,
		codes.Unavailable,
		codes.DataLoss:
		// These are retryable errors.
		return true
	case codes.ResourceExhausted:
		// Retry only if RetryInfo was supplied by the server.
		// This indicates that the server can still recover from resource exhaustion.
		return retryInfo != nil
	}
	// Don't retry on any other code.
	return false
}

func getRetryInfo(status *status.Status) *errdetails.RetryInfo {
	for _, detail := range status.Details() {
		if t, ok := detail.(*errdetails.RetryInfo); ok {
			return t
		}
	}
	return nil
}

func getThrottleDuration(t *errdetails.RetryInfo) time.Duration {
	if t == nil || t.RetryDelay == nil {
		return 0
	}
	if t.RetryDelay.Seconds > 0 || t.RetryDelay.Nanos > 0 {
		return time.Duration(t.RetryDelay.Seconds)*time.Second + time.Duration(t.RetryDelay.Nanos)*time.Nanosecond
	}
	return 0
}
