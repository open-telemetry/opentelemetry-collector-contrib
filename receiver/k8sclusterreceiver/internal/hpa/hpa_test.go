// Copyright 2020 OpenTelemetry Authors
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

package hpa

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestHPAMetrics(t *testing.T) {
	hpa := testutils.NewHPA("1")

	md := GetMetrics(receivertest.NewNopCreateSettings(), hpa)

	require.Equal(t, 1, md.ResourceMetrics().Len())
	rm := md.ResourceMetrics().At(0)
	assert.Equal(t,
		map[string]any{
			"k8s.hpa.uid":        "test-hpa-1-uid",
			"k8s.hpa.name":       "test-hpa-1",
			"k8s.namespace.name": "test-namespace",
		},
		rm.Resource().Attributes().AsRaw())

	require.Equal(t, 1, rm.ScopeMetrics().Len())
	sms := rm.ScopeMetrics().At(0)
	require.Equal(t, 4, sms.Metrics().Len())
	sms.Metrics().Sort(func(a, b pmetric.Metric) bool {
		return a.Name() < b.Name()
	})
	testutils.AssertMetricInt(t, sms.Metrics().At(0), "k8s.hpa.current_replicas", pmetric.MetricTypeGauge, 5)
	testutils.AssertMetricInt(t, sms.Metrics().At(1), "k8s.hpa.desired_replicas", pmetric.MetricTypeGauge, 7)
	testutils.AssertMetricInt(t, sms.Metrics().At(2), "k8s.hpa.max_replicas", pmetric.MetricTypeGauge, 10)
	testutils.AssertMetricInt(t, sms.Metrics().At(3), "k8s.hpa.min_replicas", pmetric.MetricTypeGauge, 2)
}
