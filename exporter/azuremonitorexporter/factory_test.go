// Copyright 2019, OpenTelemetry Authors
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

package azuremonitorexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector/config/configerror"
	"go.uber.org/zap"
)

func TestExporterTypeKey(t *testing.T) {
	factory := Factory{}

	assert.Equal(t, typeStr, factory.Type())
}

// factory.CreateDefaultConfig() exercised in config_test.go

func TestCreateMetricsExporter(t *testing.T) {
	factory := Factory{}

	exporter, err := factory.CreateMetricsExporter(zap.NewNop(), &Config{})

	// unsupported
	assert.Nil(t, exporter)
	assert.Equal(t, configerror.ErrDataTypeIsNotSupported, err)
}

func TestCreateTraceExporter(t *testing.T) {
	// mock transport channel creation
	transportChannel := &mockTransportChannel{}
	factory := Factory{
		TransportChannel: transportChannel,
	}

	exporter, err := factory.CreateTraceExporter(zap.NewNop(), factory.CreateDefaultConfig())

	assert.Nil(t, err)
	assert.NotNil(t, exporter)

	// default transport channel creation, if nothing specified
	factory = Factory{}
	exporter, err = factory.CreateTraceExporter(zap.NewNop(), factory.CreateDefaultConfig())
	assert.NotNil(t, exporter)
	assert.Nil(t, err)
	assert.NotNil(t, factory.TransportChannel)
}
