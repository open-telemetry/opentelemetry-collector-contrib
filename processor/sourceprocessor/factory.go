// Copyright 2019 OpenTelemetry Authors
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

package sourceprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "source"

	defaultSource    = "traces"
	defaultCollector = ""

	defaultSourceName                = "%{namespace}.%{pod}.%{container}"
	defaultSourceCategory            = "%{namespace}/%{pod_name}"
	defaultSourceCategoryPrefix      = "kubernetes/"
	defaultSourceCategoryReplaceDash = "/"

	defaultAnnotationPrefix   = "pod_annotation_"
	defaultContainerKey       = "container"
	defaultNamespaceKey       = "namespace"
	defaultPodIDKey           = "pod_id"
	defaultPodKey             = "pod"
	defaultPodNameKey         = "pod_name"
	defaultPodTemplateHashKey = "pod_labels_pod-template-hash"
	defaultSourceHostKey      = "source_host"
)

// NewFactory returns a new factory for the Span processor.
func NewFactory() component.ProcessorFactory {
	return processorhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		processorhelper.WithTraces(createTraceProcessor))
}

// createDefaultConfig creates the default configuration for processor.
func createDefaultConfig() config.Processor {
	return &Config{
		ProcessorSettings: &config.ProcessorSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
		Source:                    defaultSource,
		Collector:                 defaultCollector,
		SourceName:                defaultSourceName,
		SourceCategory:            defaultSourceCategory,
		SourceCategoryPrefix:      defaultSourceCategoryPrefix,
		SourceCategoryReplaceDash: defaultSourceCategoryReplaceDash,

		AnnotationPrefix:   defaultAnnotationPrefix,
		ContainerKey:       defaultContainerKey,
		NamespaceKey:       defaultNamespaceKey,
		PodKey:             defaultPodKey,
		PodIDKey:           defaultPodIDKey,
		PodNameKey:         defaultPodNameKey,
		PodTemplateHashKey: defaultPodTemplateHashKey,
		SourceHostKey:      defaultSourceHostKey,
	}
}

// CreateTraceProcessor creates a trace processor based on this config.
func createTraceProcessor(
	_ context.Context,
	_ component.ProcessorCreateParams,
	cfg config.Processor,
	nextConsumer consumer.Traces) (component.TracesProcessor, error) {

	oCfg := cfg.(*Config)
	return newSourceTraceProcessor(nextConsumer, oCfg), nil
}
