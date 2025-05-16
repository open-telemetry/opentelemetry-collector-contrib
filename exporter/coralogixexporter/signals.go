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
	exp "go.opentelemetry.io/collector/exporter"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
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
