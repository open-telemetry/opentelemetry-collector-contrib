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
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func newLogsExporter(cfg config.Exporter, set component.ExporterCreateSettings) (*logsExporter, error) {
	oCfg := cfg.(*Config)

	if oCfg.Logs.Endpoint == "" || oCfg.Logs.Endpoint == "https://" || oCfg.Logs.Endpoint == "http://" {
		return nil, errors.New("coralogix exporter config requires `logs.endpoint` configuration")
	}
	userAgent := fmt.Sprintf("%s/%s (%s/%s)",
		set.BuildInfo.Description, set.BuildInfo.Version, runtime.GOOS, runtime.GOARCH)

	return &logsExporter{config: oCfg, settings: set.TelemetrySettings, userAgent: userAgent}, nil
}

type logsExporter struct {
	// Input configuration.
	config *Config

	logExporter plogotlp.Client
	clientConn  *grpc.ClientConn
	metadata    metadata.MD
	callOptions []grpc.CallOption

	settings component.TelemetrySettings

	// Default user-agent header.
	userAgent string
}

func (e *logsExporter) start(ctx context.Context, host component.Host) (err error) {
	dialOpts, err := e.config.Logs.ToDialOptions(host, e.settings)
	if err != nil {
		return err
	}
	dialOpts = append(dialOpts, grpc.WithUserAgent(e.userAgent))

	if e.clientConn, err = grpc.DialContext(ctx, e.config.Logs.SanitizedEndpoint(), dialOpts...); err != nil {
		return err
	}

	e.logExporter = plogotlp.NewClient(e.clientConn)
	headers := e.config.Logs.Headers
	headers["CX-Application-Name"] = e.config.AppName
	headers["CX-Subsystem-Name"] = e.config.SubSystem
	headers["Authorization"] = "Bearer " + e.config.PrivateKey
	e.metadata = metadata.New(headers)
	e.callOptions = []grpc.CallOption{
		grpc.WaitForReady(e.config.Logs.WaitForReady),
	}

	return
}

func (e *logsExporter) shutdown(context.Context) error {
	return e.clientConn.Close()
}

func (e *logsExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	req := plogotlp.NewRequestFromLogs(ld)

	_, err := e.logExporter.Export(e.enhanceContext(ctx), req, e.callOptions...)
	return processError(err)
}

func (e *logsExporter) enhanceContext(ctx context.Context) context.Context {
	if e.metadata.Len() > 0 {
		return metadata.NewOutgoingContext(ctx, e.metadata)
	}
	return ctx
}
