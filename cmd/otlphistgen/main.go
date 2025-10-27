// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/aws/cloudwatch/histograms"
)

const (
	defaultGRPCEndpoint = "localhost:4317"
	defaultHTTPEndpoint = "localhost:4318"
)

var (
	errFormatOTLPAttributes       = errors.New("value should be in one of the following formats: key=\"value\", key=true, key=false, or key=<integer>")
	errDoubleQuotesOTLPAttributes = errors.New("value should be a string wrapped in double quotes")
)

type KeyValue map[string]any

var _ pflag.Value = (*KeyValue)(nil)

func (*KeyValue) String() string {
	return ""
}

func (v *KeyValue) Set(s string) error {
	kv := strings.SplitN(s, "=", 2)
	if len(kv) != 2 {
		return errFormatOTLPAttributes
	}
	val := kv[1]
	if val == "true" {
		(*v)[kv[0]] = true
		return nil
	}
	if val == "false" {
		(*v)[kv[0]] = false
		return nil
	}
	if intVal, err := strconv.Atoi(val); err == nil {
		(*v)[kv[0]] = intVal
		return nil
	}
	if len(val) < 2 || !strings.HasPrefix(val, "\"") || !strings.HasSuffix(val, "\"") {
		return errDoubleQuotesOTLPAttributes
	}

	(*v)[kv[0]] = val[1 : len(val)-1]
	return nil
}

func (*KeyValue) Type() string {
	return "map[string]any"
}

// Config describes the test scenario.
type Config struct {
	CustomEndpoint string
	Insecure       bool
	UseHTTP        bool
	HTTPPath       string
	Headers        KeyValue
}

// Endpoint returns the appropriate endpoint URL based on the selected communication mode (gRPC or HTTP)
// or custom endpoint provided in the configuration.
func (c *Config) Endpoint() string {
	if c.CustomEndpoint != "" {
		return c.CustomEndpoint
	}
	if c.UseHTTP {
		return defaultHTTPEndpoint
	}
	return defaultGRPCEndpoint
}

func (c *Config) GetHeaders() map[string]string {
	m := make(map[string]string, len(c.Headers))

	for k, t := range c.Headers {
		switch v := t.(type) {
		case bool:
			m[k] = strconv.FormatBool(v)
		case string:
			m[k] = v
		}
	}

	return m
}

func main() {
	exporter, err := createExporter(&Config{
		UseHTTP:  true,
		Insecure: true,
	})
	if err != nil {
		log.Fatal(err)
	}

	res := resource.NewWithAttributes(semconv.SchemaURL)

	startTime := time.Now()

	go func() {
		testCases := histograms.TestCases()
		ticker := time.NewTicker(time.Second * 10)
		for range ticker.C {
			for _, tc := range testCases {
				metrics := []metricdata.Metrics{{
					Name: tc.Name,
					Data: metricdata.Histogram[float64]{
						Temporality: metricdata.DeltaTemporality,
						DataPoints: []metricdata.HistogramDataPoint[float64]{
							tcToDatapoint(tc, startTime),
						},
					},
				}}
				rm := metricdata.ResourceMetrics{
					Resource:     res,
					ScopeMetrics: []metricdata.ScopeMetrics{{Metrics: metrics}},
				}

				if err := exporter.Export(context.Background(), &rm); err != nil {
					log.Fatal("exporter failed", zap.Error(err))
				}
			}
		}
	}()

	ticker := time.NewTicker(time.Second * 10)
	testCases := histograms.InvalidTestCases()
	for range ticker.C {
		for _, tc := range testCases {
			metrics := []metricdata.Metrics{{
				Name: tc.Name,
				Data: metricdata.Histogram[float64]{
					Temporality: metricdata.DeltaTemporality,
					DataPoints: []metricdata.HistogramDataPoint[float64]{
						tcToDatapoint(tc, startTime),
					},
				},
			}}
			rm := metricdata.ResourceMetrics{
				Resource:     res,
				ScopeMetrics: []metricdata.ScopeMetrics{{Metrics: metrics}},
			}

			if err := exporter.Export(context.Background(), &rm); err != nil {
				log.Fatal("exporter failed", zap.Error(err))
			}
		}
	}
}

func createExporter(cfg *Config) (sdkmetric.Exporter, error) {
	var exp sdkmetric.Exporter
	var err error
	if cfg.UseHTTP {
		log.Print("starting HTTP exporter")
		exporterOpts := httpExporterOptions(cfg)
		exp, err = otlpmetrichttp.New(context.Background(), exporterOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to obtain OTLP HTTP exporter: %w", err)
		}
	} else {
		return nil, errors.New("NotYetImplemented")
	}
	return exp, err
}

// httpExporterOptions creates the configuration options for an HTTP-based OTLP metric exporter.
// It configures the exporter with the provided endpoint, URL path, connection security settings, and headers.
func httpExporterOptions(cfg *Config) []otlpmetrichttp.Option {
	httpExpOpt := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(cfg.Endpoint()),
		otlpmetrichttp.WithURLPath(cfg.HTTPPath),
	}

	if cfg.Insecure {
		httpExpOpt = append(httpExpOpt, otlpmetrichttp.WithInsecure())
	}

	if len(cfg.Headers) > 0 {
		httpExpOpt = append(httpExpOpt, otlpmetrichttp.WithHeaders(cfg.GetHeaders()))
	}

	return httpExpOpt
}

func tcToDatapoint(tc histograms.HistogramTestCase, startTime time.Time) metricdata.HistogramDataPoint[float64] {
	attrs := []attribute.KeyValue{}
	for k, v := range tc.Input.Attributes {
		attrs = append(attrs, attribute.String(k, v))
	}

	dp := metricdata.HistogramDataPoint[float64]{
		StartTime:    startTime,
		Time:         time.Now(),
		Attributes:   attribute.NewSet(attrs...),
		Count:        tc.Input.Count,
		Sum:          tc.Input.Sum,
		Bounds:       tc.Input.Boundaries,
		BucketCounts: tc.Input.Counts,
	}

	if tc.Input.Min != nil {
		dp.Min = metricdata.NewExtrema(*tc.Input.Min)
	}
	if tc.Input.Max != nil {
		dp.Max = metricdata.NewExtrema(*tc.Input.Max)
	}
	return dp
}
