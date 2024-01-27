// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type expectedView struct {
	name     string
	tagCount int
}

func TestMetrics(t *testing.T) {
	metricViews := metricViews()
	viewNames := []expectedView{
		{name: "kafka_receiver_messages", tagCount: 2},
		{name: "kafka_receiver_current_offset", tagCount: 2},
		{name: "kafka_receiver_offset_lag", tagCount: 2},
		{name: "kafka_receiver_partition_start", tagCount: 1},
		{name: "kafka_receiver_partition_close", tagCount: 1},
		{name: "kafka_receiver_unmarshal_failed_metric_points", tagCount: 1},
		{name: "kafka_receiver_unmarshal_failed_log_records", tagCount: 1},
		{name: "kafka_receiver_unmarshal_failed_spans", tagCount: 1},
	}

	for i, expectedView := range viewNames {
		assert.Equal(t, expectedView.name, metricViews[i].Name)
		assert.Equal(t, expectedView.tagCount, len(metricViews[i].TagKeys))
	}
}
