// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package ec2 contains the AWS EC2 hostname provider
package ec2 // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/internal/ec2"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/provider"
	ec2provider "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/aws/ec2"
)

var defaultPrefixes = [3]string{"ip-", "domu", "ec2amaz-"}

type HostInfo struct {
	InstanceID  string
	EC2Hostname string
	EC2Tags     []string
}

// isDefaultHostname checks if a hostname is an EC2 default
func isDefaultHostname(hostname string) bool {
	for _, val := range defaultPrefixes {
		if strings.HasPrefix(hostname, val) {
			return true
		}
	}

	return false
}

// GetHostInfo gets the hostname info from EC2 metadata
func GetHostInfo(ctx context.Context, logger *zap.Logger) (hostInfo *HostInfo) {
	hostInfo = &HostInfo{}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		logger.Warn("Failed to build AWS config", zap.Error(err))
		return
	}

	client := imds.NewFromConfig(cfg)

	// Check if metadata service is available by trying to retrieve instance ID
	_, err = client.GetMetadata(ctx, &imds.GetMetadataInput{
		Path: "instance-id",
	})
	if err != nil {
		logger.Debug("EC2 Metadata service is not available", zap.Error(err))
		return
	}

	idDoc, err := client.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
	if err == nil {
		hostInfo.InstanceID = idDoc.InstanceID
	} else {
		logger.Warn("Failed to get EC2 instance id document", zap.Error(err))
	}

	metadataOutput, err := client.GetMetadata(ctx, &imds.GetMetadataInput{Path: "hostname"})
	if err != nil {
		logger.Warn("Failed to retrieve EC2 hostname", zap.Error(err))
	} else {
		defer metadataOutput.Content.Close()
		hostnameBytes, readErr := io.ReadAll(metadataOutput.Content)
		if readErr != nil {
			logger.Warn("Failed to read EC2 hostname content", zap.Error(readErr))
		} else {
			hostInfo.EC2Hostname = string(hostnameBytes)
		}
	}

	return
}

func (hi *HostInfo) GetHostname(_ *zap.Logger) string {
	if isDefaultHostname(hi.EC2Hostname) {
		return hi.InstanceID
	}

	return hi.EC2Hostname
}

var (
	_ source.Provider              = (*Provider)(nil)
	_ provider.ClusterNameProvider = (*Provider)(nil)
)

type Provider struct {
	once     sync.Once
	hostInfo HostInfo

	detector ec2provider.Provider
	logger   *zap.Logger
}

func NewProvider(logger *zap.Logger) (*Provider, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}
	return &Provider{
		logger:   logger,
		detector: ec2provider.NewProvider(cfg),
	}, nil
}

func (p *Provider) fillHostInfo(ctx context.Context) {
	p.once.Do(func() { p.hostInfo = *GetHostInfo(ctx, p.logger) })
}

func (p *Provider) Source(ctx context.Context) (source.Source, error) {
	p.fillHostInfo(ctx)
	if p.hostInfo.InstanceID == "" {
		return source.Source{}, errors.New("instance ID is unavailable")
	}

	return source.Source{Kind: source.HostnameKind, Identifier: p.hostInfo.InstanceID}, nil
}

// instanceTags gets the EC2 tags for the current instance.
func (p *Provider) instanceTags(ctx context.Context) (*ec2.DescribeTagsOutput, error) {
	// Get EC2 metadata to find the region and instance ID
	meta, err := p.detector.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	// Get the EC2 tags for the instance id.
	// Similar to:
	// - https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/39dbc1ac8/processor/resourcedetectionprocessor/internal/aws/ec2/ec2.go#L118-L151
	// - https://github.com/DataDog/datadog-agent/blob/1b4afdd6a03e8fabcc169b924931b2bb8935dab9/pkg/util/ec2/ec2_tags.go#L104-L134
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(meta.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := ec2.NewFromConfig(cfg)
	return client.DescribeTags(ctx, &ec2.DescribeTagsInput{
		Filters: []types.Filter{{
			Name:   aws.String("resource-id"),
			Values: []string{meta.InstanceID},
		}},
	})
}

// clusterNameFromTags gets the AWS EC2 Cluster name from the tags on an EC2 instance.
func clusterNameFromTags(ec2Tags *ec2.DescribeTagsOutput) (string, error) {
	// Similar to:
	// - https://github.com/DataDog/datadog-agent/blob/1b4afdd6a03/pkg/util/ec2/ec2.go#L256-L271
	const clusterNameTagPrefix = "kubernetes.io/cluster/"
	for _, tag := range ec2Tags.Tags {
		if strings.HasPrefix(*tag.Key, clusterNameTagPrefix) {
			if len(*tag.Key) == len(clusterNameTagPrefix) {
				return "", fmt.Errorf("missing cluster name in %q tag", *tag.Key)
			}
			return strings.Split(*tag.Key, "/")[2], nil
		}
	}

	return "", fmt.Errorf("no tag found with prefix %q", clusterNameTagPrefix)
}

// ClusterName gets the cluster name from an AWS EC2 machine.
func (p *Provider) ClusterName(ctx context.Context) (string, error) {
	ec2Tags, err := p.instanceTags(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get EC2 instance tags: %w", err)
	}
	return clusterNameFromTags(ec2Tags)
}

func (p *Provider) HostInfo() *HostInfo {
	p.fillHostInfo(context.Background())
	return &p.hostInfo
}
