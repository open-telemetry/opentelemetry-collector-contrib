// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package protocol

import (
	"bytes"
	"fmt"
	"time"

	ogrek "github.com/kisielk/og-rek"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type PickleConfig struct{}

type PickleParser struct {
	cfg *PickleConfig
}

func NewPickleParser(cfg *PickleConfig) (Parser, error) {
	if cfg == nil {
		cfg = &PickleConfig{}
	}
	return &PickleParser{cfg: cfg}, nil
}

func (*PickleConfig) BuildParser() (Parser, error) {
	return NewPickleParser(nil)
}

func (*PickleParser) Parse(data []byte) (pmetric.Metrics, error) {

	decoder := ogrek.NewDecoder(bytes.NewReader(data))
	decodedData, err := decoder.Decode()
	if err != nil {
		return pmetric.NewMetrics(), fmt.Errorf("failed to decode pickle: %w", err)
	}

	decodedDataList, ok := decodedData.([]any)
	if !ok {
		return pmetric.NewMetrics(), fmt.Errorf("unexpected pickle structure: %T", decodedData)
	}

	return toMetrics(decodedDataList)
}

// TODO how to report errors ? t.reporter.OnTranslationError(ctx, err) ?
func toMetrics(decodedDataList []any) (pmetric.Metrics, error) {
	metrics := pmetric.NewMetrics()
	rms := metrics.ResourceMetrics().AppendEmpty()
	scopedMetrics := rms.ScopeMetrics().AppendEmpty()
	metricSlice := scopedMetrics.Metrics()
	for _, item := range decodedDataList {
		entry, ok := item.([]any)
		if !ok || len(entry) != 2 {
			continue //skip malformed item
		}

		metricName, ok := entry[0].(string)
		if !ok {
			continue
		}

		point, ok := entry[1].([]any)
		if !ok || len(point) != 2 {
			continue
		}

		timestamp, ok := point[0].(string)
		value, ok2 := point[1].(string)
		if !ok || !ok2 {
			continue
		}

		unixTime, unixTimeNs, err := parseCarbonTimestamp(timestamp)
		if err != nil {
			continue
		}

		intVal, dblVal, isFloat, err := parseCarbonValue(value)
		if err != nil {
			continue
		}

		metric := metricSlice.AppendEmpty()
		metric.SetName(metricName)
		// Use Gauge by default
		metric.SetEmptyGauge()
		dp := metric.Gauge().DataPoints().AppendEmpty()
		if isFloat {
			dp.SetDoubleValue(dblVal)
		} else {
			dp.SetIntValue(intVal)
		}
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(unixTime, unixTimeNs)))
	}
	return metrics, nil
}

func pickleDefaultConfig() ParserConfig {
	return &PickleConfig{}
}
