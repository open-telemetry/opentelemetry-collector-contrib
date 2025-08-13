// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcedetectionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/collector/processor/processorhelper/xprocessorhelper"
	"go.opentelemetry.io/collector/processor/xprocessor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ec2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ecs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/elasticbeanstalk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/lambda"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/aks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/consul"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/docker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/dynatrace"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/env"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/heroku"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/k8snode"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/kubeadm"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openshift"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/system"
)

var consumerCapabilities = consumer.Capabilities{MutatesData: true}

type factory struct {
	resourceProviderFactory *internal.ResourceProviderFactory

	// providers stores a provider for each named processor that
	// may a different set of detectors configured.
	providers map[component.ID]*internal.ResourceProvider
	lock      sync.Mutex
}

// NewFactory creates a new factory for ResourceDetection processor.
func NewFactory() processor.Factory {
	resourceProviderFactory := internal.NewProviderFactory(map[internal.DetectorType]internal.DetectorFactory{
		aks.TypeStr:              aks.NewDetector,
		azure.TypeStr:            azure.NewDetector,
		consul.TypeStr:           consul.NewDetector,
		docker.TypeStr:           docker.NewDetector,
		ec2.TypeStr:              ec2.NewDetector,
		ecs.TypeStr:              ecs.NewDetector,
		eks.TypeStr:              eks.NewDetector,
		elasticbeanstalk.TypeStr: elasticbeanstalk.NewDetector,
		lambda.TypeStr:           lambda.NewDetector,
		env.TypeStr:              env.NewDetector,
		gcp.TypeStr:              gcp.NewDetector,
		heroku.TypeStr:           heroku.NewDetector,
		system.TypeStr:           system.NewDetector,
		openshift.TypeStr:        openshift.NewDetector,
		k8snode.TypeStr:          k8snode.NewDetector,
		kubeadm.TypeStr:          kubeadm.NewDetector,
		dynatrace.TypeStr:        dynatrace.NewDetector,
	})

	f := &factory{
		resourceProviderFactory: resourceProviderFactory,
		providers:               map[component.ID]*internal.ResourceProvider{},
	}

	return xprocessor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xprocessor.WithTraces(f.createTracesProcessor, metadata.TracesStability),
		xprocessor.WithMetrics(f.createMetricsProcessor, metadata.MetricsStability),
		xprocessor.WithLogs(f.createLogsProcessor, metadata.LogsStability),
		xprocessor.WithProfiles(f.createProfilesProcessor, metadata.ProfilesStability))
}

// Type gets the type of the Option config created by this factory.
func (*factory) Type() component.Type {
	return metadata.Type
}

func createDefaultConfig() component.Config {
	return &Config{
		Detectors:      []string{env.TypeStr},
		ClientConfig:   defaultClientConfig(),
		Override:       true,
		Attributes:     nil,
		DetectorConfig: detectorCreateDefaultConfig(),
		// TODO: Once issue(https://github.com/open-telemetry/opentelemetry-collector/issues/4001) gets resolved,
		//		 Set the default value of 'hostname_source' here instead of 'system' detector
	}
}

func defaultClientConfig() confighttp.ClientConfig {
	httpClientSettings := confighttp.NewDefaultClientConfig()
	httpClientSettings.Timeout = 5 * time.Second
	return httpClientSettings
}

func (f *factory) createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	rdp, err := f.getResourceDetectionProcessor(set, cfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewTraces(
		ctx,
		set,
		cfg,
		nextConsumer,
		rdp.processTraces,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(rdp.Start))
}

func (f *factory) createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	rdp, err := f.getResourceDetectionProcessor(set, cfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewMetrics(
		ctx,
		set,
		cfg,
		nextConsumer,
		rdp.processMetrics,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(rdp.Start))
}

func (f *factory) createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	rdp, err := f.getResourceDetectionProcessor(set, cfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewLogs(
		ctx,
		set,
		cfg,
		nextConsumer,
		rdp.processLogs,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(rdp.Start))
}

func (f *factory) createProfilesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer xconsumer.Profiles,
) (xprocessor.Profiles, error) {
	rdp, err := f.getResourceDetectionProcessor(set, cfg)
	if err != nil {
		return nil, err
	}

	return xprocessorhelper.NewProfiles(
		ctx,
		set,
		cfg,
		nextConsumer,
		rdp.processProfiles,
		xprocessorhelper.WithCapabilities(consumerCapabilities),
		xprocessorhelper.WithStart(rdp.Start))
}

func (f *factory) getResourceDetectionProcessor(
	params processor.Settings,
	cfg component.Config,
) (*resourceDetectionProcessor, error) {
	oCfg := cfg.(*Config)
	if oCfg.Attributes != nil {
		params.Logger.Warn("You are using deprecated `attributes` option that will be removed soon; use `resource_attributes` instead, details on configuration: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourcedetectionprocessor#migration-from-attributes-to-resource_attributes")
	}
	provider, err := f.getResourceProvider(params, oCfg.Timeout, oCfg.Detectors, oCfg.DetectorConfig, oCfg.Attributes)
	if err != nil {
		return nil, err
	}

	return &resourceDetectionProcessor{
		provider:           provider,
		override:           oCfg.Override,
		httpClientSettings: oCfg.ClientConfig,
		telemetrySettings:  params.TelemetrySettings,
	}, nil
}

func (f *factory) getResourceProvider(
	params processor.Settings,
	timeout time.Duration,
	configuredDetectors []string,
	detectorConfigs DetectorConfig,
	attributes []string,
) (*internal.ResourceProvider, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if provider, ok := f.providers[params.ID]; ok {
		return provider, nil
	}

	detectorTypes := make([]internal.DetectorType, 0, len(configuredDetectors))
	for _, key := range configuredDetectors {
		detectorTypes = append(detectorTypes, internal.DetectorType(strings.TrimSpace(key)))
	}

	provider, err := f.resourceProviderFactory.CreateResourceProvider(params, timeout, attributes, &detectorConfigs, detectorTypes...)
	if err != nil {
		return nil, err
	}

	f.providers[params.ID] = provider
	return provider, nil
}
