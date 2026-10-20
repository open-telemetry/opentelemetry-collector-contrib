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

	// maxClusterNameLength is the longest name EKS accepts for a cluster.
	maxClusterNameLength = 100
)

// errNoUsableClusterName reports that the node carried nothing that can name a
// cluster: no tag naming one, tags naming more than one, or a name that EKS
// could not have issued. It does not carry the name itself, which can come
// from an EC2 tag in a namespace a user can write to.
//
// A name EKS could not have issued is a disproven name rather than an unproven
// one: no cluster of that name can exist, so no request could confirm it and
// the tag it came from is not describing this node's cluster. A caller asking
// for ARN confirmation treats it the way it treats a confirmed mismatch.
var errNoUsableClusterName = errors.New("no usable cluster name on the node")

// validClusterName reports whether the name is shaped like an EKS cluster name,
// so that a tag value which cannot name a cluster is rejected before it costs a
// DescribeCluster request. A name must start with a letter or digit and may
// then also contain dashes and underscores.
// See https://docs.aws.amazon.com/eks/latest/APIReference/API_CreateCluster.html#AmazonEKS-CreateCluster-request-name
func validClusterName(clusterName string) bool {
	if clusterName == "" || len(clusterName) > maxClusterNameLength {
		return false
	}
	// Indexed by byte so that the length check above and the position check
	// below are in the same units. Any byte of a multi-byte character fails
	// the alphanumeric test and is neither a dash nor an underscore, so an
	// accepted name is ASCII only and its byte length is its character length.
	for i := 0; i < len(clusterName); i++ {
		char := clusterName[i]
		alphanumeric := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9'
		if alphanumeric {
			continue
		}
		if i == 0 || char != '-' && char != '_' {
			return false
		}
	}
	return true
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
	// Discard anything left in the builder by an earlier run. Detect is retried
	// on failure and re-run on every refresh_interval tick, and the error paths
	// below return without calling Emit, which is what otherwise clears it. A
	// later run would then emit attributes established by an earlier one.
	d.rb.Emit()

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
		if err := d.setClusterMetadata(ctx, apiMeta); err != nil {
			return pcommon.NewResource(), "", err
		}
	}
	return d.rb.Emit(), conventions.SchemaURL, nil
}

// setClusterARN records the ARN that EKS reports for the cluster the node
// belongs to, and reports why it could not. The lookup only runs when the
// attribute is enabled, and whether a failure is fatal is the caller's
// decision, taken from fail_on_missing_metadata, so that this attribute
// follows the same rule as every other piece of metadata the detector reads.
//
// The returned errors never carry the cluster name: it can come from an EC2
// tag in a namespace a user can write to, so it can name a cluster this node
// has nothing to do with.
//
// A name EKS could not have issued is rejected before the request is made.
// There is no cluster of that name for EKS to describe, so the rejection is
// itself the answer.
func (d *detector) setClusterARN(ctx context.Context, clusterName string) error {
	if !d.ra.AwsEksClusterArn.Enabled {
		return nil
	}
	if !validClusterName(clusterName) {
		return errNoUsableClusterName
	}
	arn, err := d.apiProvider.ClusterARN(ctx, clusterName)
	if err != nil {
		return err
	}
	d.rb.SetAwsEksClusterArn(arn)
	return nil
}

// setClusterMetadata records the cluster name and the cluster ARN, each only
// when it was actually established.
//
// The ARN check runs first because it is what decides whether the name is
// trustworthy. Two of its outcomes disprove the name rather than leaving it
// unproven, and in both the name is dropped along with the ARN, because
// reporting it would attribute this node's telemetry to a cluster that is not
// this one:
//
//   - EKS described the named cluster and it is a different cluster from the
//     one this node runs in.
//   - The name is not one EKS could have issued, so no cluster of that name
//     can exist and no request could have confirmed it.
//
// Every other ARN failure leaves the name alone, because none of them prove
// anything about it. A failed eks:DescribeCluster is the case worth being
// explicit about: ResourceNotFoundException reads like the name naming
// nothing, but the request is made with this node's role in this node's
// account, so it is equally the answer for a cluster that exists and that this
// role or account cannot see, and dropping the name there would lose a correct
// k8s.cluster.name over a missing IAM grant or a cross-account cluster.
//
// With the attribute disabled, which is the default, no check runs, nothing is
// disproven, and the name is reported as before. An empty name is never
// reported either way: the node carried no tag naming a cluster, or carried
// tags naming more than one, and an empty attribute asserts a cluster named ""
// rather than saying nothing.
func (d *detector) setClusterMetadata(ctx context.Context, meta apiprovider.InstanceMetadata) error {
	arnErr := d.setClusterARN(ctx, meta.ClusterName)

	switch {
	case meta.ClusterName == "":
		if meta.ClusterNameAmbiguous {
			d.logger.Debug("EKS cluster name unavailable; the node's EC2 tags name more than one cluster at the same precedence")
		} else {
			d.logger.Debug("EKS cluster name unavailable; the node carries no EC2 tag naming a cluster")
		}
	case errors.Is(arnErr, apiprovider.ErrClusterIdentityMismatch):
		d.logger.Debug("EKS cluster name not reported; EKS reports that the cluster named by the node's EC2 tags is not the cluster this node runs in")
	case errors.Is(arnErr, errNoUsableClusterName):
		d.logger.Debug("EKS cluster name not reported; the name on the node's EC2 tags is not one EKS could have issued")
	default:
		d.rb.SetK8sClusterName(meta.ClusterName)
	}

	if arnErr != nil {
		if d.failOnMissingMetadata {
			return fmt.Errorf("eks cluster ARN unavailable: %w", arnErr)
		}
		d.logger.Debug("EKS cluster ARN unavailable", zap.Error(arnErr))
	}
	return nil
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

	setIfNotEmpty(d.rb.SetCloudAccountID, apiMeta.AccountID)
	setIfNotEmpty(d.rb.SetHostImageID, apiMeta.ImageID)
	setIfNotEmpty(d.rb.SetHostType, apiMeta.InstanceType)
	setIfNotEmpty(d.rb.SetHostName, apiMeta.Hostname)
	if err := d.setClusterMetadata(ctx, apiMeta); err != nil {
		return pcommon.NewResource(), "", err
	}

	return d.rb.Emit(), conventions.SchemaURL, nil
}

// setIfNotEmpty leaves an attribute off rather than reporting it as an empty
// string. An ec2:DescribeInstances response can succeed while describing no
// instance, which leaves the values it would have carried empty, and an empty
// attribute asserts a value of "" rather than saying nothing.
func setIfNotEmpty(set func(string), value string) {
	if value != "" {
		set(value)
	}
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
