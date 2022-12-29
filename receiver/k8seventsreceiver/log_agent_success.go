// Copyright  OpenTelemetry Authors
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

package k8seventsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver"

import (
	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
)

// k8sEventToLogRecord converts Kubernetes event to plog.LogRecordSlice and adds the resource attributes.
func logk8sClusterSuccessMessage(logger *zap.Logger) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()

	resourceAttrs := rl.Resource().Attributes()
	resourceAttrs.EnsureCapacity(1)
	resourceAttrs.PutStr(semconv.AttributeK8SClusterName, "unknown")

	// A static message to show that the middleware agent is running
	lr.Body().SetStr("Middleware Agent installed successfully")

	// Log added as type "info"
	lr.SetSeverityNumber(9)
	// lr.SetSeverityText(ev.Type)

	return ld
}
