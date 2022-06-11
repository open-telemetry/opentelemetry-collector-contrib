// Copyright 2019 OpenTelemetry Authors
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

package googlecloudexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/resource"
	"google.golang.org/genproto/googleapis/api/monitoredres"
)

func TestResourceMapper(t *testing.T) {
	rm := resourceMapper{
		mappings: []ResourceMapping{
			{
				SourceType: "source.resource1",
				TargetType: "target_resource_1",
				LabelMappings: []LabelMapping{
					{
						SourceKey: "contrib.opencensus.io/exporter/stackdriver/project_id",
						TargetKey: "project_id",
						Optional:  true,
					},
					{
						SourceKey: "renamedLabel",
						TargetKey: "target_label",
					},
				},
			},
			{
				SourceType: "source.resource2",
				TargetType: "target_resource_2",
			},
		},
	}

	tests := []struct {
		name           string
		sourceResource *resource.Resource
		wantResource   *monitoredres.MonitoredResource
	}{
		{
			name: "Converted resource with all matching labels",
			sourceResource: &resource.Resource{
				Type: "source.resource1",
				Labels: map[string]string{
					"renamedLabel": "value1",
					"contrib.opencensus.io/exporter/stackdriver/project_id": "123",
					"unknown": "value1",
				},
			},
			wantResource: &monitoredres.MonitoredResource{
				// Resource type transformed
				Type: "target_resource_1",
				Labels: map[string]string{
					// Both labels transformed
					"project_id":   "123",
					"target_label": "value1",
				},
			},
		},
		{
			name: "Converted resource with optional label missing",
			sourceResource: &resource.Resource{
				Type: "source.resource1",
				Labels: map[string]string{
					"renamedLabel": "value1",
					"unknown":      "value1",
				},
			},
			wantResource: &monitoredres.MonitoredResource{
				// Resource type transformed
				Type: "target_resource_1",
				Labels: map[string]string{
					// Required label transformed
					"target_label": "value1",
				},
			},
		},
		{
			name: "Converted resource with required label missing",
			sourceResource: &resource.Resource{
				Type: "source.resource1",
				Labels: map[string]string{
					"contrib.opencensus.io/exporter/stackdriver/project_id": "123",
				},
			},
			// Resource with missing required labels should be converted via default implementation
			wantResource: &monitoredres.MonitoredResource{
				// Resource type transformed
				Type: "global",
				Labels: map[string]string{
					// project_id is transformed by default function
					"project_id": "123",
				},
			},
		},
		{
			name: "Resource without matching labels",
			sourceResource: &resource.Resource{
				Type: "source.resource2",
				Labels: map[string]string{
					"someLabel": "value1",
					"contrib.opencensus.io/exporter/stackdriver/project_id": "123",
				},
			},
			// Resource without matching labels should drop all labels
			wantResource: &monitoredres.MonitoredResource{
				// Resource type transformed
				Type: "target_resource_2",
				// All labels are dropped
				Labels: map[string]string{},
			},
		},
		{
			name: "Resource without matching type",
			sourceResource: &resource.Resource{
				Type: "source.resource3",
				Labels: map[string]string{
					"source.label1": "value1", // unknown label is dropped
					"contrib.opencensus.io/exporter/stackdriver/project_id":             "123",
					"cloud.availability_zone":                                           "zone1",
					"contrib.opencensus.io/exporter/stackdriver/generic_task/namespace": "ns1",
					"contrib.opencensus.io/exporter/stackdriver/generic_task/job":       "job1",
					"contrib.opencensus.io/exporter/stackdriver/generic_task/task_id":   "task1",
				},
			},
			// Resource without matching config should be converted via default implementation
			wantResource: &monitoredres.MonitoredResource{
				Type: "generic_task",
				// All labels are transformed by default function
				Labels: map[string]string{
					"project_id": "123",
					"location":   "zone1",
					"namespace":  "ns1",
					"job":        "job1",
					"task_id":    "task1",
				},
			},
		},
		{
			name: "Handle cloud.zone for backcompat",
			sourceResource: &resource.Resource{
				Type: "source.resource3",
				Labels: map[string]string{
					"contrib.opencensus.io/exporter/stackdriver/project_id": "123",
					"cloud.zone": "zone1",
					"contrib.opencensus.io/exporter/stackdriver/generic_task/namespace": "ns1",
					"contrib.opencensus.io/exporter/stackdriver/generic_task/job":       "job1",
					"contrib.opencensus.io/exporter/stackdriver/generic_task/task_id":   "task1",
				},
			},
			wantResource: &monitoredres.MonitoredResource{
				Type: "generic_task",
				// All labels are transformed by default function
				Labels: map[string]string{
					"project_id": "123",
					"location":   "zone1",
					"namespace":  "ns1",
					"job":        "job1",
					"task_id":    "task1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rm.mapResource(tt.sourceResource)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantResource.Type, result.Type)
			assert.EqualValues(t, tt.wantResource.Labels, result.Labels)
		})
	}
}
