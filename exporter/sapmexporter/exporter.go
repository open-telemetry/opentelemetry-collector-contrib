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

// Package sapmexporter exports trace data using Splunk's SAPM protocol.
package sapmexporter

import (
	"context"

	sapmclient "github.com/signalfx/sapm-proto/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/translator/trace/jaeger"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

// sapmExporter is a wrapper struct of SAPM exporter
type sapmExporter struct {
	client  *sapmclient.Client
	logger  *zap.Logger
	config  *Config
	tracker *Tracker
}

func (se *sapmExporter) Shutdown(context.Context) error {
	se.client.Stop()
	se.tracker.Shutdown()
	return nil
}

func newSAPMExporter(cfg *Config, params component.ExporterCreateParams) (sapmExporter, error) {
	err := cfg.validate()
	if err != nil {
		return sapmExporter{}, err
	}

	client, err := sapmclient.New(cfg.clientOptions()...)
	if err != nil {
		return sapmExporter{}, err
	}

	var tracker *Tracker

	if cfg.Correlation.Enabled {
		tracker = NewTracker(cfg, params)
	}

	return sapmExporter{
		client:  client,
		logger:  params.Logger,
		config:  cfg,
		tracker: tracker,
	}, err
}

func newSAPMTraceExporter(cfg *Config, params component.ExporterCreateParams) (component.TraceExporter, error) {
	se, err := newSAPMExporter(cfg, params)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewTraceExporter(
		cfg,
		se.pushTraceData,
		exporterhelper.WithShutdown(se.Shutdown))
}

// tracesByAccessToken takes a pdata.Traces struct and will iterate through its ResourceSpans' attributes,
// regrouping by any SFx access token label value if Config.AccessTokenPassthrough is enabled.  It will delete any
// set token label in any case to prevent serialization.
// It returns a map of newly constructed pdata.Traces keyed by access token, defaulting to empty string.
func (se *sapmExporter) tracesByAccessToken(td pdata.Traces) map[string]pdata.Traces {
	tracesByToken := make(map[string]pdata.Traces, 1)
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		resourceSpan := resourceSpans.At(i)
		if resourceSpan.IsNil() {
			// Invalid trace so nothing to export
			continue
		}

		accessToken := ""
		if !resourceSpan.Resource().IsNil() {
			attrs := resourceSpan.Resource().Attributes()
			attributeValue, ok := attrs.Get(splunk.SFxAccessTokenLabel)
			if ok {
				attrs.Delete(splunk.SFxAccessTokenLabel)
				if se.config.AccessTokenPassthrough {
					accessToken = attributeValue.StringVal()
				}
			}
		}

		traceForToken, ok := tracesByToken[accessToken]
		if !ok {
			traceForToken = pdata.NewTraces()
			tracesByToken[accessToken] = traceForToken
		}

		// Append ResourceSpan to trace for this access token
		traceForTokenSize := traceForToken.ResourceSpans().Len()
		traceForToken.ResourceSpans().Resize(traceForTokenSize + 1)
		traceForToken.ResourceSpans().At(traceForTokenSize).InitEmpty()
		resourceSpan.CopyTo(traceForToken.ResourceSpans().At(traceForTokenSize))
	}

	return tracesByToken
}

// pushTraceData exports traces in SAPM proto by associated SFx access token and returns number of dropped spans
// and the last experienced error if any translation or export failed
func (se *sapmExporter) pushTraceData(ctx context.Context, td pdata.Traces) (droppedSpansCount int, err error) {
	traces := se.tracesByAccessToken(td)
	droppedSpansCount = 0
	for accessToken, trace := range traces {
		batches, translateErr := jaeger.InternalTracesToJaegerProto(trace)
		if translateErr != nil {
			droppedSpansCount += trace.SpanCount()
			err = consumererror.Permanent(translateErr)
			continue
		}

		exportErr := se.client.ExportWithAccessToken(ctx, batches, accessToken)
		if exportErr != nil {
			if sendErr, ok := exportErr.(*sapmclient.ErrSend); ok {
				if sendErr.Permanent {
					err = consumererror.Permanent(sendErr)
				}
			}
			droppedSpansCount += trace.SpanCount()
		}

		// NOTE: Correlation does not currently support inline access token.
		se.tracker.AddSpans(ctx, trace)
	}
	return
}
