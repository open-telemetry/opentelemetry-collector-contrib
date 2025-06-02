// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eks // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks"

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"
	"go.uber.org/zap"

	imdsprovider "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/aws/ec2"
	apiprovider "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/aws/eks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "eks"
	// Environment variable that is set when running on Kubernetes.
	kubernetesServiceHostEnvVar = "KUBERNETES_SERVICE_HOST"
	// EKS cluster version string identifier.
	eksClusterStringIdentifier = "-eks-"

	imdsCheckMaxRetry = 1
)

// detector for EKS
type detector struct {
	cfg          Config
	logger       *zap.Logger
	imdsProvider imdsprovider.Provider
	apiProvider  apiprovider.Provider
	ra           metadata.ResourceAttributesConfig
	rb           *metadata.ResourceBuilder
	utils        detectorUtils
}

type eksDetectorUtils struct {
	cfg    Config
	logger *zap.Logger
}

type detectorUtils interface {
	isIMDSAccessible(ctx context.Context) bool
	getAWSConfig(ctx context.Context, testAccess bool) (aws.Config, error)
}

var _ internal.Detector = (*detector)(nil)

var _ detectorUtils = (*eksDetectorUtils)(nil)

// NewDetector returns a resource detector that will detect AWS EKS resources.
func NewDetector(set processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)
	utils := &eksDetectorUtils{cfg: cfg, logger: set.Logger}

	awsConfig, err := utils.getAWSConfig(context.Background(), false)
	if err != nil {
		return nil, err
	}

	nodeName := os.Getenv(cfg.NodeFromEnvVar)
	apiProvider, err := apiprovider.NewProvider(awsConfig, nodeName)
	if err != nil {
		return nil, err
	}

	return &detector{
		cfg:          cfg,
		logger:       set.Logger,
		apiProvider:  apiProvider,
		imdsProvider: imdsprovider.NewProvider(awsConfig),
		ra:           cfg.ResourceAttributes,
		rb:           metadata.NewResourceBuilder(cfg.ResourceAttributes),
		utils:        utils,
	}, nil
}

// Detect returns a Resource describing the Amazon EKS environment being run in.
func (d *detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	// Check if running on EKS.
	if isEKS, err := d.isEKS(); err != nil || !isEKS {
		if err != nil {
			d.logger.Debug("Unable to identify EKS environment", zap.Error(err))
		}
		return pcommon.NewResource(), "", err
	}

	d.rb.SetCloudProvider(conventions.CloudProviderAWS.Value.AsString())
	d.rb.SetCloudPlatform(conventions.CloudPlatformAWSEKS.Value.AsString())

	if d.utils.isIMDSAccessible(ctx) {
		return d.detectFromIMDS(ctx)
	}
	return d.detectFromAPI(ctx)
}

func (d *detector) detectFromIMDS(ctx context.Context) (pcommon.Resource, string, error) {
	imdsMeta, err := d.imdsProvider.Get(ctx)
	if err != nil {
		return d.rb.Emit(), conventions.SchemaURL, err
	}

	d.rb.SetHostID(imdsMeta.InstanceID)
	d.rb.SetCloudAvailabilityZone(imdsMeta.AvailabilityZone)
	d.rb.SetCloudRegion(imdsMeta.Region)
	d.rb.SetCloudAccountID(imdsMeta.AccountID)
	d.rb.SetHostImageID(imdsMeta.ImageID)
	d.rb.SetHostType(imdsMeta.InstanceType)

	hostname, err := d.imdsProvider.Hostname(ctx)
	if err != nil {
		return d.rb.Emit(), conventions.SchemaURL, err
	}
	d.rb.SetHostName(hostname)

	if d.ra.K8sClusterName.Enabled {
		d.apiProvider.SetRegionInstanceID(imdsMeta.Region, imdsMeta.InstanceID)
		var apiMeta apiprovider.InstanceMetadata
		if apiMeta, err = d.apiProvider.GetInstanceMetadata(ctx); err != nil {
			return d.rb.Emit(), conventions.SchemaURL, err
		}
		d.rb.SetK8sClusterName(apiMeta.ClusterName)
	}
	return d.rb.Emit(), conventions.SchemaURL, nil
}

func (d *detector) detectFromAPI(ctx context.Context) (pcommon.Resource, string, error) {
	k8sMeta, err := d.apiProvider.GetK8sInstanceMetadata(ctx)
	if err != nil {
		return d.rb.Emit(), conventions.SchemaURL, err
	}

	d.rb.SetHostID(k8sMeta.InstanceID)
	d.rb.SetCloudAvailabilityZone(k8sMeta.AvailabilityZone)
	d.rb.SetCloudRegion(k8sMeta.Region)

	apiMeta, err := d.apiProvider.GetInstanceMetadata(ctx)
	if err != nil {
		return d.rb.Emit(), conventions.SchemaURL, err
	}

	d.rb.SetCloudAccountID(apiMeta.AccountID)
	d.rb.SetHostImageID(apiMeta.ImageID)
	d.rb.SetHostType(apiMeta.InstanceType)
	d.rb.SetHostName(apiMeta.Hostname)
	d.rb.SetK8sClusterName(apiMeta.ClusterName)

	return d.rb.Emit(), conventions.SchemaURL, nil
}

func (d *detector) isEKS() (bool, error) {
	if os.Getenv(kubernetesServiceHostEnvVar) == "" {
		return false, nil
	}

	clusterVersion, err := d.apiProvider.ClusterVersion()
	if err != nil {
		return false, fmt.Errorf("isEks() error retrieving cluster version: %w", err)
	}
	if strings.Contains(clusterVersion, eksClusterStringIdentifier) {
		return true, nil
	}
	return false, nil
}

func (e eksDetectorUtils) isIMDSAccessible(ctx context.Context) bool {
	cfg, err := e.getAWSConfig(ctx, true)
	if err != nil {
		e.logger.Debug("EC2 Metadata service is not accessible", zap.Error(err))
		return false
	}

	client := imds.NewFromConfig(cfg)
	if _, err := client.GetMetadata(ctx, &imds.GetMetadataInput{Path: "instance-id"}); err != nil {
		e.logger.Debug("EC2 Metadata service is not accessible", zap.Error(err))
		return false
	}
	return true
}

func (e eksDetectorUtils) getAWSConfig(ctx context.Context, checkAccess bool) (aws.Config, error) {
	awsConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return aws.Config{}, err
	}

	maxAttempts := e.cfg.MaxAttempts
	if checkAccess {
		maxAttempts = imdsCheckMaxRetry
	}

	awsConfig.Retryer = func() aws.Retryer {
		return retry.NewStandard(func(options *retry.StandardOptions) {
			options.MaxAttempts = maxAttempts
			options.MaxBackoff = e.cfg.MaxBackoff
		})
	}
	return awsConfig, nil
}
