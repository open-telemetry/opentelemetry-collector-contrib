// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
