// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

import (
	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
	"go.opentelemetry.io/collector/pdata/pcommon" // Applies resource attributes values to data properties
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

const (
	instrumentationLibraryName    string = "instrumentationlibrary.name"
	instrumentationLibraryVersion string = "instrumentationlibrary.version"
)

// Applies resource attributes values to data properties
func applyResourcesToDataProperties(dataProperties map[string]string, resourceAttributes pcommon.Map) {
	// Copy all the resource labels into the base data properties. Resource values are always strings
	resourceAttributes.Range(func(k string, v pcommon.Value) bool {
		dataProperties[k] = v.Str()
		return true
	})
}

// Sets important ai.cloud.* tags on the envelope
func applyCloudTagsToEnvelope(envelope *contracts.Envelope, resourceAttributes pcommon.Map) {
	// Extract key service.* labels from the Resource labels and construct CloudRole and CloudRoleInstance envelope tags
	// https://github.com/open-telemetry/opentelemetry-specification/tree/main/specification/resource/semantic_conventions
	if serviceName, serviceNameExists := resourceAttributes.Get(conventions.AttributeServiceName); serviceNameExists {
		cloudRole := serviceName.Str()

		if serviceNamespace, serviceNamespaceExists := resourceAttributes.Get(conventions.AttributeServiceNamespace); serviceNamespaceExists {
			cloudRole = serviceNamespace.Str() + "." + cloudRole
		}

		envelope.Tags[contracts.CloudRole] = cloudRole
	}

	if serviceInstance, exists := resourceAttributes.Get(conventions.AttributeServiceInstanceID); exists {
		envelope.Tags[contracts.CloudRoleInstance] = serviceInstance.Str()
	}
}

// Applies instrumentation values to data properties
func applyInstrumentationScopeValueToDataProperties(dataProperties map[string]string, instrumentationScope pcommon.InstrumentationScope) {
	// Copy the instrumentation properties
	if instrumentationScope.Name() != "" {
		dataProperties[instrumentationLibraryName] = instrumentationScope.Name()
	}

	if instrumentationScope.Version() != "" {
		dataProperties[instrumentationLibraryVersion] = instrumentationScope.Version()
	}
}

// Applies attributes as Application Insights properties
func setAttributesAsProperties(attributeMap pcommon.Map, properties map[string]string) {
	attributeMap.Range(func(k string, v pcommon.Value) bool {
		properties[k] = v.AsString()
		return true
	})
}
