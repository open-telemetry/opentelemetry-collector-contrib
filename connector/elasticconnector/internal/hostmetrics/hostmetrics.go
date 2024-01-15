package hostmetrics

import (
	"fmt"
	"path"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TransformForElasticSystemMetricsCompatibilty(scopeMetrics pmetric.ScopeMetrics) error {
	scopeName := scopeMetrics.Scope().Name()
	switch scraper := path.Base(scopeName); scraper {
	case "cpu":
		return transformCPUMetrics(scopeMetrics.Metrics())
	default:
		return fmt.Errorf("no matching transform function found for scope '%s'", scopeName)
	}
}
