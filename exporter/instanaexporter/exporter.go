// Copyright 2022, OpenTelemetry Authors
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

package instanaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/backend"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/converter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/converter/model"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/otlptext"
)

type instanaExporter struct {
	config          *Config
	client          *http.Client
	tracesMarshaler ptrace.Marshaler
	settings        component.TelemetrySettings
	userAgent       string
}

func (e *instanaExporter) start(_ context.Context, host component.Host) error {
	client, err := e.config.HTTPClientSettings.ToClient(host, e.settings)
	if err != nil {
		return err
	}
	e.client = client
	return nil
}

func (e *instanaExporter) pushConvertedTraces(ctx context.Context, td ptrace.Traces) error {
	e.settings.Logger.Info("TracesExporter", zap.Int("#spans", td.SpanCount()))
	if !e.settings.Logger.Core().Enabled(zapcore.DebugLevel) {
		return nil
	}

	buf, err := e.tracesMarshaler.MarshalTraces(td)
	if err != nil {
		return err
	}
	e.settings.Logger.Debug(string(buf))

	converter := converter.NewConvertAllConverter(e.settings.Logger)
	spans := make([]model.Span, 0)

	hostID := ""
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		resSpan := resourceSpans.At(i)

		resource := resSpan.Resource()

		hostIDAttr, ex := resource.Attributes().Get(backend.AttributeInstanaHostID)
		if ex {
			hostID = hostIDAttr.StringVal()
		}

		ilSpans := resSpan.ScopeSpans()
		for j := 0; j < ilSpans.Len(); j++ {
			converterBundle := converter.ConvertSpans(resource.Attributes(), ilSpans.At(j).Spans())

			spans = append(spans, converterBundle.Spans...)
		}
	}

	bundle := model.Bundle{Spans: spans}

	if len(bundle.Spans) == 0 {
		// skip exporting, nothing to do
		return nil
	}

	req, err := bundle.Marshal()

	e.settings.Logger.Debug(string(req))

	if err != nil {
		return consumererror.NewPermanent(err)
	}

	headers := map[string]string{
		backend.HeaderKey:  e.config.AgentKey,
		backend.HeaderHost: hostID,
		backend.HeaderTime: "0",
	}

	return e.export(ctx, e.config.Endpoint, headers, req)
}

func newInstanaExporter(cfg config.Exporter, set component.ExporterCreateSettings) (*instanaExporter, error) {
	iCfg := cfg.(*Config)

	if iCfg.Endpoint != "" {
		_, err := url.Parse(iCfg.Endpoint)
		if err != nil {
			return nil, errors.New("endpoint must be a valid URL")
		}
	}

	userAgent := fmt.Sprintf("%s/%s (%s/%s)", set.BuildInfo.Description, set.BuildInfo.Version, runtime.GOOS, runtime.GOARCH)

	return &instanaExporter{
		config:          iCfg,
		settings:        set.TelemetrySettings,
		tracesMarshaler: otlptext.NewTextTracesMarshaler(),
		userAgent:       userAgent,
	}, nil
}

func (e *instanaExporter) export(ctx context.Context, url string, header map[string]string, request []byte) error {
	url = strings.TrimSuffix(url, "/") + "/bundle"

	e.settings.Logger.Debug("Preparing to make HTTP request", zap.String("url", url))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(request))
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", e.userAgent)

	for name, value := range header {
		req.Header.Set(name, value)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make an HTTP request: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		// Request is successful.
		return nil
	}

	return nil
}
