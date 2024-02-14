// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
