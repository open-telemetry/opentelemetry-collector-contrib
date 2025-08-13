// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcemapping

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

func TestCustomMonitoredResourceMapping(t *testing.T) {
	tests := []struct {
		name     string
		resource pcommon.Resource
		want     *monitoredrespb.MonitoredResource
	}{
		{
			name: "Custom Monitored Resource",
			resource: func() pcommon.Resource {
				r := pcommon.NewResource()
				r.Attributes().PutStr("gcp.resource_type", "custom_resource")
				r.Attributes().PutStr("gcp.custom_resource.label1", "value1")
				r.Attributes().PutStr("gcp.custom_resource.label2", "value2")
				r.Attributes().PutStr("other.label", "otherValue")
				return r
			}(),
			want: &monitoredrespb.MonitoredResource{
				Type: "custom_resource",
				Labels: map[string]string{
					"label1": "value1",
					"label2": "value2",
				},
			},
		},
		{
			name: "No Custom Monitored Resource",
			resource: func() pcommon.Resource {
				r := pcommon.NewResource()
				r.Attributes().PutStr("other.label", "otherValue")
				return r
			}(),
			want: &monitoredrespb.MonitoredResource{
				Type: "generic_node",
				Labels: map[string]string{
					"location":  "global",
					"namespace": "",
					"node_id":   "",
				},
			},
		},
		{
			name: "Empty Custom Monitored Resource",
			resource: func() pcommon.Resource {
				r := pcommon.NewResource()
				r.Attributes().PutStr("gcp.resource_type", "")
				r.Attributes().PutStr("gcp..label1", "value1")
				return r
			}(),
			want: &monitoredrespb.MonitoredResource{
				Type: "generic_node",
				Labels: map[string]string{
					"location":  "global",
					"namespace": "",
					"node_id":   "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CustomMetricMonitoredResourceMapping(tt.resource)
			assert.Equal(t, tt.want, got)
			got = CustomLoggingMonitoredResourceMapping(tt.resource)
			assert.Equal(t, tt.want, got)
		})
	}
}
