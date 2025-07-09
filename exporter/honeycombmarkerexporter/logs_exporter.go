// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package honeycombmarkerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombmarkerexporter"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
)

const (
	defaultDatasetSlug = "__all__"
	userAgentHeaderKey = "User-Agent"
	contentType        = "Content-Type"
	honeycombTeam      = "X-Honeycomb-Team"
)

type marker struct {
	Marker
	logBoolExpr *ottl.ConditionSequence[ottllog.TransformContext]
}

type honeycombLogsExporter struct {
	set                component.TelemetrySettings
	client             *http.Client
	httpClientSettings confighttp.ClientConfig
	apiURL             string
	apiKey             configopaque.String
	markers            []marker
	userAgentHeader    string
}

func newHoneycombLogsExporter(set exporter.Settings, config *Config) (*honeycombLogsExporter, error) {
	if config == nil {
		return nil, errors.New("unable to create honeycombLogsExporter without config")
	}

	telemetrySettings := set.TelemetrySettings
	markers := make([]marker, len(config.Markers))
	for i, m := range config.Markers {
		matchLogConditions, err := filterottl.NewBoolExprForLog(m.Rules.LogConditions, filterottl.StandardLogFuncs(), ottl.PropagateError, telemetrySettings)
		if err != nil {
			return nil, fmt.Errorf("failed to parse log conditions: %w", err)
		}
		markers[i] = marker{
			Marker:      m,
			logBoolExpr: matchLogConditions,
		}
	}
	logsExp := &honeycombLogsExporter{
		set:                telemetrySettings,
		httpClientSettings: config.ClientConfig,
		apiURL:             config.APIURL,
		apiKey:             config.APIKey,
		markers:            markers,
		userAgentHeader:    fmt.Sprintf("%s/%s (%s/%s)", set.BuildInfo.Description, set.BuildInfo.Version, runtime.GOOS, runtime.GOARCH),
	}
	return logsExp, nil
}

func (e *honeycombLogsExporter) exportMarkers(ctx context.Context, ld plog.Logs) error {
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rlogs := ld.ResourceLogs().At(i)
		for j := 0; j < rlogs.ScopeLogs().Len(); j++ {
			slogs := rlogs.ScopeLogs().At(j)
			logs := slogs.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				logRecord := logs.At(k)
				tCtx := ottllog.NewTransformContext(logRecord, slogs.Scope(), rlogs.Resource(), slogs, rlogs)
				for _, m := range e.markers {
					match, err := m.logBoolExpr.Eval(ctx, tCtx)
					if err != nil {
						return err
					}
					if match {
						err := e.sendMarker(ctx, m, logRecord)
						if err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

func (e *honeycombLogsExporter) sendMarker(ctx context.Context, m marker, logRecord plog.LogRecord) error {
	requestMap := map[string]string{
		"type": m.Type,
	}

	messageValue, found := logRecord.Attributes().Get(m.MessageKey)
	if found {
		requestMap["message"] = messageValue.AsString()
	}

	URLValue, found := logRecord.Attributes().Get(m.URLKey)
	if found {
		requestMap["url"] = URLValue.AsString()
	}

	request, err := json.Marshal(requestMap)
	if err != nil {
		return err
	}

	datasetSlug := m.DatasetSlug
	if datasetSlug == "" {
		datasetSlug = defaultDatasetSlug
	}

	url := fmt.Sprintf("%s/1/markers/%s", strings.TrimRight(e.apiURL, "/"), datasetSlug)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(request))
	if err != nil {
		return err
	}

	req.Header.Set(contentType, "application/json")
	req.Header.Set(honeycombTeam, string(e.apiKey))
	req.Header.Set(userAgentHeaderKey, e.userAgentHeader)

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send a request: %w", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("marker creation failed with %s and unable to read response body: %w", resp.Status, err)
		}
		return fmt.Errorf("marker creation failed with %s and message: %s", resp.Status, b)
	}

	return nil
}

func (e *honeycombLogsExporter) start(ctx context.Context, host component.Host) (err error) {
	client, err := e.httpClientSettings.ToClient(ctx, host, e.set)
	if err != nil {
		return err
	}

	e.client = client

	return nil
}
