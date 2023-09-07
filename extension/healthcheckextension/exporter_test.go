// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckextension

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opencensus.io/stats/view"
)

func TestHealthCheckExporter_ExportView(t *testing.T) {
	exporter := &healthCheckExporter{}
	newView := view.View{Name: exporterFailureView}
	vd := &view.Data{
		View:  &newView,
		Start: time.Time{},
		End:   time.Time{},
		Rows:  nil,
	}
	exporter.ExportView(vd)
	assert.Equal(t, 1, len(exporter.exporterFailureQueue))
}

func TestHealthCheckExporter_rotate(t *testing.T) {
	exporter := &healthCheckExporter{}
	currentTime := time.Now()
	time1 := currentTime.Add(-10 * time.Minute)
	time2 := currentTime.Add(-3 * time.Minute)
	newView := view.View{Name: exporterFailureView}
	vd1 := &view.Data{
		View:  &newView,
		Start: time1,
		End:   currentTime,
		Rows:  nil,
	}
	vd2 := &view.Data{
		View:  &newView,
		Start: time2,
		End:   currentTime,
		Rows:  nil,
	}
	exporter.ExportView(vd1)
	exporter.ExportView(vd2)
	assert.Equal(t, 2, len(exporter.exporterFailureQueue))
	exporter.rotate(5 * time.Minute)
	assert.Equal(t, 1, len(exporter.exporterFailureQueue))
}
