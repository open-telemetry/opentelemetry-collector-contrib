// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	exp "go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func newLogsExporter(cfg component.Config, set exp.CreateSettings) (*logsExporter, error) {
	oCfg := cfg.(*Config)

	if isEmpty(oCfg.Domain) && isEmpty(oCfg.Logs.Endpoint) {
		return nil, errors.New("coralogix exporter config requires `domain` or `logs.endpoint` configuration")
	}
	userAgent := fmt.Sprintf("%s/%s (%s/%s)",
		set.BuildInfo.Description, set.BuildInfo.Version, runtime.GOOS, runtime.GOARCH)

	return &logsExporter{config: oCfg, settings: set.TelemetrySettings, userAgent: userAgent}, nil
}

type logsExporter struct {
	// Input configuration.
	config *Config

	logExporter plogotlp.GRPCClient
	clientConn  *grpc.ClientConn
	callOptions []grpc.CallOption

	settings component.TelemetrySettings

	// Default user-agent header.
	userAgent string
}

func (e *logsExporter) start(ctx context.Context, host component.Host) (err error) {
	switch {
	case !isEmpty(e.config.Logs.Endpoint):
		if e.clientConn, err = e.config.Logs.ToClientConn(ctx, host, e.settings, grpc.WithUserAgent(e.userAgent)); err != nil {
			return err
		}
	case !isEmpty(e.config.Domain):

		if e.clientConn, err = e.config.getDomainGrpcSettings().ToClientConn(ctx, host, e.settings, grpc.WithUserAgent(e.userAgent)); err != nil {
			return err
		}
	}

	e.logExporter = plogotlp.NewGRPCClient(e.clientConn)
	if e.config.Logs.Headers == nil {
		e.config.Logs.Headers = make(map[string]configopaque.String)
	}
	e.config.Logs.Headers["Authorization"] = configopaque.String("Bearer " + string(e.config.PrivateKey))

	e.callOptions = []grpc.CallOption{
		grpc.WaitForReady(e.config.Logs.WaitForReady),
	}

	return
}

func (e *logsExporter) shutdown(context.Context) error {
	if e.clientConn == nil {
		return nil
	}
	return e.clientConn.Close()
}

func (e *logsExporter) pushLogs(ctx context.Context, ld plog.Logs) error {

	rss := ld.ResourceLogs()
	for i := 0; i < rss.Len(); i++ {
		resourceLog := rss.At(i)
		appName, subsystem := e.config.getMetadataFromResource(resourceLog.Resource())
		resourceLog.Resource().Attributes().PutStr(cxAppNameAttrName, appName)
		resourceLog.Resource().Attributes().PutStr(cxSubsystemNameAttrName, subsystem)
	}

	_, err := e.logExporter.Export(e.enhanceContext(ctx), plogotlp.NewExportRequestFromLogs(ld), e.callOptions...)
	if err != nil {
		return processError(err)
	}

	return nil
}

func (e *logsExporter) enhanceContext(ctx context.Context) context.Context {
	md := metadata.New(nil)
	for k, v := range e.config.Logs.Headers {
		md.Set(k, string(v))
	}
	return metadata.NewOutgoingContext(ctx, md)
}
