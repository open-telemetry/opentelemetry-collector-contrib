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

package healthcheckextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"

import (
	"sync"
	"time"

	"go.opencensus.io/stats/view"
)

const (
	exporterFailureView = "exporter/send_failed_requests"
)

// healthCheckExporter is a struct implement the exporter interface in open census that could export metrics
type healthCheckExporter struct {
	mu                   sync.Mutex
	exporterFailureQueue []*view.Data
}

func newHealthCheckExporter() *healthCheckExporter {
	return &healthCheckExporter{}
}

// ExportView function could export the failure view to the queue
func (e *healthCheckExporter) ExportView(vd *view.Data) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if vd.View.Name == exporterFailureView {
		e.exporterFailureQueue = append(e.exporterFailureQueue, vd)
	}
}

func (e *healthCheckExporter) checkHealthStatus(exporterFailureThreshold int) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	return exporterFailureThreshold >= len(e.exporterFailureQueue)
}

// rotate function could rotate the error logs that expired the time interval
func (e *healthCheckExporter) rotate(interval time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()

	viewNum := len(e.exporterFailureQueue)
	currentTime := time.Now()
	for i := 0; i < viewNum; i++ {
		vd := e.exporterFailureQueue[0]
		if vd.Start.Add(interval).After(currentTime) {
			e.exporterFailureQueue = append(e.exporterFailureQueue, vd)
		}
		e.exporterFailureQueue = e.exporterFailureQueue[1:]
	}
}
