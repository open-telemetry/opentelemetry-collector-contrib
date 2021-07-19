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

// This program sends one span to the OpenTelemetry-Collector Agent and is meant to illustrate how to connect an existing
// OpenTelemtry-instrumented programs using a DatadogTraceIDRatioBased s ampler.
// The default connection is HTTP and the gRPC connection is activated by means of the -grpc flag.

package example_custom_sampler

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"runtime"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlphttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

var (
	grpcDriver = flag.Bool("grpc", false, "use GRPC exporter instead of HTTP")
)

type datadogTraceIDRatioSampler struct {
	sdktrace.Sampler
	traceIDUpperBound uint64
	description       string
	probability       float64
}

func (dts datadogTraceIDRatioSampler) ShouldSample(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
	psc := trace.SpanContextFromContext(p.ParentContext)
	x := binary.BigEndian.Uint64(p.TraceID[0:8]) >> 1
	if x < dts.traceIDUpperBound {
		return sdktrace.SamplingResult{
			Decision:   sdktrace.RecordAndSample,
			Attributes: []attribute.KeyValue{attribute.Float64("_sample_rate", dts.probability)},
			Tracestate: psc.TraceState(),
		}
	}
	return sdktrace.SamplingResult{
		Decision:   sdktrace.RecordOnly,
		Tracestate: psc.TraceState(),
	}
}

func (dts datadogTraceIDRatioSampler) Description() string {
	return "datadogTraceIDRatioSampler"
}

// DatadogTraceIDRatioBased samples a given fraction of traces. Fractions >= 1 will
// always sample. Fractions < 0 are treated as zero. To respect the
// parent trace's `SampledFlag`, the `DatadogTraceIDRatioBased` sampler should be used
// as a delegate of a `Parent` sampler.
func DatadogTraceIDRatioBased(fraction float64) sdktrace.Sampler {
	if fraction >= 1 {
		return sdktrace.AlwaysSample()
	}

	if fraction <= 0 {
		fraction = 0
	}

	return &datadogTraceIDRatioSampler{
		traceIDUpperBound: uint64(fraction * (1 << 63)),
		description:       fmt.Sprintf("TraceIDRatioBased{%g}", fraction),
		probability:       fraction,
	}
}

func main() {
	flag.Parse()
	ctx := context.Background()

	headers := map[string]string{
		"Datadog-Meta-Lang-Version":     strings.TrimPrefix(runtime.Version(), "go"),
		"Datadog-Meta-Lang-Interpreter": runtime.Compiler + "-" + runtime.GOARCH + "-" + runtime.GOOS,
	}
	var driver otlp.ProtocolDriver = otlphttp.NewDriver(
		otlphttp.WithInsecure(),
		otlphttp.WithHeaders(headers),
	)
	if *grpcDriver {
		driver = otlpgrpc.NewDriver(
			otlpgrpc.WithInsecure(),
			otlpgrpc.WithHeaders(headers),
		)
	}
	exporter, err := otlp.NewExporter(ctx, driver)
	if err != nil {
		log.Fatalf("failed to create exporter: %v", err)
	}
	defer func() {
		if err := exporter.Shutdown(ctx); err != nil {
			log.Fatalf("failed to stop exporter: %v", err)
		}
	}()

	// Create an example provider.
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(DatadogTraceIDRatioBased(0.5))),
		sdktrace.WithSyncer(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			attribute.String("service.name", "my-service"),
			attribute.String("service.version", "1.2.3"),
			attribute.String("env", "staging"),
		)),
	)

	// Start an example tracer and span.
	tracer := provider.Tracer("mytracer", trace.WithInstrumentationVersion("1.0.0"))
	_, span := tracer.Start(ctx, "operation", trace.WithAttributes(
		attribute.Bool("mybool", false),
		attribute.Int("myint", 1),
		attribute.Float64("myfloat64", 2),
		attribute.String("mystring", "asd"),
	))
	span.End()
}
