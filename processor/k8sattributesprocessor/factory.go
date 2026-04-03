// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/collector/processor/processorhelper/xprocessorhelper"
	"go.opentelemetry.io/collector/processor/xprocessor"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/metadata"
)

var (
	errUnexpectedConfigurationType = errors.New("failed to cast configuration to k8s attributes config")
	consumerCapabilities           = consumer.Capabilities{MutatesData: true}
	defaultExcludes                = ExcludeConfig{Pods: []ExcludePodConfig{{Name: "jaeger-agent"}, {Name: "jaeger-collector"}}}
)

type kubernetesProcessorFactory struct {
	sharedWatchClients *sharedcomponent.SharedComponents
	clientProvider     kube.ClientProvider
}

func newKubernetesProcessorFactory(clientProvider kube.ClientProvider) *kubernetesProcessorFactory {
	if clientProvider == nil {
		clientProvider = kube.New
	}

	return &kubernetesProcessorFactory{
		sharedWatchClients: sharedcomponent.NewSharedComponents(),
		clientProvider:     clientProvider,
	}
}

// NewFactory returns a new factory for the k8s processor.
func NewFactory() processor.Factory {
	f := newKubernetesProcessorFactory(nil)

	return xprocessor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xprocessor.WithTraces(f.createTracesProcessor, metadata.TracesStability),
		xprocessor.WithMetrics(f.createMetricsProcessor, metadata.MetricsStability),
		xprocessor.WithLogs(f.createLogsProcessor, metadata.LogsStability),
		xprocessor.WithProfiles(f.createProfilesProcessor, metadata.ProfilesStability),
		xprocessor.WithDeprecatedTypeAlias(metadata.DeprecatedType),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		APIConfig: k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
		Exclude:   defaultExcludes,
		Extract: ExtractConfig{
			Metadata: enabledAttributes(),
		},
		WaitForMetadataTimeout: 10 * time.Second,
	}
}

func (f *kubernetesProcessorFactory) createTracesProcessor(
	ctx context.Context,
	params processor.Settings,
	cfg component.Config,
	next consumer.Traces,
) (processor.Traces, error) {
	return f.createTracesProcessorWithOptions(ctx, params, cfg, next)
}

func (f *kubernetesProcessorFactory) createLogsProcessor(
	ctx context.Context,
	params processor.Settings,
	cfg component.Config,
	nextLogsConsumer consumer.Logs,
) (processor.Logs, error) {
	return f.createLogsProcessorWithOptions(ctx, params, cfg, nextLogsConsumer)
}

func (f *kubernetesProcessorFactory) createMetricsProcessor(
	ctx context.Context,
	params processor.Settings,
	cfg component.Config,
	nextMetricsConsumer consumer.Metrics,
) (processor.Metrics, error) {
	return f.createMetricsProcessorWithOptions(ctx, params, cfg, nextMetricsConsumer)
}

func (f *kubernetesProcessorFactory) createProfilesProcessor(
	ctx context.Context,
	params processor.Settings,
	cfg component.Config,
	nextProfilesConsumer xconsumer.Profiles,
) (xprocessor.Profiles, error) {
	return f.createProfilesProcessorWithOptions(ctx, params, cfg, nextProfilesConsumer)
}

func (f *kubernetesProcessorFactory) createTracesProcessorWithOptions(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Traces,
	options ...option,
) (processor.Traces, error) {
	kp, err := f.createKubernetesProcessor(set, cfg, options...)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewTraces(
		ctx,
		set,
		cfg,
		next,
		kp.processTraces,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(kp.Start),
		processorhelper.WithShutdown(kp.Shutdown))
}

func (f *kubernetesProcessorFactory) createMetricsProcessorWithOptions(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextMetricsConsumer consumer.Metrics,
	options ...option,
) (processor.Metrics, error) {
	kp, err := f.createKubernetesProcessor(set, cfg, options...)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewMetrics(
		ctx,
		set,
		cfg,
		nextMetricsConsumer,
		kp.processMetrics,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(kp.Start),
		processorhelper.WithShutdown(kp.Shutdown))
}

func (f *kubernetesProcessorFactory) createLogsProcessorWithOptions(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextLogsConsumer consumer.Logs,
	options ...option,
) (processor.Logs, error) {
	kp, err := f.createKubernetesProcessor(set, cfg, options...)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewLogs(
		ctx,
		set,
		cfg,
		nextLogsConsumer,
		kp.processLogs,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(kp.Start),
		processorhelper.WithShutdown(kp.Shutdown))
}

func (f *kubernetesProcessorFactory) createProfilesProcessorWithOptions(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextProfilesConsumer xconsumer.Profiles,
	options ...option,
) (xprocessor.Profiles, error) {
	kp, err := f.createKubernetesProcessor(set, cfg, options...)
	if err != nil {
		return nil, err
	}

	return xprocessorhelper.NewProfiles(
		ctx,
		set,
		cfg,
		nextProfilesConsumer,
		kp.processProfiles,
		xprocessorhelper.WithCapabilities(consumerCapabilities),
		xprocessorhelper.WithStart(kp.Start),
		xprocessorhelper.WithShutdown(kp.Shutdown),
	)
}

func (f *kubernetesProcessorFactory) createKubernetesProcessor(
	params processor.Settings,
	cfg component.Config,
	options ...option,
) (*kubernetesprocessor, error) {
	telemetry, err := metadata.NewTelemetryBuilder(params.TelemetrySettings)
	if err != nil {
		params.Logger.Error("failed to create telemetry builder", zap.Error(err))
	}

	kp := &kubernetesprocessor{
		logger:            params.Logger,
		cfg:               cfg,
		options:           options,
		telemetrySettings: params.TelemetrySettings,
		telemetry:         telemetry,
		clientProvider:    f.clientProvider,
	}

	if err := kp.applyOptions(); err != nil {
		return nil, err
	}

	if err := f.attachSharedWatchClient(kp, cfg); err != nil {
		return nil, err
	}

	return kp, nil
}

func (f *kubernetesProcessorFactory) attachSharedWatchClient(kp *kubernetesprocessor, cfg component.Config) error {
	if kp.passthroughMode || !metadata.ProcessorK8sattributesShareInformerCachesFeatureGate.IsEnabled() {
		return nil
	}

	typedCfg, ok := cfg.(*Config)
	if !ok {
		return errUnexpectedConfigurationType
	}

	key, err := buildSharedWatchClientKey(typedCfg)
	if err != nil {
		return err
	}

	// Initial phase keys of the full defaulted config. This keeps sharing
	// conservative and order-sensitive so only strict-equivalent configurations
	// attach to the same shared watcher/cache instance.
	kp.sharedKubeComponent = f.sharedWatchClients.GetOrAdd(key, func() component.Component {
		return newSharedWatchClient(
			kp.logger,
			kp.telemetrySettings,
			kp.clientProvider,
			kp.apiConfig,
			kp.rules,
			kp.filters,
			kp.podAssociations,
			kp.podIgnore,
			kp.waitForMetadata,
			kp.waitForMetadataTimeout,
			key,
		)
	})

	kp.kc = kp.sharedWatchClient()

	return nil
}

func createProcessorOpts(cfg component.Config) []option {
	oCfg := cfg.(*Config)
	var opts []option
	if oCfg.Passthrough {
		opts = append(opts, withPassthrough())
	}

	// extraction rules
	opts = append(opts,
		withExtractMetadata(oCfg.Extract.Metadata...),
		withExtractLabels(oCfg.Extract.Labels...),
		withExtractAnnotations(oCfg.Extract.Annotations...),
		withOtelAnnotations(oCfg.Extract.OtelAnnotations),
		withDeploymentNameFromReplicaSet(oCfg.Extract.DeploymentNameFromReplicaSet),
		// filters
		withFilterNode(oCfg.Filter.Node, oCfg.Filter.NodeFromEnvVar),
		withFilterNamespace(oCfg.Filter.Namespace),
		withFilterLabels(oCfg.Filter.Labels...),
		withFilterFields(oCfg.Filter.Fields...),
		withAPIConfig(oCfg.APIConfig),
		withExtractPodAssociations(oCfg.Association...),
		withExcludes(oCfg.Exclude),
		withWaitForMetadataTimeout(oCfg.WaitForMetadataTimeout))

	if oCfg.WaitForMetadata {
		opts = append(opts, withWaitForMetadata(true))
	}

	return opts
}
