// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessorMetrics(t *testing.T) {
	expectedViewNames := []string{
		"jaegerexporter_conn_state",
	}

	views := MetricViews()
	for i, viewName := range expectedViewNames {
		assert.Equal(t, viewName, views[i].Name)
	}
}
