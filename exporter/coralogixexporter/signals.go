// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer/consumererror"
	exp "go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// signalConfigWrapper wraps configgrpc.ClientConfig to implement signalConfig interface
type signalConfigWrapper struct {
	config *configgrpc.ClientConfig
}

func (w *signalConfigWrapper) ToClientConn(ctx context.Context, host component.Host, settings component.TelemetrySettings, opts ...configgrpc.ToClientConnOption) (*grpc.ClientConn, error) {
	return w.config.ToClientConn(ctx, host, settings, opts...)
}

func (w *signalConfigWrapper) GetWaitForReady() bool {
	return w.config.WaitForReady
}

func (w *signalConfigWrapper) GetEndpoint() string {
	return w.config.Endpoint
}

func newSignalExporter(oCfg *Config, set exp.Settings, signalEndpoint string, headers map[string]configopaque.String) (*signalExporter, error) {
	if isEmpty(oCfg.Domain) && isEmpty(signalEndpoint) {
		return nil, errors.New("coralogix exporter config requires `domain` or `logs.endpoint` configuration")
	}

	userAgent := fmt.Sprintf("%s/%s (%s/%s)",
		set.BuildInfo.Description, set.BuildInfo.Version, runtime.GOOS, runtime.GOARCH)

	md := metadata.New(nil)
	for k, v := range headers {
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

type signalConfig interface {
	ToClientConn(ctx context.Context, host component.Host, settings component.TelemetrySettings, opts ...configgrpc.ToClientConnOption) (*grpc.ClientConn, error)
	GetWaitForReady() bool
	GetEndpoint() string
}

type signalExporter struct {
	// Input configuration.
	config *Config

	clientConn  *grpc.ClientConn
	callOptions []grpc.CallOption

	settings component.TelemetrySettings

	// Default user-agent header.
	userAgent string

	// Cached metadata for outgoing context
	metadata metadata.MD

	rateError rateError
}

func (e *signalExporter) shutdown(_ context.Context) error {
	if e.clientConn == nil {
		return nil
	}
	return e.clientConn.Close()
}

func (e *signalExporter) enhanceContext(ctx context.Context) context.Context {
	if len(e.metadata) == 0 {
		return ctx
	}

	return metadata.NewOutgoingContext(ctx, e.metadata)
}

func (e *signalExporter) startSignalExporter(ctx context.Context, host component.Host, signalConfig signalConfig) (err error) {
	switch {
	case !isEmpty(signalConfig.GetEndpoint()):
		if e.clientConn, err = signalConfig.ToClientConn(ctx, host, e.settings, configgrpc.WithGrpcDialOption(grpc.WithUserAgent(e.userAgent))); err != nil {
			return err
		}
	case !isEmpty(e.config.Domain):
		if e.clientConn, err = e.config.getDomainGrpcSettings().ToClientConn(ctx, host, e.settings, configgrpc.WithGrpcDialOption(grpc.WithUserAgent(e.userAgent))); err != nil {
			return err
		}
	}

	if signalConfigWrapper, ok := signalConfig.(*signalConfigWrapper); ok {
		if signalConfigWrapper.config.Headers == nil {
			signalConfigWrapper.config.Headers = make(map[string]configopaque.String)
		}
		signalConfigWrapper.config.Headers["Authorization"] = configopaque.String("Bearer " + string(e.config.PrivateKey))
	}

	e.callOptions = []grpc.CallOption{
		grpc.WaitForReady(signalConfig.GetWaitForReady()),
	}

	return nil
}

func (e *signalExporter) EnableRateLimit(err error) {
	e.rateError.enableRateLimit(consumererror.NewPermanent(err))
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
	if err == nil {
		return nil
	}

	st := status.Convert(err)
	if st.Code() == codes.OK {
		return nil
	}

	retryInfo := getRetryInfo(st)

	shouldRetry, shouldFlagRateLimit := shouldRetry(st.Code(), retryInfo)
	if !shouldRetry {
		if shouldFlagRateLimit {
			e.EnableRateLimit(err)
		}
		return consumererror.NewPermanent(err)
	}

	throttleDuration := getThrottleDuration(retryInfo)
	if throttleDuration != 0 {
		return exporterhelper.NewThrottleRetry(err, throttleDuration)
	}

	return err
}
