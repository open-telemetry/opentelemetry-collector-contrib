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
