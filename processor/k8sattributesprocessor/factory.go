// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"

import (
	"context"
	"fmt"

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
	pCfg := cfg.(*Config)

	err := errWrongKeyConfig(pCfg)
	if err != nil {
		return nil, err
	}

	if len(pCfg.Extract.Metadata) == 0 {
		kp.logger.Warn("Default metadata extraction rules will be changed in future releases, " +
			"please specify the extraction rules explicitly in the `processors::k8sattributes::extract::metadata` field. " +
			"See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/23136 for more details.")
	}

	allOptions := append(createProcessorOpts(pCfg), options...)

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

func createProcessorOpts(cfg *Config) []option {
	var opts []option
	if cfg.Passthrough {
		opts = append(opts, withPassthrough())
	}

	// extraction rules
	opts = append(opts, withExtractMetadata(cfg.Extract.Metadata...))
	opts = append(opts, withExtractLabels(cfg.Extract.Labels...))
	opts = append(opts, withExtractAnnotations(cfg.Extract.Annotations...))

	// filters
	opts = append(opts, withFilterNode(cfg.Filter.Node, cfg.Filter.NodeFromEnvVar))
	opts = append(opts, withFilterNamespace(cfg.Filter.Namespace))
	opts = append(opts, withFilterLabels(cfg.Filter.Labels...))
	opts = append(opts, withFilterFields(cfg.Filter.Fields...))
	opts = append(opts, withAPIConfig(cfg.APIConfig))

	opts = append(opts, withExtractPodAssociations(cfg.Association...))

	opts = append(opts, withExcludes(cfg.Exclude))

	return opts
}

func errWrongKeyConfig(cfg *Config) error {
	for _, r := range append(cfg.Extract.Labels, cfg.Extract.Annotations...) {
		if r.Key != "" && r.KeyRegex != "" {
			return fmt.Errorf("Out of Key or KeyRegex only one option is expected to be configured at a time, currently Key:%s and KeyRegex:%s", r.Key, r.KeyRegex)
		}
	}
	return nil
}
