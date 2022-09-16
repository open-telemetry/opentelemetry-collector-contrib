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
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type tracesExporter struct {
	// Input configuration.
	config *Config

	traceExporter ptraceotlp.Client
	clientConn    *grpc.ClientConn
	metadata      metadata.MD
	callOptions   []grpc.CallOption

	settings component.TelemetrySettings

	// Default user-agent header.
	userAgent string
}

func newTracesExporter(cfg config.Exporter, set component.ExporterCreateSettings) (*tracesExporter, error) {
	oCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config exporter, expect type: %T, got: %T", &Config{}, cfg)
	}

	if oCfg.Traces.Endpoint == "" || oCfg.Traces.Endpoint == "https://" || oCfg.Traces.Endpoint == "http://" {
		return nil, errors.New("coralogix exporter config requires `Traces.endpoint` configuration")
	}
	userAgent := fmt.Sprintf("%s/%s (%s/%s)",
		set.BuildInfo.Description, set.BuildInfo.Version, runtime.GOOS, runtime.GOARCH)

	return &tracesExporter{config: oCfg, settings: set.TelemetrySettings, userAgent: userAgent}, nil
}

func (e *tracesExporter) start(_ context.Context, host component.Host) error {
	dialOpts, err := e.config.Traces.ToDialOptions(host, e.settings)
	if err != nil {
		return err
	}
	dialOpts = append(dialOpts, grpc.WithUserAgent(e.userAgent))

	if e.clientConn, err = grpc.Dial(e.config.Traces.SanitizedEndpoint(), dialOpts...); err != nil {
		return err
	}

	e.traceExporter = ptraceotlp.NewClient(e.clientConn)
	if e.config.Traces.Headers == nil {
		e.config.Traces.Headers = make(map[string]string)
	}
	headers := e.config.Traces.Headers
	headers["CX-Application-Name"] = e.config.AppName
	headers["CX-Subsystem-Name"] = e.config.SubSystem
	headers["Authorization"] = "Bearer " + e.config.PrivateKey
	e.metadata = metadata.New(headers)
	e.callOptions = []grpc.CallOption{
		grpc.WaitForReady(e.config.Traces.WaitForReady),
	}

	return nil
}

func (e *tracesExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {
	req := ptraceotlp.NewRequestFromTraces(td)

	_, err := e.traceExporter.Export(e.enhanceContext(ctx), req, e.callOptions...)
	return processError(err)
}
func (e *tracesExporter) shutdown(context.Context) error {
	return e.clientConn.Close()
}
func (e *tracesExporter) enhanceContext(ctx context.Context) context.Context {
	if e.metadata.Len() > 0 {
		return metadata.NewOutgoingContext(ctx, e.metadata)
	}
	return ctx
}
