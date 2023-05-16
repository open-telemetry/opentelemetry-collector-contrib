// Copyright The OpenTelemetry Authors
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

package metricstransformprocessor

import (
	"context"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

type metricsGroupingTest struct {
	name       string // test name
	transforms []internalTransform
}

var (
	groupingTests = []metricsGroupingTest{
		{
			name: "metric_group_by_strict_name",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterStrict{include: "foo/metric"},
					Action:              Group,
					GroupResourceLabels: map[string]string{"resource.type": "foo"},
				},
			},
		},
		{
			name: "metric_group_regex_multiple_empty_resource",
			transforms: []internalTransform{
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("^container.(.*)$")},
					Action:              Group,
					GroupResourceLabels: map[string]string{"resource.type": "container"},
				},
				{
					MetricIncludeFilter: internalFilterRegexp{include: regexp.MustCompile("^k8s.pod.(.*)$")},
					Action:              Group,
					GroupResourceLabels: map[string]string{"resource.type": "k8s.pod"},
				},
			},
		},
	}
)

func TestMetricsGrouping(t *testing.T) {
	for _, useOTLP := range []bool{false, true} {
		for _, test := range groupingTests {
			t.Run(test.name, func(t *testing.T) {
				next := new(consumertest.MetricsSink)

				p := &metricsTransformProcessor{
					transforms:               test.transforms,
					logger:                   zap.NewExample(),
					otlpDataModelGateEnabled: useOTLP,
				}

				mtp, err := processorhelper.NewMetricsProcessor(
					context.Background(),
					processortest.NewNopCreateSettings(),
					&Config{},
					next, p.processMetrics, processorhelper.WithCapabilities(consumerCapabilities))
				require.NoError(t, err)

				caps := mtp.Capabilities()
				assert.Equal(t, true, caps.MutatesData)

				input, err := golden.ReadMetrics(filepath.Join("testdata", "operation_group", test.name+"_in.yaml"))
				require.NoError(t, err)
				expected, err := golden.ReadMetrics(filepath.Join("testdata", "operation_group", test.name+"_out.yaml"))
				require.NoError(t, err)

				cErr := mtp.ConsumeMetrics(context.Background(), input)
				assert.NoError(t, cErr)

				got := next.AllMetrics()
				require.Equal(t, 1, len(got))
				require.NoError(t, pmetrictest.CompareMetrics(expected, got[0], pmetrictest.IgnoreMetricValues()))

				assert.NoError(t, mtp.Shutdown(context.Background()))
			})
		}
	}
}
