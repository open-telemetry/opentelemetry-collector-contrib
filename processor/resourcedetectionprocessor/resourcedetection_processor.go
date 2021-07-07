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
	provider *internal.ResourceProvider
	resource pdata.Resource
	override bool
}

// Start is invoked during service startup.
func (rdp *resourceDetectionProcessor) Start(ctx context.Context, _ component.Host) error {
	var err error
	rdp.resource, err = rdp.provider.Get(ctx)
	return err
}

// ProcessTraces implements the TracesProcessor interface
func (rdp *resourceDetectionProcessor) ProcessTraces(_ context.Context, td pdata.Traces) (pdata.Traces, error) {
	rs := td.ResourceSpans()
	for i := 0; i < rs.Len(); i++ {
		res := rs.At(i).Resource()
		internal.MergeResource(res, rdp.resource, rdp.override)
	}
	return td, nil
}

// ProcessMetrics implements the MetricsProcessor interface
func (rdp *resourceDetectionProcessor) ProcessMetrics(_ context.Context, md pdata.Metrics) (pdata.Metrics, error) {
	rm := md.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		res := rm.At(i).Resource()
		internal.MergeResource(res, rdp.resource, rdp.override)
	}
	return md, nil
}

// ProcessLogs implements the LogsProcessor interface
func (rdp *resourceDetectionProcessor) ProcessLogs(_ context.Context, ld pdata.Logs) (pdata.Logs, error) {
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		res := rls.At(i).Resource()
		internal.MergeResource(res, rdp.resource, rdp.override)
	}
	return ld, nil
}
