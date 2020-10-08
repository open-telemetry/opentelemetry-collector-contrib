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

package loadbalancingexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.uber.org/zap"
)

func TestProcessorGetsCreatedWithValidConfiguration(t *testing.T) {
	// prepare
	factory := NewFactory()
	creationParams := component.ExporterCreateParams{Logger: zap.NewNop()}
	cfg := &Config{
		ExporterSettings: configmodels.ExporterSettings{
			NameVal: "loadbalancing",
			TypeVal: "loadbalancing",
		},
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: []string{"endpoint-1"}},
		},
	}

	// test
	exp, err := factory.CreateTraceExporter(context.Background(), creationParams, cfg)

	// verify
	assert.Nil(t, err)
	assert.NotNil(t, exp)
}
