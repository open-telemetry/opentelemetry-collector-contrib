// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package instanaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter"

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/backend"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/converter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/converter/model"
)

type instanaExporter struct {
	config    *Config
	client    *http.Client
	settings  component.TelemetrySettings
	userAgent string
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
	converter := converter.NewConvertAllConverter(e.settings.Logger)
	var spans []model.Span

	hostID := ""
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		resSpan := resourceSpans.At(i)

		resource := resSpan.Resource()

		hostIDAttr, ok := resource.Attributes().Get(backend.AttributeInstanaHostID)
		if ok {
			hostID = hostIDAttr.Str()
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
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	headers := map[string]string{
		backend.HeaderKey:  string(e.config.AgentKey),
		backend.HeaderHost: hostID,
		// Used only by the Instana agent and can be set to "0" for the exporter
		backend.HeaderTime: "0",
	}

	return e.export(ctx, e.config.Endpoint, headers, req)
}

func newInstanaExporter(cfg component.Config, set exporter.CreateSettings) *instanaExporter {
	iCfg := cfg.(*Config)
	userAgent := fmt.Sprintf("%s/%s (%s/%s)", set.BuildInfo.Description, set.BuildInfo.Version, runtime.GOOS, runtime.GOARCH)
	return &instanaExporter{
		config:    iCfg,
		settings:  set.TelemetrySettings,
		userAgent: userAgent,
	}
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
		return fmt.Errorf("failed to send a request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode <= 499 {
		return consumererror.NewPermanent(fmt.Errorf("error when sending payload to %s: %s",
			url, resp.Status))
	}
	if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
		return fmt.Errorf("error when sending payload to %s: %s", url, resp.Status)
	}

	return nil
}
