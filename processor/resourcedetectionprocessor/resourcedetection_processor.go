// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcedetectionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

type resourceDetectionProcessor struct {
	provider           *internal.ResourceProvider
	resource           pcommon.Resource
	schemaURL          string
	override           bool
	httpClientSettings confighttp.HTTPClientSettings
	telemetrySettings  component.TelemetrySettings
}

// Start is invoked during service startup.
func (rdp *resourceDetectionProcessor) Start(ctx context.Context, host component.Host) error {
	client, _ := rdp.httpClientSettings.ToClient(host, rdp.telemetrySettings)
	ctx = internal.ContextWithClient(ctx, client)
	var err error
	rdp.resource, rdp.schemaURL, err = rdp.provider.Get(ctx, client)
	return err
}

// processTraces implements the ProcessTracesFunc type.
func (rdp *resourceDetectionProcessor) processTraces(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	rs := td.ResourceSpans()
	for i := 0; i < rs.Len(); i++ {
		rss := rs.At(i)
		rss.SetSchemaUrl(internal.MergeSchemaURL(rss.SchemaUrl(), rdp.schemaURL))
		res := rss.Resource()
		internal.MergeResource(res, rdp.resource, rdp.override)
	}
	return td, nil
}

// processMetrics implements the ProcessMetricsFunc type.
func (rdp *resourceDetectionProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rm := md.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		rss := rm.At(i)
		rss.SetSchemaUrl(internal.MergeSchemaURL(rss.SchemaUrl(), rdp.schemaURL))
		res := rss.Resource()
		internal.MergeResource(res, rdp.resource, rdp.override)
	}
	return md, nil
}

// processLogs implements the ProcessLogsFunc type.
func (rdp *resourceDetectionProcessor) processLogs(_ context.Context, ld plog.Logs) (plog.Logs, error) {
	rl := ld.ResourceLogs()
	for i := 0; i < rl.Len(); i++ {
		rss := rl.At(i)
		rss.SetSchemaUrl(internal.MergeSchemaURL(rss.SchemaUrl(), rdp.schemaURL))
		res := rss.Resource()
		internal.MergeResource(res, rdp.resource, rdp.override)
	}
	return ld, nil
}
