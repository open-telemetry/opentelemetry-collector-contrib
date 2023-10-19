// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package honeycombmarkerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombmarkerexporter"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"go.opentelemetry.io/collector/component"
	"net/http"
	"strings"

	"go.opentelemetry.io/collector/pdata/plog"

	"go.uber.org/zap"
)

type honeycombLogsExporter struct {
	logger  *zap.Logger
	markers []Marker
	client  *http.Client
	APIURL  string
}

func newHoneycombLogsExporter(set component.TelemetrySettings, config *Config) (*honeycombLogsExporter, error) {
	if config == nil {
		return nil, fmt.Errorf("unable to create honeycombLogsExporter without config")
	}

	for _, m := range config.Markers {
		matchLogConditions, err := filterottl.NewBoolExprForLog(m.Rules.LogConditions, filterottl.StandardLogFuncs(), ottl.PropagateError, set)
		if err != nil {
			return nil, fmt.Errorf("failed to parse log conditions: %w", err)
		}

		m.Rules.logBoolExpr = matchLogConditions
	}
	logsExp := &honeycombLogsExporter{
		logger:  set.Logger,
		markers: config.Markers,
		APIURL:  config.APIURL,
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
				tCtx := ottllog.NewTransformContext(logRecord, slogs.Scope(), rlogs.Resource())
				for _, m := range e.markers {
					match, err := m.Rules.logBoolExpr.Eval(ctx, tCtx)
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

func (e *honeycombLogsExporter) sendMarker(ctx context.Context, marker Marker, logRecord plog.LogRecord) error {
	requestMap := map[string]string{
		"type": marker.Type,
	}

	messageField, found := logRecord.Attributes().Get(marker.MessageField)
	if found {
		requestMap["messageField"] = messageField.AsString()
	}

	URLField, found := logRecord.Attributes().Get(marker.URLField)
	if found {
		requestMap["URLField"] = URLField.AsString()
	}

	request, err := json.Marshal(requestMap)

	url := strings.TrimSuffix(e.APIURL, "/") + "/bundle"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(request))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send a request: %w", err)
	}

	defer resp.Body.Close()

	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("failed with %s and message: %s", resp.Status, resp.Body)
	}

	return nil
}
