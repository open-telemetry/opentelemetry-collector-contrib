// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		wantErr string
	}{
		{
			name:    "testdata/no_type.yaml",
			wantErr: "missing type",
		},
		{
			name:    "testdata/no_status.yaml",
			wantErr: "missing status",
		},
		{
			name:    "testdata/no_class.yaml",
			wantErr: "missing class",
		},
		{
			name:    "testdata/invalid_class.yaml",
			wantErr: "invalid class: incorrectclass",
		},
		{
			name:    "testdata/no_stability.yaml",
			wantErr: "missing stability",
		},
		{
			name:    "testdata/invalid_stability.yaml",
			wantErr: "invalid stability: incorrectstability",
		},
		{
			name:    "testdata/no_stability_component.yaml",
			wantErr: "missing component for stability: beta",
		},
		{
			name:    "testdata/invalid_stability_component.yaml",
			wantErr: "invalid component: incorrectcomponent",
		},
		{
			name:    "testdata/no_description_rattr.yaml",
			wantErr: "empty description for resource attribute: string.resource.attr",
		},
		{
			name:    "testdata/no_type_rattr.yaml",
			wantErr: "empty type for resource attribute: string.resource.attr",
		},
		{
			name:    "testdata/no_metric_description.yaml",
			wantErr: "metric \"default.metric\": missing metric description",
		},
		{
			name:    "testdata/no_metric_unit.yaml",
			wantErr: "metric \"default.metric\": missing metric unit",
		},
		{
			name: "testdata/no_metric_type.yaml",
			wantErr: "metric system.cpu.time doesn't have a metric type key, " +
				"one of the following has to be specified: sum, gauge",
		},
		{
			name: "testdata/two_metric_types.yaml",
			wantErr: "metric system.cpu.time has more than one metric type keys, " +
				"only one of the following has to be specified: sum, gauge",
		},
		{
			name:    "testdata/invalid_input_type.yaml",
			wantErr: "metric \"system.cpu.time\": invalid `input_type` value \"double\", must be \"\" or \"string\"",
		},
		{
			name:    "testdata/unknown_metric_attribute.yaml",
			wantErr: "metric \"system.cpu.time\" refers to undefined attributes: [missing]",
		},
		{
			name:    "testdata/unused_attribute.yaml",
			wantErr: "unused attributes: [unused_attr]",
		},
		{
			name:    "testdata/no_description_attr.yaml",
			wantErr: "missing attribute description for: string_attr",
		},
		{
			name:    "testdata/no_type_attr.yaml",
			wantErr: "empty type for attribute: used_attr",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadMetadata(tt.name)
			require.Error(t, err)
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestMetricCollision(t *testing.T) {
	allMetrics := map[string][]string{}
	err := filepath.Walk("../../receiver", func(path string, info fs.FileInfo, err error) error {
		if info.Name() == "metadata.yaml" {
			// TODO: Remove once https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/24011 is merged.
			if path == "../../receiver/googlecloudspannerreceiver/internal/metadataconfig/metadata.yaml" {
				return nil
			}
			md, err := loadMetadata(path)
			assert.NoError(t, err)
			if len(md.Metrics) > 0 {
				for metricName := range md.Metrics {
					allMetrics[md.Type] = append(allMetrics[md.Type], string(metricName))
				}
			}
		}
		return nil
	})
	assert.NoError(t, err)

	seen := make(map[string]string)
	for receiver, metrics := range allMetrics {
		for _, metricName := range metrics {
			// TODO: Remove one https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/24009 and
			// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/24008 have been addressed.
			if metricName == "container.cpu.utilization" || metricName == "container.memory.rss" {
				continue
			}
			val, exists := seen[metricName]
			assert.False(t, exists, fmt.Sprintf("Collision for metric %v in receivers %v and %v \n", metricName, receiver, val))
			seen[metricName] = receiver
		}
	}
}
