package kinesisexporter

import (
	"fmt"

	"go.opentelemetry.io/collector/consumer/pdata"
)

const (
	serviceKey = "service_id"
	envKey     = "environment"
)

func metricTenants(md pdata.Metrics) []string {
	m := make(map[string]struct{})
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		if t := resourceTenant(md.ResourceMetrics().At(i).Resource()); len(t) > 0 {
			if _, inMap := m[t]; !inMap {
				m[t] = struct{}{}
			}
		}
	}
	return tenantList(m)
}

func traceTenants(td pdata.Traces) []string {
	m := make(map[string]struct{})
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		if t := resourceTenant(td.ResourceSpans().At(i).Resource()); len(t) > 0 {
			if _, inMap := m[t]; !inMap {
				m[t] = struct{}{}
			}
		}
	}
	return tenantList(m)
}

func resourceTenant(r pdata.Resource) string {
	if svcId, svcExists := r.Attributes().Get(serviceKey); svcExists {
		if env, envExists := r.Attributes().Get(envKey); envExists {
			return fmt.Sprintf("%s_%s", svcId.StringVal(), env.StringVal())
		}
	}

	return ""
}

func tenantList(tenants map[string]struct{}) []string {
	ret := make([]string, len(tenants))
	i := 0
	for k := range tenants {
		ret[i] = k
		i++
	}
	return ret
}
