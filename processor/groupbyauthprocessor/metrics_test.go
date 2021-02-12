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

package groupbyauthprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessorMetrics(t *testing.T) {
	expectedViewNames := []string{
		"processor/groupbyauth/processor_groupbyauth_batches_no_token",
		"processor/groupbyauth/processor_groupbyauth_batches_with_token",
		"processor/groupbyauth/processor_groupbyauth_conf_num_traces",
		"processor/groupbyauth/processor_groupbyauth_num_events_in_queue",
		"processor/groupbyauth/processor_groupbyauth_num_traces_in_memory",
		"processor/groupbyauth/processor_groupbyauth_traces_evicted",
		"processor/groupbyauth/processor_groupbyauth_spans_released",
		"processor/groupbyauth/processor_groupbyauth_traces_released",
		"processor/groupbyauth/processor_groupbyauth_incomplete_releases",
		"processor/groupbyauth/processor_groupbyauth_event_latency",
	}

	views := MetricViews()
	for i, viewName := range expectedViewNames {
		assert.Equal(t, viewName, views[i].Name)
	}
}
