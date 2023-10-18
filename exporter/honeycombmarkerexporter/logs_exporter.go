// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package honeycombmarkerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombmarkerexporter"

import (
	"bytes"
	"context"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"net/http"
	"strings"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/plog"

	"go.uber.org/zap"
)

type honeycombLogsExporter struct {
	logger  *zap.Logger
	markers []Marker
	client  *http.Client
}

//// ties marker and associated conditions
//type marker struct { //rename
//	marker Marker

//}

func newHoneycombLogsExporter(logger *zap.Logger, config *Config) (*honeycombLogsExporter, error) {
	if config == nil {
		return nil, fmt.Errorf("unable to create honeycombLogsExporter without config")
	}
	for _, m := range config.Markers {
		matchLogConditions, err := filterottl.NewBoolExprForLog(m.Rules.LogConditions, filterottl.StandardLogFuncs(), ottl.PropagateError, componenttest.NewNopTelemetrySettings())
		if err != nil {
			return nil, fmt.Errorf("failed to compare log conditions: %w", err)
		}

		matchResourceConditions, err := filterottl.NewBoolExprForLog(m.Rules.ResourceConditions, filterottl.StandardLogFuncs(), ottl.PropagateError, componenttest.NewNopTelemetrySettings())
		if err != nil {
			return nil, fmt.Errorf("failed to compare resource conditions: %w", err)
		}

		m.Rules.logBoolExpr = matchLogConditions
		m.Rules.resourceBoolExpr = matchResourceConditions

	}
	logsExp := &honeycombLogsExporter{
		logger:  logger,
		markers: config.Markers,
	}
	return logsExp, nil
}

func (e *honeycombLogsExporter) exportMarkers(ctx context.Context, pl plog.Logs) error {

	// if any log or resource condition matches, send marker
	//for _, cond := range e.matchedLogConditions {
	//	logResult, err := cond.Eval(ctx, ottllog.NewTransformContext(plog.NewLogRecord(), pcommon.NewInstrumentationScope(), pcommon.NewResource()))
	//	if err != nil {
	//		return fmt.Errorf("failed to evaluate log conditions: %w", err)
	//	}
	//
	//	if logResult {
	//		headers := map[string]string{}
	//		//requestBody, err := json.Marshal(map[string]string{
	//		//	"type": marker.Type,
	//		//})
	//
	//		if err != nil {
	//			return fmt.Errorf("failed to create request body: %w", err)
	//		}
	//
	//		err = e.sendMarker(ctx, m.URLField, headers, requestBody)
	//		if err != nil {
	//			return fmt.Errorf("failed to send Marker: %w", err)
	//		}
	//	}
	//}

	resourceResult, err := e.matchedResourceConditions.Eval(ctx, ottllog.NewTransformContext(plog.NewLogRecord(), pcommon.NewInstrumentationScope(), pcommon.NewResource()))
	if err != nil {
		return fmt.Errorf("failed to evaluate resource conditions: %w", err)
	}

	if logResult || resourceResult {

	}
	return nil
}

func (e *honeycombLogsExporter) sendMarker(ctx context.Context, url string, header map[string]string, request []byte) error {
	url = strings.TrimSuffix(url, "/") + "/bundle"
	//e.settings.Logger.Debug("Preparing to make HTTP request", zap.String("url", url))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(request))
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	req.Header.Set("Content-Type", "application/json")
	//req.Header.Set("User-Agent", e.userAgent)

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
