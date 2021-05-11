package awskinesisexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
)

func TestMetricTenants(t *testing.T) {
	t.Parallel()
	tcs := []struct {
		name            string
		svcKey          string
		envKey          string
		svc1            string
		env1            string
		svc2            string
		env2            string
		expectedTenants []string
	}{
		{
			name:            "two valid tenants are returned",
			svcKey:          serviceKey,
			envKey:          envKey,
			svc1:            "pineapple",
			env1:            "ddev",
			svc2:            "not-pineapple",
			env2:            "ddev",
			expectedTenants: []string{"pineapple_ddev", "not-pineapple_ddev"},
		},
		{
			name:            "no duplicate tenants returned",
			svcKey:          serviceKey,
			envKey:          envKey,
			svc1:            "pineapple",
			env1:            "ddev",
			svc2:            "pineapple",
			env2:            "ddev",
			expectedTenants: []string{"pineapple_ddev"},
		},
		{
			name:            "invalid tenants not returned",
			svcKey:          "bad",
			envKey:          "bad",
			svc1:            "pineapple",
			env1:            "ddev",
			svc2:            "not-pineapple",
			env2:            "ddev",
			expectedTenants: []string{},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			td := pdata.NewMetrics()
			td.ResourceMetrics().Resize(2)
			td.ResourceMetrics().At(0).Resource().Attributes().InsertString(tc.svcKey, tc.svc1)
			td.ResourceMetrics().At(0).Resource().Attributes().InsertString(tc.envKey, tc.env1)
			td.ResourceMetrics().At(1).Resource().Attributes().InsertString(tc.svcKey, tc.svc2)
			td.ResourceMetrics().At(1).Resource().Attributes().InsertString(tc.envKey, tc.env2)

			assert.Equal(t, tc.expectedTenants, metricTenants(td))
		})

	}
}

func TestTraceTenants(t *testing.T) {
	t.Parallel()
	tcs := []struct {
		name            string
		svcKey          string
		envKey          string
		svc1            string
		env1            string
		svc2            string
		env2            string
		expectedTenants []string
	}{
		{
			name:            "two valid tenants are returned",
			svcKey:          serviceKey,
			envKey:          envKey,
			svc1:            "pineapple",
			env1:            "ddev",
			svc2:            "not-pineapple",
			env2:            "ddev",
			expectedTenants: []string{"pineapple_ddev", "not-pineapple_ddev"},
		},
		{
			name:            "no duplicate tenants returned",
			svcKey:          serviceKey,
			envKey:          envKey,
			svc1:            "pineapple",
			env1:            "ddev",
			svc2:            "pineapple",
			env2:            "ddev",
			expectedTenants: []string{"pineapple_ddev"},
		},
		{
			name:            "invalid tenants not returned",
			svcKey:          "bad",
			envKey:          "bad",
			svc1:            "pineapple",
			env1:            "ddev",
			svc2:            "not-pineapple",
			env2:            "ddev",
			expectedTenants: []string{},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			td := pdata.NewTraces()
			td.ResourceSpans().Resize(2)
			td.ResourceSpans().At(0).Resource().Attributes().InsertString(tc.svcKey, tc.svc1)
			td.ResourceSpans().At(0).Resource().Attributes().InsertString(tc.envKey, tc.env1)
			td.ResourceSpans().At(1).Resource().Attributes().InsertString(tc.svcKey, tc.svc2)
			td.ResourceSpans().At(1).Resource().Attributes().InsertString(tc.envKey, tc.env2)

			assert.Equal(t, tc.expectedTenants, traceTenants(td))
		})

	}
}
