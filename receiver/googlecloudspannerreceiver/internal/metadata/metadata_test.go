// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	labelName       = "LabelName"
	labelColumnName = "LabelColumnName"

	stringValue             = "stringValue"
	int64Value              = int64(64)
	float64Value            = float64(64.64)
	defaultNullFloat64Value = float64(0)
	boolValue               = true

	metricName       = "metricName"
	metricColumnName = "metricColumnName"
	metricDataType   = pmetric.MetricTypeGauge
	metricUnit       = "metricUnit"
	metricNamePrefix = "metricNamePrefix-"

	timestampColumnName = "INTERVAL_END"
)
