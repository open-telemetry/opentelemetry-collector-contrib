// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/metadata"
)

var kubeClientProvider = kube.ClientProvider(nil)
var consumerCapabilities = consumer.Capabilities{MutatesData: true}
var defaultExcludes = ExcludeConfig{Pods: []ExcludePodConfig{{Name: "jaeger-agent"}, {Name: "jaeger-collector"}}}

// NewFactory returns a new factory for the k8s processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, metadata.TracesStability),
		processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability),
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
		Exclude:   defaultExcludes,
		Extract: ExtractConfig{
			Metadata: enabledAttributes(),
		},
	}
}

func createTracesProcessor(
	ctx context.Context,
	params processor.CreateSettings,
	cfg component.Config,
	next consumer.Traces,
) (processor.Traces, error) {
	return createTracesProcessorWithOptions(ctx, params, cfg, next)
}

func createLogsProcessor(
	ctx context.Context,
	params processor.CreateSettings,
	cfg component.Config,
	nextLogsConsumer consumer.Logs,
) (processor.Logs, error) {
	return createLogsProcessorWithOptions(ctx, params, cfg, nextLogsConsumer)
}

func createMetricsProcessor(
	ctx context.Context,
	params processor.CreateSettings,
	cfg component.Config,
	nextMetricsConsumer consumer.Metrics,
) (processor.Metrics, error) {
	return createMetricsProcessorWithOptions(ctx, params, cfg, nextMetricsConsumer)
}

func createTracesProcessorWithOptions(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	next consumer.Traces,
	options ...option,
) (processor.Traces, error) {
	kp, err := createKubernetesProcessor(set, cfg, options...)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewTracesProcessor(
		ctx,
		set,
		cfg,
		next,
		kp.processTraces,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(kp.Start),
		processorhelper.WithShutdown(kp.Shutdown))
}

func createMetricsProcessorWithOptions(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextMetricsConsumer consumer.Metrics,
	options ...option,
) (processor.Metrics, error) {
	kp, err := createKubernetesProcessor(set, cfg, options...)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewMetricsProcessor(
		ctx,
		set,
		cfg,
		nextMetricsConsumer,
		kp.processMetrics,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(kp.Start),
		processorhelper.WithShutdown(kp.Shutdown))
}

func createLogsProcessorWithOptions(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextLogsConsumer consumer.Logs,
	options ...option,
) (processor.Logs, error) {
	kp, err := createKubernetesProcessor(set, cfg, options...)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewLogsProcessor(
		ctx,
		set,
		cfg,
		nextLogsConsumer,
		kp.processLogs,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(kp.Start),
		processorhelper.WithShutdown(kp.Shutdown))
}

func createKubernetesProcessor(
	params processor.CreateSettings,
	cfg component.Config,
	options ...option,
) (*kubernetesprocessor, error) {
	kp := &kubernetesprocessor{logger: params.Logger}

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

func createProcessorOpts(cfg component.Config) []option {
	oCfg := cfg.(*Config)
	var opts []option
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
