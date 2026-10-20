// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !omit_detector_aws_eks

package eks // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.uber.org/zap"

	imdsprovider "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/aws/ec2"
	apiprovider "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/aws/eks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks/internal/metadata"
)

const (
	// Environment variable that is set when running on Kubernetes.
	kubernetesServiceHostEnvVar = "KUBERNETES_SERVICE_HOST"
	// EKS cluster version string identifier.
	eksClusterStringIdentifier = "-eks-"
	// EKS OIDC issuer identifier.
	eksOIDCIssuerIdentifier = "oidc.eks."
	// EKS IRSA token file path identifier.
	eksIRSATokenPathIdentifier = "eks.amazonaws.com" //nolint:gosec // not a credential
	// EKS Pod Identity token file path identifier.
	eksPodIdentityPathIdentifier = "eks-pod-identity"
	// Environment variable for IRSA web identity token file.
	awsWebIdentityTokenFileEnvVar = "AWS_WEB_IDENTITY_TOKEN_FILE" //nolint:gosec // env var name, not a credential
	// Environment variable for EKS Pod Identity authorization token file.
	awsContainerAuthTokenFileEnvVar = "AWS_CONTAINER_AUTHORIZATION_TOKEN_FILE" //nolint:gosec // env var name, not a credential

	imdsCheckMaxRetry = 1
)

// eksClusterARNFormat is the ARN format of an EKS cluster. The ARN is fully
// determined by the partition, the region, the account id and the cluster name.
// See https://docs.aws.amazon.com/service-authorization/latest/reference/list_amazonelastickubernetesservice.html#amazonelastickubernetesservice-resources-for-iam-policies
const eksClusterARNFormat = "arn:%s:eks:%s:%s:cluster/%s"

// arnPartitions maps current region prefixes to ARN partitions. Malformed
// regions and regions with an unrecognized leading prefix are rejected rather
// than assigned a valid-looking ARN in a guessed partition.
var arnPartitions = []struct {
	regionPrefix string
	partition    string
}{
	{regionPrefix: "cn-", partition: "aws-cn"},
	{regionPrefix: "us-gov-", partition: "aws-us-gov"},
	{regionPrefix: "us-iso-", partition: "aws-iso"},
	{regionPrefix: "us-isob-", partition: "aws-iso-b"},
	{regionPrefix: "us-isof-", partition: "aws-iso-f"},
	{regionPrefix: "eu-isoe-", partition: "aws-iso-e"},
	{regionPrefix: "eusc-de-", partition: "aws-eusc"},
	{regionPrefix: "us-", partition: "aws"},
	{regionPrefix: "eu-", partition: "aws"},
	{regionPrefix: "ap-", partition: "aws"},
	{regionPrefix: "sa-", partition: "aws"},
	{regionPrefix: "ca-", partition: "aws"},
	{regionPrefix: "me-", partition: "aws"},
	{regionPrefix: "af-", partition: "aws"},
	{regionPrefix: "il-", partition: "aws"},
	{regionPrefix: "mx-", partition: "aws"},
}

// partitionForRegion returns an empty string for an unknown or malformed region.
func partitionForRegion(region string) string {
	if !validRegion(region) {
		return ""
	}
	for _, p := range arnPartitions {
		if strings.HasPrefix(region, p.regionPrefix) {
			return p.partition
		}
	}
	return ""
}

func validRegion(region string) bool {
	if len(region) < 3 || len(region) > 64 || region[0] == '-' || region[len(region)-1] == '-' || strings.Contains(region, "--") {
		return false
	}
	for _, char := range region {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
			return false
		}
	}
	return true
}

func validAccountID(accountID string) bool {
	if len(accountID) != 12 {
		return false
	}
	for _, char := range accountID {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func validClusterName(clusterName string) bool {
	if clusterName == "" || len(clusterName) > 100 {
		return false
	}
	for i, char := range clusterName {
		alphanumeric := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9'
		if !alphanumeric && (i == 0 || char != '-' && char != '_') {
			return false
		}
	}
	return true
}

// constructClusterARN builds the ARN only when every part has a valid shape.
// The account is the EC2 node owner's account. EKS managed and self-managed
// EC2 nodes are expected to belong to the same account as their cluster.
func constructClusterARN(region, accountID, clusterName string) string {
	partition := partitionForRegion(region)
	if partition == "" || !validAccountID(accountID) || !validClusterName(clusterName) {
		return ""
	}
	return fmt.Sprintf(eksClusterARNFormat, partition, region, accountID, clusterName)
}

// detector for EKS
type detector struct {
	cfg                   Config
	logger                *zap.Logger
	imdsProvider          imdsprovider.Provider
	apiProvider           apiprovider.Provider
	ra                    metadata.ResourceAttributesConfig
	rb                    *metadata.ResourceBuilder
	utils                 detectorUtils
	failOnMissingMetadata bool
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
func NewDetector(set processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	cfg := dcfg.(Config)
	utils := &eksDetectorUtils{cfg: cfg, logger: set.Logger}

	awsConfig, err := utils.getAWSConfig(context.Background(), false)
	if err != nil {
		return nil, err
	}

	nodeName := os.Getenv(cfg.NodeFromEnvVar)
	apiProvider, err := apiprovider.NewProvider(awsConfig, nodeName, cfg.APIConfig)
	if err != nil {
		return nil, err
	}

	return &detector{
		cfg:                   cfg,
		logger:                set.Logger,
		apiProvider:           apiProvider,
		imdsProvider:          imdsprovider.NewProvider(awsConfig),
		ra:                    cfg.ResourceAttributes,
		rb:                    metadata.NewResourceBuilder(cfg.ResourceAttributes),
		utils:                 utils,
		failOnMissingMetadata: failOnMissingMetadata,
	}, nil
}

// Detect returns a Resource describing the Amazon EKS environment being run in.
func (d *detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	// Check if running on EKS.
	if isEKS, err := d.isEKS(ctx); err != nil || !isEKS {
		if err != nil {
			d.logger.Debug("Unable to identify EKS environment", zap.Error(err))
			if d.failOnMissingMetadata {
				return pcommon.NewResource(), "", fmt.Errorf("eks metadata unavailable: %w", err)
			}
		} else if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", errors.New("eks metadata unavailable: not running on EKS")
		}
		return pcommon.NewResource(), "", nil
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
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", err
		}
		d.logger.Debug("EKS IMDS metadata unavailable", zap.Error(err))
		return d.rb.Emit(), conventions.SchemaURL, nil
	}

	d.rb.SetHostID(imdsMeta.InstanceID)
	d.rb.SetCloudAvailabilityZone(imdsMeta.AvailabilityZone)
	d.rb.SetCloudRegion(imdsMeta.Region)
	d.rb.SetCloudAccountID(imdsMeta.AccountID)
	d.rb.SetHostImageID(imdsMeta.ImageID)
	d.rb.SetHostType(imdsMeta.InstanceType)

	hostname, err := d.imdsProvider.Hostname(ctx)
	if err != nil {
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", err
		}
		d.logger.Debug("EKS IMDS hostname unavailable", zap.Error(err))
		return d.rb.Emit(), conventions.SchemaURL, nil
	}
	d.rb.SetHostName(hostname)

	if d.ra.K8sClusterName.Enabled || d.ra.AwsEksClusterArn.Enabled {
		d.apiProvider.SetRegionInstanceID(imdsMeta.Region, imdsMeta.InstanceID)
		var apiMeta apiprovider.InstanceMetadata
		if apiMeta, err = d.apiProvider.GetInstanceMetadata(ctx); err != nil {
			if d.failOnMissingMetadata {
				return pcommon.NewResource(), "", err
			}
			d.logger.Debug("EKS instance metadata unavailable", zap.Error(err))
			return d.rb.Emit(), conventions.SchemaURL, nil
		}
		d.rb.SetK8sClusterName(apiMeta.ClusterName)
		if arn := constructClusterARN(imdsMeta.Region, imdsMeta.AccountID, apiMeta.ClusterName); arn != "" {
			d.rb.SetAwsEksClusterArn(arn)
		} else if d.ra.AwsEksClusterArn.Enabled {
			d.logger.Debug(
				"EKS cluster ARN unavailable",
				zap.Bool("region_valid", partitionForRegion(imdsMeta.Region) != ""),
				zap.Bool("account_id_valid", validAccountID(imdsMeta.AccountID)),
				zap.Bool("cluster_name_valid", validClusterName(apiMeta.ClusterName)),
			)
		}
	}
	return d.rb.Emit(), conventions.SchemaURL, nil
}

func (d *detector) detectFromAPI(ctx context.Context) (pcommon.Resource, string, error) {
	k8sMeta, err := d.apiProvider.GetK8sInstanceMetadata(ctx)
	if err != nil {
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", err
		}
		d.logger.Debug("EKS k8s instance metadata unavailable", zap.Error(err))
		return d.rb.Emit(), conventions.SchemaURL, nil
	}

	d.rb.SetHostID(k8sMeta.InstanceID)
	d.rb.SetCloudAvailabilityZone(k8sMeta.AvailabilityZone)
	d.rb.SetCloudRegion(k8sMeta.Region)

	apiMeta, err := d.apiProvider.GetInstanceMetadata(ctx)
	if err != nil {
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", err
		}
		d.logger.Debug("EKS instance metadata unavailable", zap.Error(err))
		return d.rb.Emit(), conventions.SchemaURL, nil
	}

	d.rb.SetCloudAccountID(apiMeta.AccountID)
	d.rb.SetHostImageID(apiMeta.ImageID)
	d.rb.SetHostType(apiMeta.InstanceType)
	d.rb.SetHostName(apiMeta.Hostname)
	d.rb.SetK8sClusterName(apiMeta.ClusterName)
	if arn := constructClusterARN(k8sMeta.Region, apiMeta.AccountID, apiMeta.ClusterName); arn != "" {
		d.rb.SetAwsEksClusterArn(arn)
	} else if d.ra.AwsEksClusterArn.Enabled {
		d.logger.Debug(
			"EKS cluster ARN unavailable",
			zap.Bool("region_valid", partitionForRegion(k8sMeta.Region) != ""),
			zap.Bool("account_id_valid", validAccountID(apiMeta.AccountID)),
			zap.Bool("cluster_name_valid", validClusterName(apiMeta.ClusterName)),
		)
	}

	return d.rb.Emit(), conventions.SchemaURL, nil
}

func (d *detector) isEKS(ctx context.Context) (bool, error) {
	if os.Getenv(kubernetesServiceHostEnvVar) == "" {
		return false, nil
	}

	// Check for EKS-specific IRSA token path
	if tokenFile := os.Getenv(awsWebIdentityTokenFileEnvVar); strings.Contains(tokenFile, eksIRSATokenPathIdentifier) {
		return true, nil
	}

	// Check for EKS Pod Identity token path
	if authFile := os.Getenv(awsContainerAuthTokenFileEnvVar); strings.Contains(authFile, eksPodIdentityPathIdentifier) {
		return true, nil
	}

	// Check OIDC issuer for EKS-specific identifier
	if issuer, err := d.apiProvider.OIDCIssuer(ctx); err == nil && strings.Contains(issuer, eksOIDCIssuerIdentifier) {
		return true, nil
	}

	// Fallback: check cluster version for -eks- identifier
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

func (eksDetectorUtils) getAWSConfig(ctx context.Context, checkAccess bool) (aws.Config, error) {
	awsConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return aws.Config{}, err
	}

	if checkAccess {
		awsConfig.Retryer = func() aws.Retryer {
			return retry.NewStandard(func(options *retry.StandardOptions) {
				options.MaxAttempts = imdsCheckMaxRetry
			})
		}
	}

	return awsConfig, nil
}
