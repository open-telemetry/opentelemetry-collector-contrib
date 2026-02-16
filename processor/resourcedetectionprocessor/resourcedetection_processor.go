// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcedetectionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

type resourceDetectionProcessor struct {
	provider           *internal.ResourceProvider
	override           bool
	httpClientSettings confighttp.ClientConfig
	refreshInterval    time.Duration
	telemetrySettings  component.TelemetrySettings
}

// Start is invoked during service startup.
func (rdp *resourceDetectionProcessor) Start(ctx context.Context, host component.Host) error {
	client, err := rdp.httpClientSettings.ToClient(ctx, host.GetExtensions(), rdp.telemetrySettings)
	if err != nil {
		return err
	}
	ctx = internal.ContextWithClient(ctx, client)

	// Perform initial resource detection
	err = rdp.provider.Refresh(ctx, client)
	if err != nil {
		return err
	}

	// Start periodic refresh if configured
	rdp.provider.StartRefreshing(rdp.refreshInterval, client)
	return nil
}

// Shutdown is invoked during service shutdown.
func (rdp *resourceDetectionProcessor) Shutdown(_ context.Context) error {
	rdp.provider.StopRefreshing()
	return nil
}

// processTraces implements the ProcessTracesFunc type.
func (rdp *resourceDetectionProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	res, schemaURL, _ := rdp.provider.Get(ctx, nil)
	rs := td.ResourceSpans()
	for i := 0; i < rs.Len(); i++ {
		rss := rs.At(i)
		rss.SetSchemaUrl(internal.MergeSchemaURL(rss.SchemaUrl(), schemaURL))
		internal.MergeResource(rss.Resource(), res, rdp.override)
	}
	return td, nil
}

// processMetrics implements the ProcessMetricsFunc type.
func (rdp *resourceDetectionProcessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	res, schemaURL, _ := rdp.provider.Get(ctx, nil)
	rm := md.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		rss := rm.At(i)
		rss.SetSchemaUrl(internal.MergeSchemaURL(rss.SchemaUrl(), schemaURL))
		internal.MergeResource(rss.Resource(), res, rdp.override)
	}
	return md, nil
}

// processLogs implements the ProcessLogsFunc type.
func (rdp *resourceDetectionProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	res, schemaURL, _ := rdp.provider.Get(ctx, nil)
	rl := ld.ResourceLogs()
	for i := 0; i < rl.Len(); i++ {
		rss := rl.At(i)
		rss.SetSchemaUrl(internal.MergeSchemaURL(rss.SchemaUrl(), schemaURL))
		internal.MergeResource(rss.Resource(), res, rdp.override)
	}
	return ld, nil
}

// processProfiles implements the ProcessProfilesFunc type.
func (rdp *resourceDetectionProcessor) processProfiles(ctx context.Context, ld pprofile.Profiles) (pprofile.Profiles, error) {
	res, schemaURL, _ := rdp.provider.Get(ctx, nil)
	rl := ld.ResourceProfiles()
	for i := 0; i < rl.Len(); i++ {
		rss := rl.At(i)
		rss.SetSchemaUrl(internal.MergeSchemaURL(rss.SchemaUrl(), schemaURL))
		internal.MergeResource(rss.Resource(), res, rdp.override)
	}
	return ld, nil
}
