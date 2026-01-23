// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer/consumererror"
	exp "go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	experimentalgrpc "google.golang.org/grpc/experimental"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type signalConfig interface {
	ToClientConn(ctx context.Context, host component.Host, settings component.TelemetrySettings, opts ...configgrpc.ToClientConnOption) (*grpc.ClientConn, error)
	ToHTTPClient(ctx context.Context, host component.Host, settings component.TelemetrySettings) (*http.Client, error)
	GetWaitForReady() bool
	GetEndpoint() string
	GetAcceptEncoding() string
}

var _ signalConfig = (*signalConfigWrapper)(nil)

type signalConfigWrapper struct {
	config *TransportConfig
}

func (w *signalConfigWrapper) ToClientConn(ctx context.Context, host component.Host, settings component.TelemetrySettings, opts ...configgrpc.ToClientConnOption) (*grpc.ClientConn, error) {
	return w.config.ToClientConn(ctx, host.GetExtensions(), settings, opts...)
}

func (w *signalConfigWrapper) ToHTTPClient(ctx context.Context, host component.Host, settings component.TelemetrySettings) (*http.Client, error) {
	return w.config.ToHTTPClient(ctx, host, settings)
}

func (w *signalConfigWrapper) GetWaitForReady() bool {
	return w.config.WaitForReady
}

func (w *signalConfigWrapper) GetEndpoint() string {
	return w.config.Endpoint
}

func (w *signalConfigWrapper) GetAcceptEncoding() string {
	return w.config.GetAcceptEncoding()
}

func newSignalExporter(oCfg *Config, set exp.Settings, signalEndpoint string, headers configopaque.MapList) (*signalExporter, error) {
	if isEmpty(oCfg.Domain) && isEmpty(signalEndpoint) {
		return nil, errors.New("coralogix exporter config requires `domain` or `logs.endpoint` configuration")
	}

	userAgent := fmt.Sprintf("%s/%s (%s/%s)",
		set.BuildInfo.Description, set.BuildInfo.Version, runtime.GOOS, runtime.GOARCH)

	md := metadata.New(nil)
	for k, v := range headers.Iter {
		md.Set(k, string(v))
	}

	return &signalExporter{
		config:    oCfg,
		settings:  set.TelemetrySettings,
		userAgent: userAgent,
		metadata:  md,
		rateError: rateError{
			enabled:   oCfg.RateLimiter.Enabled,
			threshold: oCfg.RateLimiter.Threshold,
			duration:  oCfg.RateLimiter.Duration,
		},
	}, nil
}

type signalExporter struct {
	// Input configuration.
	config *Config

	// GRPC exporter
	clientConn  *grpc.ClientConn
	callOptions []grpc.CallOption

	// HTTP exporter
	clientHTTP *http.Client

	settings component.TelemetrySettings

	// Default user-agent header.
	userAgent string

	// Cached metadata for outgoing context
	metadata metadata.MD

	rateError rateError
}

func (e *signalExporter) shutdown(_ context.Context) error {
	if e.clientConn != nil {
		return e.clientConn.Close()
	}

	if e.clientHTTP != nil {
		e.clientHTTP.CloseIdleConnections()
	}
	return nil
}

func (e *signalExporter) enhanceContext(ctx context.Context) context.Context {
	if len(e.metadata) == 0 {
		return ctx
	}

	return metadata.NewOutgoingContext(ctx, e.metadata)
}

func (e *signalExporter) startSignalExporter(ctx context.Context, host component.Host, signalConfig signalConfig) (err error) {
	if signalConfigWrapper, ok := signalConfig.(*signalConfigWrapper); ok {
		signalConfigWrapper.config.Headers.Set("Authorization", configopaque.String("Bearer "+string(e.config.PrivateKey)))
		if e.config.Protocol != httpProtocol {
			for k, v := range signalConfigWrapper.config.Headers.Iter {
				e.metadata.Set(k, string(v))
			}
		}
	}

	signalConfigWrapper, isWrapper := signalConfig.(*signalConfigWrapper)
	if !isWrapper {
		return errors.New("unexpected signal config type")
	}

	var transportConfig *TransportConfig
	if !isEmpty(e.config.Domain) && isEmpty(signalConfig.GetEndpoint()) {
		// TODO: Remove this function call, already done in Unmarshal, see github.com/open-telemetry/opentelemetry-collector-contrib/issues/44731
		transportConfig = setMergedTransportConfig(e.config, &e.config.DomainSettings, signalConfigWrapper.config)
	} else {
		transportConfig = signalConfigWrapper.config
	}

	if e.config.Protocol == httpProtocol {
		e.clientHTTP, err = transportConfig.ToHTTPClient(ctx, host, e.settings)
		if err != nil {
			return err
		}
	} else {
		if e.clientConn, err = transportConfig.ToClientConn(ctx, host.GetExtensions(), e.settings, configgrpc.WithGrpcDialOption(grpc.WithUserAgent(e.userAgent))); err != nil {
			return err
		}
		callOptions := []grpc.CallOption{
			grpc.WaitForReady(signalConfig.GetWaitForReady()),
		}
		// Only add AcceptCompressors if a compression encoding is specified
		if acceptEncoding := signalConfig.GetAcceptEncoding(); acceptEncoding != "" {
			callOptions = append(callOptions, experimentalgrpc.AcceptCompressors(acceptEncoding))
		}
		e.callOptions = callOptions
	}

	return nil
}

func (e *signalExporter) EnableRateLimit() {
	e.rateError.enableRateLimit()
}

func (e *signalExporter) canSend() bool {
	if !e.rateError.isRateLimited() {
		return true
	}

	if e.rateError.canDisableRateLimit() {
		e.rateError.disableRateLimit()
		return true
	}

	return false
}

// processError implements the common OTLP logic around request handling such as retries and throttling.
// Send a telemetry data request to the server. "perform" function is expected to make
// the actual gRPC unary call that sends the request.
func (e *signalExporter) processError(err error) error {
	switch e.config.Protocol {
	case grpcProtocol:
		st := status.Convert(err)
		if st.Code() == codes.OK {
			e.rateError.errorCount.Store(0)
			return nil
		}

		retryInfo := getRetryInfo(st)

		shouldRetry, shouldFlagRateLimit := shouldRetry(st.Code(), retryInfo)
		if !shouldRetry {
			if shouldFlagRateLimit {
				e.EnableRateLimit()
			}
			return consumererror.NewPermanent(err)
		}

		throttleDuration := getThrottleDuration(retryInfo)
		if throttleDuration != 0 {
			return exporterhelper.NewThrottleRetry(err, throttleDuration)
		}
	case httpProtocol:
		var httpErr *httpError
		if !errors.As(err, &httpErr) {
			return err
		}

		if httpErr.StatusCode == http.StatusOK {
			e.rateError.errorCount.Store(0)
			return nil
		}

		shouldRetry, shouldFlagRateLimit := shouldRetryHTTP(httpErr.StatusCode)
		if !shouldRetry {
			if shouldFlagRateLimit {
				e.EnableRateLimit()
			}
			return consumererror.NewPermanent(err)
		}

		throttleDuration := getHTTPThrottleDuration(httpErr.StatusCode, httpErr.Header)
		if throttleDuration != 0 {
			return exporterhelper.NewThrottleRetry(err, throttleDuration)
		}
	}
	return err
}
