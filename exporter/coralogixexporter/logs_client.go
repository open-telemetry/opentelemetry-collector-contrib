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

	logExporter plogotlp.GRPCClient
	clientConn  *grpc.ClientConn
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

	e.logExporter = plogotlp.NewGRPCClient(e.clientConn)
	if e.config.Logs.Headers == nil {
		e.config.Logs.Headers = make(map[string]string)
	}
	e.config.Logs.Headers["Authorization"] = "Bearer " + e.config.PrivateKey

	e.callOptions = []grpc.CallOption{
		grpc.WaitForReady(e.config.Logs.WaitForReady),
	}

	return
}

func (e *logsExporter) shutdown(context.Context) error {
	return e.clientConn.Close()
}

func (e *logsExporter) pushLogs(ctx context.Context, ld plog.Logs) error {

	rss := ld.ResourceLogs()
	for i := 0; i < rss.Len(); i++ {
		resourceLog := rss.At(i)
		appName, subsystem := e.config.getMetadataFromResource(resourceLog.Resource())

		ld := plog.NewLogs()
		newRss := ld.ResourceLogs().AppendEmpty()
		resourceLog.CopyTo(newRss)

		req := plogotlp.NewExportRequestFromLogs(ld)
		_, err := e.logExporter.Export(e.enhanceContext(ctx, appName, subsystem), req, e.callOptions...)
		if err != nil {
			return processError(err)
		}
	}
	return nil
}

func (e *logsExporter) enhanceContext(ctx context.Context, appName, subSystemName string) context.Context {
	headers := make(map[string]string)
	for k, v := range e.config.Logs.Headers {
		headers[k] = v
	}

	headers["CX-Application-Name"] = appName
	headers["CX-Subsystem-Name"] = subSystemName

	return metadata.NewOutgoingContext(ctx, metadata.New(headers))
}
