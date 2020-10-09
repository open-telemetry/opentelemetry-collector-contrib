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

package sapmexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

func TestTrackerAddSpans(t *testing.T) {
	tracker := NewTracker(&Config{
		Correlation: CorrelationConfig{
			Enabled: true,
		},
		AccessToken: "",
	}, component.ExporterCreateParams{
		Logger: zap.NewNop(),
	})

	traces := pdata.NewTraces()
	spans := pdata.NewResourceSpans()
	spans.InitEmpty()
	spans.Resource().InitEmpty()
	attr := spans.Resource().Attributes()
	attr.InsertString("host.name", "localhost")
	traces.ResourceSpans().Append(spans)

	// Add empty first, should ignore.
	tracker.AddSpans(context.Background(), pdata.NewTraces())
	assert.Nil(t, tracker.correlation)
	assert.Nil(t, tracker.traceTracker)

	tracker.AddSpans(context.Background(), traces)

	assert.NotNil(t, tracker.correlation, "correlation context should be set")
	assert.NotNil(t, tracker.traceTracker, "trace tracker should be set")

	tracker.Shutdown()
}
