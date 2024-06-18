// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"

import (
	"context"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
)

func httpMetrics(meter metric.Meter, next http.Handler) http.Handler {
	hist, err := meter.Int64Histogram("datadog.http.server.duration",
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10),
	)
	if err != nil {
		panic(err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := recrw{ResponseWriter: w, Code: http.StatusOK}
		next.ServeHTTP(&rw, r)

		hist.Record(context.Background(), int64(time.Since(start)), metric.WithAttributes(
			semconv.HTTPMethod(r.Method),
			semconv.HTTPRoute(r.URL.Path),
			semconv.HTTPStatusCode(rw.Code),
		))
	})
}

type recrw struct {
	Code int
	http.ResponseWriter
}

func (rw *recrw) WriteHeader(code int) {
	rw.Code = code
	rw.ResponseWriter.WriteHeader(code)
}
