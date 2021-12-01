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

package coralogixexporter

import (
	"context"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestLoadConfig(t *testing.T) {
	t.Logf("CORALOGIX CONFIG CHECK : Start To Test")

	factories, _ := componenttest.NopFactories()
	factory := NewFactory()
	factories.Exporters[typestr] = factory
	t.Log("new exporter " + typestr)
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "example", "config.yaml"), factories)
	if err != nil {
		t.Logf("Config file with errors ")
		t.Log(err)
	}
	apiConfig := cfg.Exporters[config.NewComponentID(typestr)].(*Config)
	// assert.Equal(t, apiConfig, factory.CreateDefaultConfig())
	assert.Equal(t, apiConfig, &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID("coralogix")),
		QueueSettings:    exporterhelper.DefaultQueueSettings(),
		RetrySettings:    exporterhelper.DefaultRetrySettings(),
		Endpoint:         "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		PrivateKey:       "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		AppName:          "APP_NAME",
		SubSystem:        "SUBSYSTEM_NAME",
	})

	// t.Log(string(apiConfig))
	t.Logf("CORALOGIX CONFIG CHECK : END To Test")
}

func TestExporter(t *testing.T) {
	t.Logf("CORALOGIX EXPORTER CHECK : START To Test")

	factories, _ := componenttest.NopFactories()
	factory := NewFactory()
	factories.Exporters[typestr] = factory
	t.Log("new exporter " + typestr)
	cfg, _ := configtest.LoadConfigAndValidate(path.Join(".", "example", "config.yaml"), factories)

	apiConfig := cfg.Exporters[config.NewComponentID(typestr)].(*Config)
	params := componenttest.NewNopExporterCreateSettings()

	te := NewCoralogixExporter(apiConfig, params)
	assert.NotNil(t, te, "failed to create trace exporter")

	traceID := pdata.NewTraceID([16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15})
	spanID := pdata.NewSpanID([8]byte{0, 1, 2, 3, 4, 5, 6, 7})
	td := pdata.NewTraces()
	span := td.ResourceSpans().AppendEmpty().InstrumentationLibrarySpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)

	err := te.tracesPusher(context.Background(), td)
	assert.Nil(t, err)

	t.Logf("CORALOGIX EXPORTER CHECK : END To Test")

}
