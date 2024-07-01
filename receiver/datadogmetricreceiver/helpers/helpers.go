package helpers

import (
	"errors"
	"strings"
	"time"
metricsV2 "github.com/DataDog/agent-payload/v5/gogen"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	scopeName    = "mw"
	scopeVersion = "v0.0.1"
)

const (
	datadogMetricTypeCount = int32(metricsV2.MetricPayload_COUNT)
	datadogMetricTypeGauge = int32(metricsV2.MetricPayload_GAUGE)
	datadogMetricTypeRate  = int32(metricsV2.MetricPayload_RATE)

	datadogAPIKeyHeader = "Dd-Api-Key"
)


type CommonResourceAttributes struct {
	Origin   string
	ApiKey   string
	MwSource string
	Host     string
}

func CalculateCreateTime(creationTimestamp int64) int64 {
	currentTime := time.Now()
	milliseconds := (currentTime.UnixNano() / int64(time.Millisecond)) * 1000000
	createtime := (int64(milliseconds/1000000000) - creationTimestamp)
	return createtime
}

func GetMillis() int64 {
	currentTime := time.Now()
	milliseconds := (currentTime.UnixNano() / int64(time.Millisecond)) * 1000000
	return milliseconds
}

// NewErrNoMetricsInPayload creates a new error indicating no metrics found in the payload with the given message.
// If message is empty, a default error message is used.
func NewErrNoMetricsInPayload(message string) error {
	if message == "" {
		message = "no metrics found in payload"
	}
	return errors.New(message)
}

func SetMetricResourceAttributes(attributes pcommon.Map,
	cra CommonResourceAttributes) {
	if cra.Origin != "" {
		attributes.PutStr("mw.client_origin", cra.Origin)
	}
	if cra.ApiKey != "" {
		attributes.PutStr("mw.account_key", cra.ApiKey)
	}
	if cra.MwSource != "" {
		attributes.PutStr("mw_source", cra.MwSource)
	}
	if cra.Host != "" {
		attributes.PutStr("host.id", cra.Host)
		attributes.PutStr("host.name", cra.Host)
	}
}

func AppendInstrScope(rm *pmetric.ResourceMetrics) pmetric.ScopeMetrics {
	scopeMetrics := rm.ScopeMetrics().AppendEmpty()
	instrumentationScope := scopeMetrics.Scope()
	instrumentationScope.SetName(scopeName)
	instrumentationScope.SetVersion(scopeVersion)
	return scopeMetrics
}

func SkipDatadogMetrics(metricName string, metricType int32) bool {
	if strings.HasPrefix(metricName, "datadog") {
		return true
	}

	if strings.HasPrefix(metricName, "n_o_i_n_d_e_x.datadog") {
		return true
	}

	if metricType != datadogMetricTypeRate &&
		metricType != datadogMetricTypeGauge &&
		metricType != datadogMetricTypeCount {
		return true
	}
	return false
}
