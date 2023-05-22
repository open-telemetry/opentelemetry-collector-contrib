// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbyattrsprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessorMetrics(t *testing.T) {
	expectedViewNames := []string{
		"processor/groupbyattrs/num_grouped_spans",
		"processor/groupbyattrs/num_non_grouped_spans",
		"processor/groupbyattrs/span_groups",
		"processor/groupbyattrs/num_grouped_logs",
		"processor/groupbyattrs/num_non_grouped_logs",
		"processor/groupbyattrs/log_groups",
	}

	views := MetricViews()
	for i, viewName := range expectedViewNames {
		assert.Equal(t, viewName, views[i].Name)
	}
}
