// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package icingareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/icingareceiver"

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

// icingaParser supports the Parse method for parsing Icinga events.
type icingaParser struct {
	ctx      context.Context
	logger   *zap.Logger
	config   Config
	gauges   map[icingaMetricDescription]pmetric.ScopeMetrics
	counters map[icingaMetricDescription]pmetric.ScopeMetrics
}

type MetricType string

type icingaMetric struct {
	description icingaMetricDescription

	asFloat float64
	unit    string
}

type icingaMetricDescription struct {
	name       string
	metricType MetricType
	attrs      attribute.Set
}

const (
	CounterType MetricType = "counter"
	GaugeType   MetricType = "gauge"
)

var (
	numbersOnly    = regexp.MustCompile(`^[\.0-9]+`)
	nonNumbersOnly = regexp.MustCompile(`[^0-9\.]+`)
)

func (p *icingaParser) initialize() error {
	p.gauges = make(map[icingaMetricDescription]pmetric.ScopeMetrics)
	p.counters = make(map[icingaMetricDescription]pmetric.ScopeMetrics)

	return nil
}

// GetMetrics gets the metrics preparing for flushing and reset the state.
func (p *icingaParser) getMetrics() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()

	for _, metric := range p.gauges {
		metric.CopyTo(rm.ScopeMetrics().AppendEmpty())
	}

	for _, metric := range p.counters {
		metric.CopyTo(rm.ScopeMetrics().AppendEmpty())
	}

	return metrics
}

// Aggregate for each metric.
func (p *icingaParser) Aggregate(line string) error {
	parsedMetrics, err := p.parseMessageToMetric(line)
	if err != nil {
		return err
	}
	for _, parsedMetric := range parsedMetrics {
		switch parsedMetric.description.metricType {
		case GaugeType:
			p.gauges[parsedMetric.description] = buildGaugeMetric(parsedMetric, time.Now())

		case CounterType:
			_, ok := p.counters[parsedMetric.description]
			if !ok {
				p.counters[parsedMetric.description] = buildCounterMetric(parsedMetric, time.Now())
			} else {
				point := p.counters[parsedMetric.description].Metrics().At(0).Sum().DataPoints().At(0)
				point.SetIntVal(point.IntVal() + parsedMetric.counterValue())
			}

		}
	}
	return nil
}

type CheckResultResponse struct {
	Service     string      `json:"service"`
	Timestamp   float64     `json:"timestamp"`
	Host        string      `json:"host"`
	CheckResult CheckResult `json:"check_result"`
}

type CheckResult struct {
	State           float32  `json:"state"`
	PerformanceData []string `json:"performance_data"`
}

func (p *icingaParser) parseMessageToMetric(line string) ([]icingaMetric, error) {
	var checkResult CheckResultResponse
	json.Unmarshal([]byte(line), &checkResult)

	var kvs []attribute.KeyValue
	kvs = append(kvs, attribute.String(conventions.AttributeServiceInstanceID, checkResult.Host))

	var result []icingaMetric

	objectType := "service"
	if checkResult.Service == "" {
		objectType = "host"
	} else {
		kvs = append(kvs, attribute.String(conventions.AttributeServiceName, checkResult.Service))
	}

	result = append(result, icingaMetric{
		description: icingaMetricDescription{
			name:       "icinga.gauge." + objectType + ".state",
			metricType: GaugeType,
			attrs:      attribute.NewSet(kvs...),
		},
		asFloat: float64(checkResult.CheckResult.State),
	})

	kvs = append(kvs, attribute.String("state", fmt.Sprint(checkResult.CheckResult.State)))
	result = append(result, icingaMetric{
		description: icingaMetricDescription{
			name:       "icinga.counter." + objectType + ".check_result",
			metricType: CounterType,
			attrs:      attribute.NewSet(kvs...),
		},
		asFloat: float64(1),
	})

	result = p.getPerformanceMetrics(checkResult, kvs, result, objectType)

	return result, nil
}

func (p *icingaParser) getPerformanceMetrics(checkResult CheckResultResponse, kvs []attribute.KeyValue, result []icingaMetric, objectType string) []icingaMetric {
	perfData := parsePerformanceData(checkResult.CheckResult.PerformanceData)

	for key, dataPoint := range perfData {
		kvs = append(kvs, attribute.String("type", key))
		result = append(result, icingaMetric{
			description: icingaMetricDescription{
				name:       "icinga.gauge." + objectType + ".perf." + dataPoint.unitDisplayName,
				metricType: GaugeType,
				attrs:      attribute.NewSet(kvs...),
			},
			unit:    dataPoint.unit,
			asFloat: dataPoint.value,
		})

		histogramValues := p.getMatchingHistogramValues(checkResult.Service, checkResult.Host, key)
		for _, histogramValue := range histogramValues {
			value := 0.
			if histogramValue >= dataPoint.value {
				value = 1.
			}
			kvs = append(kvs, attribute.String("le", fmt.Sprintf("%f", histogramValue)))
			result = append(result, icingaMetric{
				description: icingaMetricDescription{
					name:       "icinga.histogram." + objectType + ".perf." + dataPoint.unitDisplayName,
					metricType: CounterType,
					attrs:      attribute.NewSet(kvs...),
				},
				unit:    dataPoint.unit,
				asFloat: value,
			})
		}

		if len(histogramValues) > 0 {
			kvs = append(kvs, attribute.String("le", "+Inf"))
			result = append(result, icingaMetric{
				description: icingaMetricDescription{
					name:       "icinga.histogram." + objectType + ".perf." + dataPoint.unitDisplayName,
					metricType: CounterType,
					attrs:      attribute.NewSet(kvs...),
				},
				unit:    dataPoint.unit,
				asFloat: float64(1),
			})
		}
	}
	return result
}

func (p *icingaParser) getMatchingHistogramValues(service string, host string, perfType string) []float64 {
	var result []float64
	for _, histogramConfig := range p.config.Histograms {
		if histogramConfig.Service != "" {
			match, _ := regexp.MatchString(histogramConfig.Service, service)
			if !match {
				continue
			}
		}
		if histogramConfig.Host != "" {
			match, _ := regexp.MatchString(histogramConfig.Host, host)
			if !match {
				continue
			}
		}
		if histogramConfig.Type != "" {
			match, _ := regexp.MatchString(histogramConfig.Type, perfType)
			if !match {
				continue
			}
		}
		result = append(result, histogramConfig.Values...)
	}
	return result
}

type performanceDataPoint struct {
	value           float64
	unit            string
	unitDisplayName string
}

func parsePerformanceData(perfData []string) map[string]performanceDataPoint {
	result := make(map[string]performanceDataPoint)
	for _, perfPoint := range perfData {
		parts := strings.Split(perfPoint, "=")
		if len(parts) == 2 {
			valueWithUnit := strings.Split(parts[1], ";")[0]
			value, err := strconv.ParseFloat(numbersOnly.FindString(valueWithUnit), 64)
			if err != nil {
				value = 0
			}
			unit := nonNumbersOnly.FindString(valueWithUnit)

			normalizedValue, normalizedUnit, unitDisplayName := normalizeUnit(value, unit)
			result[parts[0]] = performanceDataPoint{
				value:           normalizedValue,
				unit:            normalizedUnit,
				unitDisplayName: unitDisplayName,
			}
		}
	}
	return result
}

func normalizeUnit(value float64, unit string) (normalizedValue float64, normalizedUnit string, unitDisplayName string) {
	switch unit {
	// General
	case "":
		return value, "", "current"
	case "-":
		return value, "", "current"
	case "%":
		return value, "%", "percent"
	case "Synced":
		return value, "", "synced"

	// Durations
	case "ms":
		return value, "ms", "millis"
	case "s":
		return value * 1000, "ms", "millis"
	case "us":
		return value / 1000, "ms", "millis"
	case "ns":
		return value / 1000 / 1000, "ms", "millis"

	// Information Units
	case "B":
		return value, "B", "bytes"
	case "KB":
		return value * 1024, "B", "bytes"
	case "kB":
		return value * 1024, "B", "bytes"
	case "MB":
		return value * 1024 * 1024, "B", "bytes"
	case "GB":
		return value * 1024 * 1024 * 1024, "B", "bytes"
	case "TB":
		return value * 1024 * 1024 * 1024 * 1024, "B", "bytes"
	}

	// unknown unit
	return value, "", "current"
}
