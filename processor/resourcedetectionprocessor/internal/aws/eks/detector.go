// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eks // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks"

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

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
)

// detector for EKS
type detector struct {
	cfg          Config
	imdsProvider imdsprovider.Provider
	apiProvider  apiprovider.Provider
	logger       *zap.Logger
	ra           metadata.ResourceAttributesConfig
	rb           *metadata.ResourceBuilder
}

var _ internal.Detector = (*detector)(nil)

// NewDetector returns a resource detector that will detect AWS EKS resources.
func NewDetector(set processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)
	awsConfig, err := getAWSConfig(context.Background(), cfg.MaxAttempts, cfg.MaxBackoff)
	if err != nil {
		return nil, err
	}

	apiProvider, err := apiprovider.NewProvider(awsConfig)
	if err != nil {
		return nil, err
	}

	return &detector{
		cfg:          cfg,
		apiProvider:  apiProvider,
		imdsProvider: imdsprovider.NewProvider(awsConfig),
		logger:       set.Logger,
		ra:           cfg.ResourceAttributes,
		rb:           metadata.NewResourceBuilder(cfg.ResourceAttributes),
	}, nil
}

// Detect returns a Resource describing the Amazon EKS environment being run in.
func (d *detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	// Check if running on EKS.
	isEKS, err := d.isEKS()
	if err != nil {
		d.logger.Debug("Unable to identify EKS environment", zap.Error(err))
		return pcommon.NewResource(), "", err
	}
	if !isEKS {
		return pcommon.NewResource(), "", nil
	}

	d.rb.SetCloudProvider(conventions.CloudProviderAWS.Value.AsString())
	d.rb.SetCloudPlatform(conventions.CloudPlatformAWSEKS.Value.AsString())

	// Don't run once as the state of the node/cluster/instance might change in case of failures and retry
	imdsAccessible := isIMDSAccessible(ctx, d.logger, d.cfg.IMDSCheckMaxRetry, d.cfg.MaxBackoff)
	var instanceID string
	if imdsAccessible {
		var meta imds.InstanceIdentityDocument
		if meta, err = d.imdsProvider.Get(ctx); err != nil {
			d.logger.Debug("Unable to get instance identity document", zap.Error(err))
		}
		instanceID = meta.InstanceID
		d.rb.SetHostID(instanceID)
		d.rb.SetCloudAvailabilityZone(meta.AvailabilityZone)
		d.rb.SetCloudRegion(meta.Region)
		d.rb.SetCloudAccountID(meta.AccountID)
	} else {
		nodeName, exist := os.LookupEnv(d.cfg.NodeFromEnvVar)
		if !exist {
			return pcommon.NewResource(), "",
				fmt.Errorf(" can't find Instance Metadata. node name not found in environment variable %s", d.cfg.NodeFromEnvVar)
		}
		// InitializeInstanceMetadata needs to run before using the api provider
		// It should also pass the caller's context and bail out if we can't set the instance metadata
		if err = d.apiProvider.InitializeInstanceMetadata(ctx, nodeName); err != nil {
			d.logger.Debug("Unable to initialize API provider", zap.Error(err))
			return pcommon.NewResource(), "", err
		}
		instanceID = d.apiProvider.InstanceID()
		d.rb.SetHostID(instanceID)
		d.rb.SetCloudAvailabilityZone(d.apiProvider.AvailabilityZone())
		d.rb.SetCloudRegion(d.apiProvider.Region())
		if d.ra.CloudAccountID.Enabled {
			var accountID string
			if accountID, err = d.apiProvider.AccountID(ctx); err != nil {
				d.logger.Debug("Unable to get account ID", zap.Error(err))
			}
			d.rb.SetCloudAccountID(accountID)
		}
	}

	if d.ra.K8sClusterName.Enabled {
		var clusterName string
		if clusterName, err = d.apiProvider.ClusterName(ctx, instanceID); err != nil {
			d.logger.Debug("Unable to get cluster name", zap.Error(err))
		}
		d.rb.SetK8sClusterName(clusterName)
	}

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

func isIMDSAccessible(ctx context.Context, logger *zap.Logger, maxRetry int, maxBackoff time.Duration) bool {
	cfg, err := getAWSConfig(ctx, maxRetry, maxBackoff)
	if err != nil {
		logger.Debug("EC2 Metadata service is not accessible", zap.Error(err))
		return false
	}

	client := imds.NewFromConfig(cfg)
	if _, err := client.GetMetadata(ctx, &imds.GetMetadataInput{Path: "instance-id"}); err != nil {
		logger.Debug("EC2 Metadata service is not accessible", zap.Error(err))
		return false
	}
	return true
}

func getAWSConfig(ctx context.Context, maxAttempts int, maxBackoff time.Duration) (aws.Config, error) {
	awsConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return aws.Config{}, err
	}
	awsConfig.Retryer = func() aws.Retryer {
		return retry.NewStandard(func(options *retry.StandardOptions) {
			options.MaxAttempts = maxAttempts
			options.MaxBackoff = maxBackoff
		})
	}
	return awsConfig, nil
}
