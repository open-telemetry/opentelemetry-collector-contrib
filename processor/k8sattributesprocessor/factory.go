// Copyright 2020 OpenTelemetry Authors
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

package k8sattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
)

const (
	// The value of "type" key in configuration.
	typeStr = "k8sattributes"
)

var kubeClientProvider = kube.ClientProvider(nil)
var consumerCapabilities = consumer.Capabilities{MutatesData: true}
var defaultExcludes = ExcludeConfig{Pods: []ExcludePodConfig{{Name: "jaeger-agent"}, {Name: "jaeger-collector"}}}

// NewFactory returns a new factory for the k8s processor.
func NewFactory() component.ProcessorFactory {
	return component.NewProcessorFactory(
		typeStr,
		createDefaultConfig,
		component.WithTracesProcessor(createTracesProcessor),
		component.WithMetricsProcessor(createMetricsProcessor),
		component.WithLogsProcessor(createLogsProcessor),
	)
}

func createDefaultConfig() config.Processor {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
		APIConfig:         k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
		Exclude:           defaultExcludes,
	}
}

func createTracesProcessor(
	ctx context.Context,
	params component.ProcessorCreateSettings,
	cfg config.Processor,
	next consumer.Traces,
) (component.TracesProcessor, error) {
	return createTracesProcessorWithOptions(ctx, params, cfg, next)
}

func createLogsProcessor(
	ctx context.Context,
	params component.ProcessorCreateSettings,
	cfg config.Processor,
	nextLogsConsumer consumer.Logs,
) (component.LogsProcessor, error) {
	return createLogsProcessorWithOptions(ctx, params, cfg, nextLogsConsumer)
}

func createMetricsProcessor(
	ctx context.Context,
	params component.ProcessorCreateSettings,
	cfg config.Processor,
	nextMetricsConsumer consumer.Metrics,
) (component.MetricsProcessor, error) {
	return createMetricsProcessorWithOptions(ctx, params, cfg, nextMetricsConsumer)
}

func createTracesProcessorWithOptions(
	_ context.Context,
	params component.ProcessorCreateSettings,
	cfg config.Processor,
	next consumer.Traces,
	options ...option,
) (component.TracesProcessor, error) {
	kp, err := createKubernetesProcessor(params, cfg, options...)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewTracesProcessor(
		cfg,
		next,
		kp.processTraces,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(kp.Start),
		processorhelper.WithShutdown(kp.Shutdown))
}

func createMetricsProcessorWithOptions(
	_ context.Context,
	params component.ProcessorCreateSettings,
	cfg config.Processor,
	nextMetricsConsumer consumer.Metrics,
	options ...option,
) (component.MetricsProcessor, error) {
	kp, err := createKubernetesProcessor(params, cfg, options...)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewMetricsProcessor(
		cfg,
		nextMetricsConsumer,
		kp.processMetrics,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(kp.Start),
		processorhelper.WithShutdown(kp.Shutdown))
}

func createLogsProcessorWithOptions(
	_ context.Context,
	params component.ProcessorCreateSettings,
	cfg config.Processor,
	nextLogsConsumer consumer.Logs,
	options ...option,
) (component.LogsProcessor, error) {
	kp, err := createKubernetesProcessor(params, cfg, options...)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewLogsProcessor(
		cfg,
		nextLogsConsumer,
		kp.processLogs,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(kp.Start),
		processorhelper.WithShutdown(kp.Shutdown))
}

func createKubernetesProcessor(
	params component.ProcessorCreateSettings,
	cfg config.Processor,
	options ...option,
) (*kubernetesprocessor, error) {
	kp := &kubernetesprocessor{logger: params.Logger}

	warnDeprecatedMetadataConfig(kp.logger, cfg)
	warnDeprecatedPodAssociationConfig(kp.logger, cfg)

	err := errWrongKeyConfig(cfg)
	if err != nil {
		return nil, err
	}

	allOptions := append(createProcessorOpts(cfg), options...)

	for _, opt := range allOptions {
		if err := opt(kp); err != nil {
			return nil, err
		}
	}

	// This might have been set by an option already
	if kp.kc == nil {
		err := kp.initKubeClient(kp.logger, kubeClientProvider)
		if err != nil {
			return nil, err
		}
	}

	return kp, nil
}

func createProcessorOpts(cfg config.Processor) []option {
	oCfg := cfg.(*Config)
	opts := []option{}
	if oCfg.Passthrough {
		opts = append(opts, withPassthrough())
	}

	// extraction rules
	opts = append(opts, withExtractMetadata(oCfg.Extract.Metadata...))
	opts = append(opts, withExtractLabels(oCfg.Extract.Labels...))
	opts = append(opts, withExtractAnnotations(oCfg.Extract.Annotations...))

	// filters
	opts = append(opts, withFilterNode(oCfg.Filter.Node, oCfg.Filter.NodeFromEnvVar))
	opts = append(opts, withFilterNamespace(oCfg.Filter.Namespace))
	opts = append(opts, withFilterLabels(oCfg.Filter.Labels...))
	opts = append(opts, withFilterFields(oCfg.Filter.Fields...))
	opts = append(opts, withAPIConfig(oCfg.APIConfig))

	opts = append(opts, withExtractPodAssociations(oCfg.Association...))

	opts = append(opts, withExcludes(oCfg.Exclude))

	return opts
}

func warnDeprecatedMetadataConfig(logger *zap.Logger, cfg config.Processor) {
	oCfg := cfg.(*Config)
	for _, field := range oCfg.Extract.Metadata {
		var oldName, newName string
		switch field {
		case metdataNamespace:
			oldName = metdataNamespace
			newName = conventions.AttributeK8SNamespaceName
		case metadataPodName:
			oldName = metadataPodName
			newName = conventions.AttributeK8SPodName
		case metadataPodUID:
			oldName = metadataPodUID
			newName = conventions.AttributeK8SPodUID
		case metadataStartTime:
			oldName = metadataStartTime
			newName = metadataPodStartTime
		case metadataDeployment:
			oldName = metadataDeployment
			newName = conventions.AttributeK8SDeploymentName
		case metadataNode:
			oldName = metadataNode
			newName = conventions.AttributeK8SNodeName
		case deprecatedMetadataCluster:
			logger.Warn("cluster metadata param has been deprecated and will be removed soon")
		case conventions.AttributeK8SClusterName:
			logger.Warn("k8s.cluster.name metadata param has been deprecated and will be removed soon")
		}
		if oldName != "" {
			logger.Warn(fmt.Sprintf("%s has been deprecated in favor of %s for k8s-tagger processor", oldName, newName))
		}
	}

}

func errWrongKeyConfig(cfg config.Processor) error {
	oCfg := cfg.(*Config)

	for _, r := range append(oCfg.Extract.Labels, oCfg.Extract.Annotations...) {
		if r.Key != "" && r.KeyRegex != "" {
			return fmt.Errorf("Out of Key or KeyRegex only one option is expected to be configured at a time, currently Key:%s and KeyRegex:%s", r.Key, r.KeyRegex)
		}
	}

	return nil
}

func warnDeprecatedPodAssociationConfig(logger *zap.Logger, cfg config.Processor) {
	oCfg := cfg.(*Config)
	deprecated := ""
	actual := ""
	for _, assoc := range oCfg.Association {
		if assoc.From == "" && assoc.Name == "" {
			continue
		}

		deprecated += fmt.Sprintf(`
- from: %s`, assoc.From)
		actual += fmt.Sprintf(`
- sources:
  - from: %s`, assoc.From)

		if assoc.Name != "" {
			deprecated += fmt.Sprintf(`
  name: %s`, assoc.Name)
		}

		if assoc.From != kube.ConnectionSource {
			actual += fmt.Sprintf(`
    name: %s`, assoc.Name)
		}
	}

	if deprecated != "" {
		logger.Warn(fmt.Sprintf(`Deprecated pod_association configuration detected. Please replace:

pod_association:%s

with

pod_association:%s

`, deprecated, actual))
	}
}
