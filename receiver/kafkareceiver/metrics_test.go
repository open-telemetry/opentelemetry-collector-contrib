// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetrics(t *testing.T) {
	metricViews := metricViews()
	viewNames := []string{
		"kafka_receiver_messages",
		"kafka_receiver_current_offset",
		"kafka_receiver_offset_lag",
		"kafka_receiver_partition_start",
		"kafka_receiver_partition_close",
		"kafka_receiver_unmarshal_failed_metric_points",
		"kafka_receiver_unmarshal_failed_log_records",
		"kafka_receiver_unmarshal_failed_spans",
	}
	for i, viewName := range viewNames {
		assert.Equal(t, viewName, metricViews[i].Name)
	}
}
