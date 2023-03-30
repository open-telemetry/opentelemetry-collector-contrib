// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

func resourceSignature(attributes pcommon.Map) string {
	job, _ := extractJob(attributes)
	instance, _ := extractInstance(attributes)
	if job == "" || instance == "" {
		return ""
	}

	return job + separatorString + instance
}

func extractInstance(attributes pcommon.Map) (string, bool) {
	// Map service.instance.id to instance
	if inst, ok := attributes.Get(conventions.AttributeServiceInstanceID); ok {
		return inst.AsString(), true
	}
	return "", false
}

func extractJob(attributes pcommon.Map) (string, bool) {
	// Map service.name + service.namespace to job
	if serviceName, ok := attributes.Get(conventions.AttributeServiceName); ok {
		job := serviceName.AsString()
		if serviceNamespace, ok := attributes.Get(conventions.AttributeServiceNamespace); ok {
			job = fmt.Sprintf("%s/%s", serviceNamespace.AsString(), job)
		}
		return job, true
	}
	return "", false
}
