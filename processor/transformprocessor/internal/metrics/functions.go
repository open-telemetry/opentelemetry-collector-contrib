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

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

// registry is a map of names to functions for metrics pipelines
var datapointRegistry = map[string]interface{}{
	"convert_sum_to_gauge":             convertSumToGauge,
	"convert_gauge_to_sum":             convertGaugeToSum,
	"convert_summary_sum_val_to_sum":   convertSummarySumValToSum,
	"convert_summary_count_val_to_sum": convertSummaryCountValToSum,
	"convert_gauge_to_histogram":       convertGaugeToHistogram,
}

func init() {
	// Init metrics registry with default functions common to all signals
	for k, v := range common.Functions[ottldatapoint.TransformContext]() {
		datapointRegistry[k] = v
	}
}

func DataPointFunctions() map[string]interface{} {
	return datapointRegistry
}

func MetricFunctions() map[string]interface{} {
	return common.Functions[ottlmetric.TransformContext]()
}
