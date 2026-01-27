// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/golden/internal"

import (
	"errors"
	"strings"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

type Config struct {
	ExpectedFile    string
	WriteExpected   bool
	CompareOptions  []pmetrictest.CompareMetricsOption
	OTLPEndpoint    string
	OTLPHTTPEndoint string
	Timeout         time.Duration
}

func ReadConfig(args []string) (*Config, error) {
	opts := []pmetrictest.CompareMetricsOption{}
	writeExpected := false
	var expectedFile string
	var otlpEndpoint string
	var otlpHTTPEndpoint string
	timeout := 2 * time.Minute
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--timeout":
			i++
			if i == len(args) {
				return nil, errors.New("--timeout requires an argument")
			}
			var err error
			timeout, err = time.ParseDuration(args[i])
			if err != nil {
				return nil, err
			}
		case "--expected":
			i++
			if i == len(args) {
				return nil, errors.New("--expected requires an argument")
			}
			expectedFile = args[i]
		case "--otlp-endpoint":
			i++
			if i == len(args) {
				return nil, errors.New("--otlp-endpoint requires an argument")
			}
			otlpEndpoint = args[i]
		case "--otlp-http-endpoint":
			i++
			if i == len(args) {
				return nil, errors.New("--otlp-endpoint requires an argument")
			}
			otlpHTTPEndpoint = args[i]
		case "--write-expected":
			writeExpected = true
		case "--ignore-start-timestamp":
			opts = append(opts, pmetrictest.IgnoreStartTimestamp())
		case "--ignore-timestamp":
			opts = append(opts, pmetrictest.IgnoreTimestamp())
		case "--ignore-metrics-data-points-order":
			opts = append(opts, pmetrictest.IgnoreMetricDataPointsOrder())
		case "--ignore-metrics-order":
			opts = append(opts, pmetrictest.IgnoreMetricsOrder())
		case "--ignore-scope-metrics-order":
			opts = append(opts, pmetrictest.IgnoreScopeMetricsOrder())
		case "--ignore-resource-metrics-order":
			opts = append(opts, pmetrictest.IgnoreResourceMetricsOrder())
		case "--ignore-exemplars":
			opts = append(opts, pmetrictest.IgnoreExemplars())
		case "--ignore-exemplar-slice":
			opts = append(opts, pmetrictest.IgnoreExemplarSlice())
		case "--ignore-scope-version":
			opts = append(opts, pmetrictest.IgnoreScopeVersion())
		case "--ignore-data-points-attributes-order":
			opts = append(opts, pmetrictest.IgnoreDatapointAttributesOrder())
		case "--ignore-resource-attribute-value":
			i++
			if i == len(args) {
				return nil, errors.New("--ignore-resource-attribute-value requires an argument")
			}
			opts = append(opts, pmetrictest.IgnoreResourceAttributeValue(args[i]))
		case "--ignore-metric-attribute-value":
			i++
			if i == len(args) {
				return nil, errors.New("--ignore-metric-attribute-value requires an argument")
			}
			opts = append(opts, pmetrictest.IgnoreMetricAttributeValue(args[i]))
		case "--ignore-metric-values":
			if i < len(args)-1 && !strings.HasPrefix(args[i+1], "--") {
				i++
				opts = append(opts, pmetrictest.IgnoreMetricValues(args[i]))
			} else {
				opts = append(opts, pmetrictest.IgnoreMetricValues())
			}
		}
	}
	return &Config{
		WriteExpected:   writeExpected,
		CompareOptions:  opts,
		ExpectedFile:    expectedFile,
		OTLPEndpoint:    otlpEndpoint,
		OTLPHTTPEndoint: otlpHTTPEndpoint,
		Timeout:         timeout,
	}, nil
}
