// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ec2 // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/aws/ec2"

import (
	"context"

	"github.com/aws/aws-sdk-go/aws/request"

	override "github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
)

type Provider interface {
	Get(ctx context.Context) (ec2metadata.EC2InstanceIdentityDocument, error)
	GetHandlers() *request.Handlers
	Hostname(ctx context.Context) (string, error)
	InstanceID(ctx context.Context) (string, error)
}

type metadataClient struct {
	metadata               *ec2metadata.EC2Metadata
	metadataFallbackEnable *ec2metadata.EC2Metadata
}

var _ Provider = (*metadataClient)(nil)

func NewProvider(sess *session.Session) Provider {
	return &metadataClient{
		metadata: ec2metadata.New(sess, &aws.Config{
			Retryer:                   override.NewIMDSRetryer(override.DefaultIMDSRetries),
			EC2MetadataEnableFallback: aws.Bool(false),
		}),
		metadataFallbackEnable: ec2metadata.New(sess, &aws.Config{}),
	}
}

func (c *metadataClient) InstanceID(_ context.Context) (string, error) {
	instanceID, err := c.metadata.GetMetadata("instance-id")
	if err == nil {
		return instanceID, err
	}
	return c.metadataFallbackEnable.GetMetadata("instance-id")
}

func (c *metadataClient) Hostname(_ context.Context) (string, error) {
	hostname, err := c.metadata.GetMetadata("hostname")
	if err == nil {
		return hostname, err
	}
	return c.metadataFallbackEnable.GetMetadata("hostname")
}

func (c *metadataClient) Get(_ context.Context) (ec2metadata.EC2InstanceIdentityDocument, error) {
	document, err := c.metadata.GetInstanceIdentityDocument()
	if err == nil {
		return document, err
	}
	return c.metadataFallbackEnable.GetInstanceIdentityDocument()
}

func (c *metadataClient) GetHandlers() *request.Handlers {
	return &c.metadata.Handlers
}
