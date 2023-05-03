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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

func DataPointFunctions() map[string]ottl.Factory[ottldatapoint.TransformContext] {
	functions := common.Functions[ottldatapoint.TransformContext]()

	datapointFunctions := ottl.CreateFactoryMap[ottldatapoint.TransformContext](
		newConvertSumToGaugeFactory(),
		newConvertGaugeToSumFactory(),
		newConvertSummarySumValToSumFactory(),
		newConvertSummaryCountValToSumFactory(),
	)

	for k, v := range datapointFunctions {
		functions[k] = v
	}

	return functions
}

func MetricFunctions() map[string]ottl.Factory[ottlmetric.TransformContext] {
	return common.Functions[ottlmetric.TransformContext]()
}
