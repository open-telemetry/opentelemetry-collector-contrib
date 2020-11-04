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

package cascadingfilterprocessor

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"

	cfconfig "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cascadingfilterprocessor/config"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Processors[factory.Type()] = factory

	cfg, err := configtest.LoadConfig(path.Join(".", "testdata", "cascading_filter_config.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	minDurationValue := 9 * time.Second
	minSpansValue := 10
	minErrorsValue := 2
	probFilteringRatio := float32(0.1)
	probFilteringRate := int32(100)
	namePatternValue := "foo.*"
	healthCheckNamePatternValue := "health.*"

	id1 := config.NewComponentIDWithName("cascading_filter", "1")
	ps1 := config.NewProcessorSettings(id1)
	assert.Equal(t, cfg.Processors[id1],
		&cfconfig.Config{
			DecisionWait:               30 * time.Second,
			SpansPerSecond:             0,
			NumTraces:                  100000,
			ProcessorSettings:          &ps1,
			ProbabilisticFilteringRate: &probFilteringRate,
			TraceRejectCfgs: []cfconfig.TraceRejectCfg{
				{
					Name:        "healthcheck-rule",
					NamePattern: &healthCheckNamePatternValue,
				},
			},
			TraceAcceptCfgs: []cfconfig.TraceAcceptCfg{
				{
					Name:           "include-errors",
					SpansPerSecond: 200,
					PropertiesCfg: cfconfig.PropertiesCfg{
						MinNumberOfErrors: &minErrorsValue,
					},
				},
				{
					Name:           "include-long-traces",
					SpansPerSecond: 300,
					PropertiesCfg: cfconfig.PropertiesCfg{
						MinNumberOfSpans: &minSpansValue,
					},
				},
				{
					Name:           "include-high-latency",
					SpansPerSecond: 400,
					PropertiesCfg: cfconfig.PropertiesCfg{
						MinDuration: &minDurationValue,
					},
				},
			},
		})

	id2 := config.NewComponentIDWithName("cascading_filter", "2")
	ps2 := config.NewProcessorSettings(id2)
	assert.Equal(t, cfg.Processors[id2],
		&cfconfig.Config{
			ProcessorSettings:           &ps2,
			DecisionWait:                10 * time.Second,
			NumTraces:                   100,
			ExpectedNewTracesPerSec:     10,
			SpansPerSecond:              1000,
			ProbabilisticFilteringRatio: &probFilteringRatio,
			TraceRejectCfgs: []cfconfig.TraceRejectCfg{
				{
					Name:        "healthcheck-rule",
					NamePattern: &healthCheckNamePatternValue,
				},
				{
					Name:                "remove-all-traces-with-healthcheck-service",
					NamePattern:         nil,
					NumericAttributeCfg: nil,
					StringAttributeCfg: &cfconfig.StringAttributeCfg{
						Key:    "service.name",
						Values: []string{"healthcheck"},
					},
				},
			},
			TraceAcceptCfgs: []cfconfig.TraceAcceptCfg{
				{
					Name: "test-policy-1",
				},
				{
					Name:                "test-policy-2",
					NumericAttributeCfg: &cfconfig.NumericAttributeCfg{Key: "key1", MinValue: 50, MaxValue: 100},
				},
				{
					Name:               "test-policy-3",
					StringAttributeCfg: &cfconfig.StringAttributeCfg{Key: "key2", Values: []string{"value1", "value2"}},
				},
				{
					Name:           "test-policy-4",
					SpansPerSecond: 35,
				},
				{
					Name:           "test-policy-5",
					SpansPerSecond: 123,
					NumericAttributeCfg: &cfconfig.NumericAttributeCfg{
						Key: "key1", MinValue: 50, MaxValue: 100},
					InvertMatch: true,
				},
				{
					Name:           "test-policy-6",
					SpansPerSecond: 50,

					PropertiesCfg: cfconfig.PropertiesCfg{MinDuration: &minDurationValue},
				},
				{
					Name: "test-policy-7",
					PropertiesCfg: cfconfig.PropertiesCfg{
						NamePattern:       &namePatternValue,
						MinDuration:       &minDurationValue,
						MinNumberOfSpans:  &minSpansValue,
						MinNumberOfErrors: &minErrorsValue,
					},
				},
				{
					Name:           "everything_else",
					SpansPerSecond: -1,
				},
			},
		})
}
