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

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

func Test_DataPointFunctions(t *testing.T) {
	expected := common.Functions[ottldatapoint.TransformContext]()
	expected["convert_sum_to_gauge"] = newConvertSumToGaugeFactory()
	expected["convert_gauge_to_sum"] = newConvertGaugeToSumFactory()
	expected["convert_summary_sum_val_to_sum"] = newConvertSummarySumValToSumFactory()
	expected["convert_summary_count_val_to_sum"] = newConvertSummaryCountValToSumFactory()

	actual := DataPointFunctions()

	require.Equal(t, len(expected), len(actual))
	for k := range actual {
		assert.Contains(t, expected, k)
	}
}

func Test_MetricFunctions(t *testing.T) {
	expected := common.Functions[ottlmetric.TransformContext]()
	actual := MetricFunctions()
	require.Equal(t, len(expected), len(actual))
	for k := range actual {
		assert.Contains(t, expected, k)
	}
}
