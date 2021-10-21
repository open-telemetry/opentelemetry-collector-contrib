// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resourcedetectionprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

type resourceDetectionProcessor struct {
	provider  *internal.ResourceProvider
	resource  pdata.Resource
	schemaURL string
	override  bool
}

// Start is invoked during service startup.
func (rdp *resourceDetectionProcessor) Start(ctx context.Context, _ component.Host) error {
	var err error
	rdp.resource, rdp.schemaURL, err = rdp.provider.Get(ctx)
	return err
}

// processTraces implements the ProcessTracesFunc type.
func (rdp *resourceDetectionProcessor) processTraces(_ context.Context, td pdata.Traces) (pdata.Traces, error) {
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
func (rdp *resourceDetectionProcessor) processMetrics(_ context.Context, md pdata.Metrics) (pdata.Metrics, error) {
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
func (rdp *resourceDetectionProcessor) processLogs(_ context.Context, ld pdata.Logs) (pdata.Logs, error) {
	rl := ld.ResourceLogs()
	for i := 0; i < rl.Len(); i++ {
		rss := rl.At(i)
		rss.SetSchemaUrl(internal.MergeSchemaURL(rss.SchemaUrl(), rdp.schemaURL))
		res := rss.Resource()
		internal.MergeResource(res, rdp.resource, rdp.override)
	}
	return ld, nil
}
