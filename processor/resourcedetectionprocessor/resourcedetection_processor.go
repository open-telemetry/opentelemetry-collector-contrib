// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcedetectionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"

import (
	"context"
	"slices"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pentity"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

type resourceDetectionProcessor struct {
	provider           *internal.ResourceProvider
	resource           pcommon.Resource
	entityTypes        []string
	schemaURL          string
	override           bool
	httpClientSettings confighttp.ClientConfig
	telemetrySettings  component.TelemetrySettings
}

// Start is invoked during service startup.
func (rdp *resourceDetectionProcessor) Start(ctx context.Context, host component.Host) error {
	client, _ := rdp.httpClientSettings.ToClient(ctx, host, rdp.telemetrySettings)
	ctx = internal.ContextWithClient(ctx, client)
	var err error
	rdp.resource, rdp.schemaURL, err = rdp.provider.Get(ctx, client)
	for i := 0; i < rdp.resource.Entities().Len(); i++ {
		entityType := rdp.resource.Entities().At(i).Type()
		if entityType != "" {
			rdp.entityTypes = append(rdp.entityTypes, entityType)
		}
	}
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

// processLogs implements the ProcessLogsFunc type.
func (rdp *resourceDetectionProcessor) processEntities(_ context.Context, ld pentity.Entities) (pentity.Entities, error) {
	rl := ld.ResourceEntities()
	for i := 0; i < rl.Len(); i++ {
		rss := rl.At(i)

		// first check entity events for matching entity types. If found, skip the rest of the processing.
		eventsProcessed := rdp.processEntityEvents(rss)
		if eventsProcessed {
			continue
		}

		// if no entity events matched, merge the resource information
		rss.SetSchemaUrl(internal.MergeSchemaURL(rss.SchemaUrl(), rdp.schemaURL))
		internal.MergeResource(rss.Resource(), rdp.resource, rdp.override)

		internal.RemoveInvalidEntities(rss)
	}
	return ld, nil
}

func (rdp *resourceDetectionProcessor) processEntityEvents(re pentity.ResourceEntities) (eventsMatched bool) {
	for i := 0; i < re.ScopeEntities().Len(); i++ {
		se := re.ScopeEntities().At(i)
		for j := 0; j < se.EntityEvents().Len(); j++ {
			ee := se.EntityEvents().At(j)
			if ee.Type() == pentity.EventTypeEntityState && slices.Contains(rdp.entityTypes, ee.EntityType()) {
				eventsMatched = true
				var entityRef pcommon.ResourceEntityRef
				for k := 0; k < rdp.resource.Entities().Len(); k++ {
					entity := rdp.resource.Entities().At(k)
					if entity.Type() == ee.EntityType() {
						entityRef = entity
						break
					}
				}
				internal.MergeEntityRef(ee, entityRef, rdp.resource.Attributes(), rdp.override)
			}
		}
	}
	return
}
