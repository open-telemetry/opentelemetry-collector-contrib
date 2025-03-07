// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcemapping // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter/internal/resourcemapping"

import (
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"go.opentelemetry.io/collector/pdata/pcommon"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

var (
	mappingKey                   = "gcp.resource_type"
	monitoredResourceLabelPrefix = "gcp."
)

// CustomMonitoredResourceMapping allows mapping from OTel resources to
// Monitored Resources defined here:
// https://cloud.google.com/monitoring/api/resources
//
// The monitored resource type is extracted from the `gcp.resource_type`
// attribute. And the monitored resource labels are extracted from resource
// attributes with the prefix `gcp.<monitored resource type>.`.
func CustomMonitoredResourceMapping(r pcommon.Resource) *monitoredrespb.MonitoredResource {
	var monitoredResourceType string
	monitoredResourceLabels := make(map[string]string)
	r.Attributes().Range(func(k string, v pcommon.Value) bool {
		if k == mappingKey {
			monitoredResourceType = v.AsString()
			return false
		}
		return true
	})

	if monitoredResourceType == "" {
		return collector.DefaultConfig().MetricConfig.MapMonitoredResource(r)
	}

	prefix := monitoredResourceLabelPrefix + monitoredResourceType + "."
	r.Attributes().Range(func(k string, v pcommon.Value) bool {
		// Extract the monitored resource label by separating it from the prefix.
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			monitoredResourceLabels[k[len(prefix):]] = v.AsString()
		}
		return true
	})

	return &monitoredrespb.MonitoredResource{
		Type:   monitoredResourceType,
		Labels: monitoredResourceLabels,
	}
}
