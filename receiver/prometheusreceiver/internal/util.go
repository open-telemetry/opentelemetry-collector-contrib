// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/textparse"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	metricsSuffixCount  = "_count"
	metricsSuffixBucket = "_bucket"
	metricsSuffixSum    = "_sum"
	metricSuffixTotal   = "_total"
	metricSuffixInfo    = "_info"
	metricSuffixCreated = "_created"
	startTimeMetricName = "process_start_time_seconds"
	scrapeUpMetricName  = "up"

	transport  = "http"
	dataformat = "prometheus"
)

var (
	trimmableSuffixes     = []string{metricsSuffixBucket, metricsSuffixCount, metricsSuffixSum, metricSuffixTotal, metricSuffixInfo, metricSuffixCreated}
	errNoDataToBuild      = errors.New("there's no data to build")
	errNoBoundaryLabel    = errors.New("given metricType has no 'le' or 'quantile' label")
	errEmptyQuantileLabel = errors.New("'quantile' label on summary metric is missing or empty")
	errEmptyLeLabel       = errors.New("'le' label on histogram metric is missing or empty")
	errMetricNameNotFound = errors.New("metricName not found from labels")
	errTransactionAborted = errors.New("transaction aborted")
	errNoJobInstance      = errors.New("job or instance cannot be found from labels")

	notUsefulLabelsHistogram = sortString([]string{model.MetricNameLabel, model.InstanceLabel, model.SchemeLabel, model.MetricsPathLabel, model.JobLabel, model.BucketLabel})
	notUsefulLabelsSummary   = sortString([]string{model.MetricNameLabel, model.InstanceLabel, model.SchemeLabel, model.MetricsPathLabel, model.JobLabel, model.QuantileLabel})
	notUsefulLabelsOther     = sortString([]string{model.MetricNameLabel, model.InstanceLabel, model.SchemeLabel, model.MetricsPathLabel, model.JobLabel})
)

func sortString(strs []string) []string {
	sort.Strings(strs)
	return strs
}

func getSortedNotUsefulLabels(mType pmetric.MetricType) []string {
	switch mType {
	case pmetric.MetricTypeHistogram:
		return notUsefulLabelsHistogram
	case pmetric.MetricTypeSummary:
		return notUsefulLabelsSummary
	default:
		return notUsefulLabelsOther
	}
}

func timestampFromFloat64(ts float64) pcommon.Timestamp {
	secs := int64(ts)
	nanos := int64((ts - float64(secs)) * 1e9)
	return pcommon.Timestamp(secs*1e9 + nanos)
}

func timestampFromMs(timeAtMs int64) pcommon.Timestamp {
	return pcommon.Timestamp(timeAtMs * 1e6)
}

func getBoundary(metricType pmetric.MetricType, labels labels.Labels) (float64, error) {
	val := ""
	switch metricType {
	case pmetric.MetricTypeHistogram:
		val = labels.Get(model.BucketLabel)
		if val == "" {
			return 0, errEmptyLeLabel
		}
	case pmetric.MetricTypeSummary:
		val = labels.Get(model.QuantileLabel)
		if val == "" {
			return 0, errEmptyQuantileLabel
		}
	default:
		return 0, errNoBoundaryLabel
	}

	return strconv.ParseFloat(val, 64)
}

// convToMetricType returns the data type and if it is monotonic
func convToMetricType(metricType textparse.MetricType) (pmetric.MetricType, bool) {
	switch metricType {
	case textparse.MetricTypeCounter:
		// always use float64, as it's the internal data type used in prometheus
		return pmetric.MetricTypeSum, true
	// textparse.MetricTypeUnknown is converted to gauge by default to prevent Prometheus untyped metrics from being dropped
	case textparse.MetricTypeGauge, textparse.MetricTypeUnknown:
		return pmetric.MetricTypeGauge, false
	case textparse.MetricTypeHistogram:
		return pmetric.MetricTypeHistogram, true
	// dropping support for gaugehistogram for now until we have an official spec of its implementation
	// a draft can be found in: https://docs.google.com/document/d/1KwV0mAXwwbvvifBvDKH_LU1YjyXE_wxCkHNoCGq1GX0/edit#heading=h.1cvzqd4ksd23
	// case textparse.MetricTypeGaugeHistogram:
	//	return <pdata gauge histogram type>
	case textparse.MetricTypeSummary:
		return pmetric.MetricTypeSummary, true
	case textparse.MetricTypeInfo, textparse.MetricTypeStateset:
		return pmetric.MetricTypeSum, false
	default:
		// including: textparse.MetricTypeGaugeHistogram
		return pmetric.MetricTypeEmpty, false
	}
}

func normalizeMetricName(name string) string {
	for _, s := range trimmableSuffixes {
		if strings.HasSuffix(name, s) && name != s {
			return strings.TrimSuffix(name, s)
		}
	}
	return name
}
