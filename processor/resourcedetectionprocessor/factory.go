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
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/akamai"
	alibabaecs "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/alibaba/ecs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ec2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ecs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/elasticbeanstalk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/lambda"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/aks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/consul"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/digitalocean"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/docker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/dynatrace"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/env"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/heroku"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/hetzner"
	ibmcloudclassic "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/ibmcloud/classic"
	ibmcloudvpc "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/ibmcloud/vpc"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/k8snode"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/kubeadm"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openshift"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openstack/nova"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/oraclecloud"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/scaleway"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/system"
	tencentcvm "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/tencent/cvm"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/upcloud"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/vultr"
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
		akamai.TypeStr:           akamai.NewDetector,
		alibabaecs.TypeStr:       alibabaecs.NewDetector,
		aks.TypeStr:              aks.NewDetector,
		azure.TypeStr:            azure.NewDetector,
		consul.TypeStr:           consul.NewDetector,
		digitalocean.TypeStr:     digitalocean.NewDetector,
		docker.TypeStr:           docker.NewDetector,
		ec2.TypeStr:              ec2.NewDetector,
		ecs.TypeStr:              ecs.NewDetector,
		eks.TypeStr:              eks.NewDetector,
		elasticbeanstalk.TypeStr: elasticbeanstalk.NewDetector,
		lambda.TypeStr:           lambda.NewDetector,
		env.TypeStr:              env.NewDetector,
		gcp.TypeStr:              gcp.NewDetector,
		heroku.TypeStr:           heroku.NewDetector,
		hetzner.TypeStr:          hetzner.NewDetector,
		ibmcloudclassic.TypeStr:  ibmcloudclassic.NewDetector,
		ibmcloudvpc.TypeStr:      ibmcloudvpc.NewDetector,
		scaleway.TypeStr:         scaleway.NewDetector,
		system.TypeStr:           system.NewDetector,
		openshift.TypeStr:        openshift.NewDetector,
		nova.TypeStr:             nova.NewDetector,
		oraclecloud.TypeStr:      oraclecloud.NewDetector,
		k8snode.TypeStr:          k8snode.NewDetector,
		kubeadm.TypeStr:          kubeadm.NewDetector,
		dynatrace.TypeStr:        dynatrace.NewDetector,
		tencentcvm.TypeStr:       tencentcvm.NewDetector,
		upcloud.TypeStr:          upcloud.NewDetector,
		vultr.TypeStr:            vultr.NewDetector,
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
		Detectors:       []string{env.TypeStr},
		ClientConfig:    defaultClientConfig(),
		Override:        true,
		DetectorConfig:  detectorCreateDefaultConfig(),
		RefreshInterval: 0,
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
		processorhelper.WithStart(rdp.Start),
		processorhelper.WithShutdown(rdp.Shutdown),
	)
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
		processorhelper.WithStart(rdp.Start),
		processorhelper.WithShutdown(rdp.Shutdown),
	)
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
		processorhelper.WithStart(rdp.Start),
		processorhelper.WithShutdown(rdp.Shutdown),
	)
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
		xprocessorhelper.WithStart(rdp.Start),
		xprocessorhelper.WithShutdown(rdp.Shutdown),
	)
}

func (f *factory) getResourceDetectionProcessor(
	params processor.Settings,
	cfg component.Config,
) (*resourceDetectionProcessor, error) {
	oCfg := cfg.(*Config)

	warnDeprecatedPerDetectorFlags(params.Logger, oCfg)

	provider, err := f.getResourceProvider(params, oCfg.Timeout, oCfg.Detectors, oCfg.DetectorConfig)
	if err != nil {
		return nil, err
	}

	// Resolve the effective fail_on_missing_metadata: top-level OR any per-detector flag (OR logic).
	failOnMissingMetadata := oCfg.FailOnMissingMetadata ||
		oCfg.DetectorConfig.EC2Config.FailOnMissingMetadata || //nolint:staticcheck
		oCfg.DetectorConfig.AlibabaECSConfig.FailOnMissingMetadata || //nolint:staticcheck
		oCfg.DetectorConfig.TencentCVMConfig.FailOnMissingMetadata || //nolint:staticcheck
		oCfg.DetectorConfig.UpcloudConfig.FailOnMissingMetadata || //nolint:staticcheck
		oCfg.DetectorConfig.VultrConfig.FailOnMissingMetadata || //nolint:staticcheck
		oCfg.DetectorConfig.OpenStackNovaConfig.FailOnMissingMetadata //nolint:staticcheck

	return &resourceDetectionProcessor{
		provider:              provider,
		override:              oCfg.Override,
		httpClientSettings:    oCfg.ClientConfig,
		refreshInterval:       oCfg.RefreshInterval,
		telemetrySettings:     params.TelemetrySettings,
		failOnMissingMetadata: failOnMissingMetadata,
	}, nil
}

// warnDeprecatedPerDetectorFlags emits a deprecation warning if any per-detector
// fail_on_missing_metadata fields are set without the top-level flag.
func warnDeprecatedPerDetectorFlags(logger *zap.Logger, oCfg *Config) {
	if oCfg.FailOnMissingMetadata {
		return // top-level is set; per-detector fields are superseded
	}
	var affectedDetectors []string
	if oCfg.DetectorConfig.EC2Config.FailOnMissingMetadata { //nolint:staticcheck
		affectedDetectors = append(affectedDetectors, "ec2")
	}
	if oCfg.DetectorConfig.AlibabaECSConfig.FailOnMissingMetadata { //nolint:staticcheck
		affectedDetectors = append(affectedDetectors, "alibaba_ecs")
	}
	if oCfg.DetectorConfig.TencentCVMConfig.FailOnMissingMetadata { //nolint:staticcheck
		affectedDetectors = append(affectedDetectors, "tencent_cvm")
	}
	if oCfg.DetectorConfig.UpcloudConfig.FailOnMissingMetadata { //nolint:staticcheck
		affectedDetectors = append(affectedDetectors, "upcloud")
	}
	if oCfg.DetectorConfig.VultrConfig.FailOnMissingMetadata { //nolint:staticcheck
		affectedDetectors = append(affectedDetectors, "vultr")
	}
	if oCfg.DetectorConfig.OpenStackNovaConfig.FailOnMissingMetadata { //nolint:staticcheck
		affectedDetectors = append(affectedDetectors, "nova")
	}
	if len(affectedDetectors) > 0 {
		logger.Warn(
			"per-detector fail_on_missing_metadata fields are deprecated; use the top-level fail_on_missing_metadata instead",
			zap.Strings("affected_detectors", affectedDetectors),
		)
	}
}

func (f *factory) getResourceProvider(
	params processor.Settings,
	timeout time.Duration,
	configuredDetectors []string,
	detectorConfigs DetectorConfig,
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

	provider, err := f.resourceProviderFactory.CreateResourceProvider(params, timeout, &detectorConfigs, detectorTypes...)
	if err != nil {
		return nil, err
	}

	f.providers[params.ID] = provider
	return provider, nil
}
