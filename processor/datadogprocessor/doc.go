// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package datadogprocessor contains the Datadog Processor. The Datadog Processor is used in
// conjunction with the Collector's tail samplers (such as the tailsamplingprocessor or the
// probabilisticsamplerprocessor) to extract accurate APM Stats in situations when not all
// traces are seen by the Datadog Exporter.
//
// By default, the processor looks for an exporter named "datadog" in a metrics pipeline.
// It can use any other exporter (in case the name is different, a gateway deployment is used
// or the collector runs alongside the Datadog Agent) but this needs to be specified via config,
// such as for example:
//
//	processor:
//	  datadog:
//	    metrics_exporter: otlp
package datadogprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadogprocessor"
