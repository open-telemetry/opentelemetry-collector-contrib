// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessorMetrics(t *testing.T) {
	expectedViewNames := []string{
		"processor/groupbytrace/processor_groupbytrace_conf_num_traces",
		"processor/groupbytrace/processor_groupbytrace_num_events_in_queue",
		"processor/groupbytrace/processor_groupbytrace_num_traces_in_memory",
		"processor/groupbytrace/processor_groupbytrace_traces_evicted",
		"processor/groupbytrace/processor_groupbytrace_spans_released",
		"processor/groupbytrace/processor_groupbytrace_traces_released",
		"processor/groupbytrace/processor_groupbytrace_incomplete_releases",
		"processor/groupbytrace/processor_groupbytrace_event_latency",
	}

	views := MetricViews()
	for i, viewName := range expectedViewNames {
		assert.Equal(t, viewName, views[i].Name)
	}
}
