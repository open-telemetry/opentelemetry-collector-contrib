// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type graphiteFormatter struct {
	template sourceFormat
	replacer *strings.Replacer
}

const (
	graphiteMetricNamePlaceholder = "_metric_"
)

// newGraphiteFormatter creates new formatter for given SourceFormat template
func newGraphiteFormatter(template string) graphiteFormatter {
	r := regexp.MustCompile(sourceRegex)

	sf := newSourceFormat(r, template)

	return graphiteFormatter{
		template: sf,
		replacer: strings.NewReplacer(`.`, `_`, ` `, `_`),
	}
}

// escapeGraphiteString replaces dot and space using replacer,
// as dot is special character for graphite format
func (gf *graphiteFormatter) escapeGraphiteString(value string) string {
	return gf.replacer.Replace(value)
}

// format returns metric name basing on template for given fields nas metric name
func (gf *graphiteFormatter) format(f fields, metricName string) string {
	s := gf.template
	labels := make([]any, 0, len(s.matches))

	for _, matchset := range s.matches {
		if matchset == graphiteMetricNamePlaceholder {
			labels = append(labels, gf.escapeGraphiteString(metricName))
		} else {
			attr, ok := f.orig.Get(matchset)
			var value string
			if ok {
				value = attr.AsString()
			} else {
				value = ""
			}
			labels = append(labels, gf.escapeGraphiteString(value))
		}
	}

	return fmt.Sprintf(s.template, labels...)
}

// numberRecord converts NumberDataPoint to graphite metric string
// with additional information from fields
func (gf *graphiteFormatter) numberRecord(fs fields, name string, dataPoint pmetric.NumberDataPoint) string {
	switch dataPoint.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		return fmt.Sprintf("%s %g %d",
			gf.format(fs, name),
			dataPoint.DoubleValue(),
			dataPoint.Timestamp()/pcommon.Timestamp(time.Second),
		)
	case pmetric.NumberDataPointValueTypeInt:
		return fmt.Sprintf("%s %d %d",
			gf.format(fs, name),
			dataPoint.IntValue(),
			dataPoint.Timestamp()/pcommon.Timestamp(time.Second),
		)
	case pmetric.NumberDataPointValueTypeEmpty:
		return ""
	}
	return ""
}

// metric2String returns stringified metricPair
func (gf *graphiteFormatter) metric2String(record metricPair) string {
	var nextLines []string
	fs := newFields(record.attributes)
	name := record.metric.Name()
	//exhaustive:enforce
	switch record.metric.Type() {
	case pmetric.MetricTypeGauge:
		dps := record.metric.Gauge().DataPoints()
		nextLines = make([]string, 0, dps.Len())
		for i := 0; i < dps.Len(); i++ {
			nextLines = append(nextLines, gf.numberRecord(fs, name, dps.At(i)))
		}
	case pmetric.MetricTypeSum:
		dps := record.metric.Sum().DataPoints()
		nextLines = make([]string, 0, dps.Len())
		for i := 0; i < dps.Len(); i++ {
			nextLines = append(nextLines, gf.numberRecord(fs, name, dps.At(i)))
		}
	// Skip complex metrics
	case pmetric.MetricTypeHistogram:
	case pmetric.MetricTypeSummary:
	case pmetric.MetricTypeEmpty:
	case pmetric.MetricTypeExponentialHistogram:
	}

	return strings.Join(nextLines, "\n")
}
