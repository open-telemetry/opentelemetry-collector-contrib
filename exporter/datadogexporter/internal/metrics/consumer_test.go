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

package metrics

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/attributes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/translator"
)

func newTestCache() *translator.TTLCache {
	cache := translator.NewTTLCache(1800, 3600)
	return cache
}

type testProvider string

func (t testProvider) Hostname(context.Context) (string, error) {
	return string(t), nil
}

func newTranslator(logger *zap.Logger, cfg config.MetricsConfig) *translator.Translator {
	return translator.New(newTestCache(), logger, cfg, testProvider("fallbackHostname"))
}

func TestRunningMetrics(t *testing.T) {
	ms := pdata.NewMetrics()
	rms := ms.ResourceMetrics()

	rm := rms.AppendEmpty()
	resAttrs := rm.Resource().Attributes()
	resAttrs.Insert(attributes.AttributeDatadogHostname, pdata.NewAttributeValueString("resource-hostname-1"))

	rm = rms.AppendEmpty()
	resAttrs = rm.Resource().Attributes()
	resAttrs.Insert(attributes.AttributeDatadogHostname, pdata.NewAttributeValueString("resource-hostname-1"))

	rm = rms.AppendEmpty()
	resAttrs = rm.Resource().Attributes()
	resAttrs.Insert(attributes.AttributeDatadogHostname, pdata.NewAttributeValueString("resource-hostname-2"))

	rms.AppendEmpty()

	cfg := config.MetricsConfig{}
	logger, _ := zap.NewProduction()
	tr := newTranslator(logger, cfg)

	ctx := context.Background()
	consumer := NewConsumer()
	tr.MapMetrics(ctx, ms, consumer)

	runningHostnames := []string{}
	for _, metric := range consumer.runningMetrics(0, component.BuildInfo{}) {
		if metric.Host != nil {
			runningHostnames = append(runningHostnames, *metric.Host)
		}
	}

	assert.ElementsMatch(t,
		runningHostnames,
		[]string{"fallbackHostname", "resource-hostname-1", "resource-hostname-2"},
	)

}
