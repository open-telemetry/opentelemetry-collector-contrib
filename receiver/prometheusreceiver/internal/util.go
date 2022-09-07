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
	"strings"
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
	errNoBoundaryLabel    = errors.New("given metricType has no 'le' or 'quantile' label")
	errEmptyQuantileLabel = errors.New("'quantile' label on summary metric missing is empty")
	errEmptyLeLabel       = errors.New("'le' label on histogram metric id missing or empty")
	errMetricNameNotFound = errors.New("metricName not found from labels")
	errTransactionAborted = errors.New("transaction aborted")
	errNoJobInstance      = errors.New("job or instance cannot be found from labels")
	errNoStartTimeMetrics = errors.New("process_start_time_seconds metric is missing")
)

func normalizeMetricName(name string) string {
	for _, s := range trimmableSuffixes {
		if strings.HasSuffix(name, s) && name != s {
			return strings.TrimSuffix(name, s)
		}
	}
	return name
}
