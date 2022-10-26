// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filereloadereceiver

import (
	"go.opencensus.io/metric"
	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/metric/metricproducer"
)

const (
	pipelineType = "pipeline_type"
	reason       = "reason"
	status       = "status"
	failed       = "failed"
	succeeded    = "succeeded"
)

var (
	globalInstruments = newInstruments(metric.NewRegistry())
)

func init() {
	metricproducer.GlobalManager().AddProducer(globalInstruments.registry)
}

type instruments struct {
	registry                  *metric.Registry
	reloadsTotal              *metric.Int64Cumulative
	fileChangesTotal          *metric.Int64Cumulative
	createPipelineErrorsTotal *metric.Int64Cumulative
	startedPipelinesTotal     *metric.Int64Cumulative
	stoppedPipelinesTotal     *metric.Int64Cumulative
	activePipelines           *metric.Int64DerivedGauge
}

func newInstruments(registry *metric.Registry) *instruments {
	insts := &instruments{
		registry: registry,
	}
	insts.reloadsTotal, _ = registry.AddInt64Cumulative(
		typeStr+"_reloads_total",
		metric.WithDescription("total reload done by "+typeStr),
		metric.WithLabelKeys(status),
		metric.WithUnit(metricdata.UnitDimensionless),
	)
	insts.fileChangesTotal, _ = registry.AddInt64Cumulative(
		typeStr+"_file_changes_total",
		metric.WithDescription("total file changes detected by "+typeStr),
		metric.WithLabelKeys(),
		metric.WithUnit(metricdata.UnitDimensionless),
	)
	insts.createPipelineErrorsTotal, _ = registry.AddInt64Cumulative(
		typeStr+"_create_pipeline_errors_total",
		metric.WithDescription("total create pipeline errors in "+typeStr),
		metric.WithLabelKeys(pipelineType, reason),
		metric.WithUnit(metricdata.UnitDimensionless),
	)
	insts.startedPipelinesTotal, _ = registry.AddInt64Cumulative(
		typeStr+"_started_pipelines_total",
		metric.WithDescription("total pipelines started by "+typeStr),
		metric.WithLabelKeys(pipelineType, status),
		metric.WithUnit(metricdata.UnitDimensionless),
	)
	insts.stoppedPipelinesTotal, _ = registry.AddInt64Cumulative(
		typeStr+"_stopped_pipelines_total",
		metric.WithDescription("total pipelines stopped by "+typeStr),
		metric.WithLabelKeys(pipelineType, status),
		metric.WithUnit(metricdata.UnitDimensionless),
	)
	insts.activePipelines, _ = registry.AddInt64DerivedGauge(
		typeStr+"_active_pipelines",
		metric.WithDescription("current active pipelines in "+typeStr),
		metric.WithLabelKeys(pipelineType),
		metric.WithUnit(metricdata.UnitDimensionless),
	)

	return insts
}
