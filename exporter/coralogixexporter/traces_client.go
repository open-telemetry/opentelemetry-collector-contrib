// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type tracesExporter struct {
	// Input configuration.
	config *Config

	traceExporter ptraceotlp.GRPCClient
	clientConn    *grpc.ClientConn
	callOptions   []grpc.CallOption

	settings component.TelemetrySettings

	// Default user-agent header.
	userAgent string
}

func newTracesExporter(cfg component.Config, set exporter.CreateSettings) (*tracesExporter, error) {
	oCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config exporter, expect type: %T, got: %T", &Config{}, cfg)
	}

	if isEmpty(oCfg.Domain) && isEmpty(oCfg.Traces.Endpoint) {
		return nil, errors.New("coralogix exporter config requires `domain` or `traces.endpoint` configuration")
	}
	userAgent := fmt.Sprintf("%s/%s (%s/%s)",
		set.BuildInfo.Description, set.BuildInfo.Version, runtime.GOOS, runtime.GOARCH)

	return &tracesExporter{config: oCfg, settings: set.TelemetrySettings, userAgent: userAgent}, nil
}

func (e *tracesExporter) start(ctx context.Context, host component.Host) (err error) {

	switch {
	case !isEmpty(e.config.Traces.Endpoint):
		if e.clientConn, err = e.config.Traces.ToClientConn(ctx, host, e.settings, grpc.WithUserAgent(e.userAgent)); err != nil {
			return err
		}
	case !isEmpty(e.config.Domain):
		if e.clientConn, err = e.config.getDomainGrpcSettings().ToClientConn(ctx, host, e.settings, grpc.WithUserAgent(e.userAgent)); err != nil {
			return err
		}
	}

	e.traceExporter = ptraceotlp.NewGRPCClient(e.clientConn)
	if e.config.Traces.Headers == nil {
		e.config.Traces.Headers = make(map[string]configopaque.String)
	}
	e.config.Traces.Headers["Authorization"] = configopaque.String("Bearer " + string(e.config.PrivateKey))

	e.callOptions = []grpc.CallOption{
		grpc.WaitForReady(e.config.Traces.WaitForReady),
	}

	return nil
}

func (e *tracesExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {

	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		resourceSpan := rss.At(i)
		appName, subsystem := e.config.getMetadataFromResource(resourceSpan.Resource())
		resourceSpan.Resource().Attributes().PutStr(cxAppNameAttrName, appName)
		resourceSpan.Resource().Attributes().PutStr(cxSubsystemNameAttrName, subsystem)

	}

	_, err := e.traceExporter.Export(e.enhanceContext(ctx), ptraceotlp.NewExportRequestFromTraces(td), e.callOptions...)
	if err != nil {
		return processError(err)
	}

	return nil
}
func (e *tracesExporter) shutdown(context.Context) error {
	if e.clientConn == nil {
		return nil
	}
	return e.clientConn.Close()
}

func (e *tracesExporter) enhanceContext(ctx context.Context) context.Context {
	md := metadata.New(nil)
	for k, v := range e.config.Traces.Headers {
		md.Set(k, string(v))
	}
	return metadata.NewOutgoingContext(ctx, md)
}
