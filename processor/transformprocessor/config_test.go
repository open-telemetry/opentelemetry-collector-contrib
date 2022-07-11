// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package transformprocessor

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"
)

func TestLoadingConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Processors[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)
	assert.NoError(t, err)
	require.NotNil(t, cfg)

	p0 := cfg.Processors[config.NewComponentID(typeStr)]
	assert.Equal(t, p0, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
		Traces: SignalConfig{
			Queries: []string{
				`set(name, "bear") where attributes["http.path"] == "/animal"`,
				`keep_keys(attributes, "http.method", "http.path")`,
			},

			functions: traces.DefaultFunctions(),
		},
		Metrics: SignalConfig{
			Queries: []string{
				`set(metric.name, "bear") where attributes["http.path"] == "/animal"`,
				`keep_keys(attributes, "http.method", "http.path")`,
			},

			functions: metrics.DefaultFunctions(),
		},
		Logs: SignalConfig{
			Queries: []string{
				`set(body, "bear") where attributes["http.path"] == "/animal"`,
				`keep_keys(attributes, "http.method", "http.path")`,
			},

			functions: logs.DefaultFunctions(),
		},
	})
}

func TestLoadingOperationsConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Processors[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "operations_config.yaml"), factories)
	assert.NoError(t, err)
	require.NotNil(t, cfg)

	p0 := cfg.Processors[config.NewComponentID(typeStr)]
	assert.Equal(t, p0, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
		Traces: SignalConfig{
			Queries: nil,
			Operations: []common.ParsedQuery{
				{
					Invocation: common.Invocation{
						Function: "set",
						Arguments: []common.Value{
							{
								Path: &common.Path{
									Fields: []common.Field{
										{
											Name: "name",
										},
									},
								},
							},
							{
								String: testhelper.Strp("bear"),
							},
						},
					},
					Condition: &common.Condition{
						Left: common.Value{
							Path: &common.Path{
								Fields: []common.Field{
									{
										Name:   "attributes",
										MapKey: testhelper.Strp("http.path"),
									},
								},
							},
						},
						Op: "==",
						Right: common.Value{
							String: testhelper.Strp("/animal"),
						},
					},
				},
				{
					Invocation: common.Invocation{
						Function: "keep_keys",
						Arguments: []common.Value{
							{
								Path: &common.Path{
									Fields: []common.Field{
										{
											Name: "attributes",
										},
									},
								},
							},
							{
								String: testhelper.Strp("http.method"),
							},
							{
								String: testhelper.Strp("http.path"),
							},
						},
					},
				},
			},

			functions: traces.DefaultFunctions(),
		},
		Metrics: SignalConfig{
			Queries: nil,
			Operations: []common.ParsedQuery{
				{
					Invocation: common.Invocation{
						Function: "set",
						Arguments: []common.Value{
							{
								Path: &common.Path{
									Fields: []common.Field{
										{
											Name: "metric",
										},
										{
											Name: "name",
										},
									},
								},
							},
							{
								String: testhelper.Strp("bear"),
							},
						},
					},
					Condition: &common.Condition{
						Left: common.Value{
							Path: &common.Path{
								Fields: []common.Field{
									{
										Name:   "attributes",
										MapKey: testhelper.Strp("http.path"),
									},
								},
							},
						},
						Op: "==",
						Right: common.Value{
							String: testhelper.Strp("/animal"),
						},
					},
				},
				{
					Invocation: common.Invocation{
						Function: "keep_keys",
						Arguments: []common.Value{
							{
								Path: &common.Path{
									Fields: []common.Field{
										{
											Name: "attributes",
										},
									},
								},
							},
							{
								String: testhelper.Strp("http.method"),
							},
							{
								String: testhelper.Strp("http.path"),
							},
						},
					},
				},
			},

			functions: metrics.DefaultFunctions(),
		},
		Logs: SignalConfig{
			Queries: nil,
			Operations: []common.ParsedQuery{
				{
					Invocation: common.Invocation{
						Function: "set",
						Arguments: []common.Value{
							{
								Path: &common.Path{
									Fields: []common.Field{
										{
											Name: "body",
										},
									},
								},
							},
							{
								String: testhelper.Strp("bear"),
							},
						},
					},
					Condition: &common.Condition{
						Left: common.Value{
							Path: &common.Path{
								Fields: []common.Field{
									{
										Name:   "attributes",
										MapKey: testhelper.Strp("http.path"),
									},
								},
							},
						},
						Op: "==",
						Right: common.Value{
							String: testhelper.Strp("/animal"),
						},
					},
				},
				{
					Invocation: common.Invocation{
						Function: "keep_keys",
						Arguments: []common.Value{
							{
								Path: &common.Path{
									Fields: []common.Field{
										{
											Name: "attributes",
										},
									},
								},
							},
							{
								String: testhelper.Strp("http.method"),
							},
							{
								String: testhelper.Strp("http.path"),
							},
						},
					},
				},
			},

			functions: logs.DefaultFunctions(),
		},
	})
}

func TestLoadInvalidConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Processors[typeStr] = factory

	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "invalid_config_bad_syntax_trace.yaml"), factories)
	assert.Error(t, err)
	assert.NotNil(t, cfg)

	cfg, err = servicetest.LoadConfigAndValidate(filepath.Join("testdata", "invalid_config_unknown_function_trace.yaml"), factories)
	assert.Error(t, err)
	assert.NotNil(t, cfg)

	cfg, err = servicetest.LoadConfigAndValidate(filepath.Join("testdata", "invalid_config_bad_syntax_metric.yaml"), factories)
	assert.Error(t, err)
	assert.NotNil(t, cfg)

	cfg, err = servicetest.LoadConfigAndValidate(filepath.Join("testdata", "invalid_config_unknown_function_metric.yaml"), factories)
	assert.Error(t, err)
	assert.NotNil(t, cfg)

	cfg, err = servicetest.LoadConfigAndValidate(filepath.Join("testdata", "invalid_config_bad_syntax_log.yaml"), factories)
	assert.Error(t, err)
	assert.NotNil(t, cfg)

	cfg, err = servicetest.LoadConfigAndValidate(filepath.Join("testdata", "invalid_config_unknown_function_log.yaml"), factories)
	assert.Error(t, err)
	assert.NotNil(t, cfg)

	cfg, err = servicetest.LoadConfigAndValidate(filepath.Join("testdata", "invalid_config_both_syntax_type.yaml"), factories)
	assert.Error(t, err)
	assert.NotNil(t, cfg)

	cfg, err = servicetest.LoadConfigAndValidate(filepath.Join("testdata", "invalid_config_no_operations_or_queries.yaml"), factories)
	assert.Error(t, err)
	assert.NotNil(t, cfg)
}
