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

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"errors"
	"strconv"
	"strings"

	"github.com/prometheus/prometheus/model/labels"
)

const (
	metricsSuffixCount  = "_count"
	metricsSuffixBucket = "_bucket"
	metricsSuffixSum    = "_sum"
	metricSuffixTotal   = "_total"
	metricSuffixInfo    = "_info"
	startTimeMetricName = "process_start_time_seconds"
	scrapeUpMetricName  = "up"

	transport  = "http"
	dataformat = "prometheus"
)

var (
	trimmableSuffixes     = []string{metricsSuffixBucket, metricsSuffixCount, metricsSuffixSum, metricSuffixTotal, metricSuffixInfo}
	errNoDataToBuild      = errors.New("there's no data to build")
	errNoBoundaryLabel    = errors.New("given metricType has no BucketLabel or QuantileLabel")
	errEmptyBoundaryLabel = errors.New("BucketLabel or QuantileLabel is empty")
	errMetricNameNotFound = errors.New("metricName not found from labels")
	errTransactionAborted = errors.New("transaction aborted")
	errNoJobInstance      = errors.New("job or instance cannot be found from labels")
	errNoStartTimeMetrics = errors.New("process_start_time_seconds metric is missing")
)

// dpgSignature is used to create a key for data complexValue belong to a same group of a metric family
func dpgSignature(orderedKnownLabelKeys []string, ls labels.Labels) string {
	size := 0
	for _, k := range orderedKnownLabelKeys {
		v := ls.Get(k)
		if v == "" {
			continue
		}
		// 2 enclosing quotes + 1 equality sign = 3 extra chars.
		// Note: if any character in the label value requires escaping,
		// we'll need more space than that, which will lead to some
		// extra allocation.
		size += 3 + len(k) + len(v)
	}
	sign := make([]byte, 0, size)
	for _, k := range orderedKnownLabelKeys {
		v := ls.Get(k)
		if v == "" {
			continue
		}
		sign = strconv.AppendQuote(sign, k+"="+v)
	}
	return string(sign)
}

func normalizeMetricName(name string) string {
	for _, s := range trimmableSuffixes {
		if strings.HasSuffix(name, s) && name != s {
			return strings.TrimSuffix(name, s)
		}
	}
	return name
}

func isInternalMetric(metricName string) bool {
	if metricName == scrapeUpMetricName || strings.HasPrefix(metricName, "scrape_") {
		return true
	}
	return false
}
