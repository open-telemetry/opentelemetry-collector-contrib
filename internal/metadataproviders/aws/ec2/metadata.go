// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ec2 // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/aws/ec2"

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
)

type Provider interface {
	Get(ctx context.Context) (ec2metadata.EC2InstanceIdentityDocument, error)
	Hostname(ctx context.Context) (string, error)
	InstanceID(ctx context.Context) (string, error)
}

type metadataClient struct {
	metadata *ec2metadata.EC2Metadata
}

var _ Provider = (*metadataClient)(nil)

// New Provider returns a new AWS EC2 metadata provider. The provider is established
// with a custom logger to suppress any SDK logs, in order to work around an issue in the AWS SDK Go
// (https://github.com/aws/aws-sdk-go/issues/5116), which causes unwanted output to appear on
// the stdout. This change can be reverted once the issue is resolved upstream and a new version
// of the SDK is used in the provider.
func NewProvider(sess *session.Session) Provider {
	return &metadataClient{
		metadata: ec2metadata.New(sess, aws.NewConfig().WithLogger(aws.LoggerFunc(func(args ...any) {}))),
	}
}

func (c *metadataClient) InstanceID(ctx context.Context) (string, error) {
	return c.metadata.GetMetadataWithContext(ctx, "instance-id")
}

func (c *metadataClient) Hostname(ctx context.Context) (string, error) {
	return c.metadata.GetMetadataWithContext(ctx, "hostname")
}

func (c *metadataClient) Get(ctx context.Context) (ec2metadata.EC2InstanceIdentityDocument, error) {
	return c.metadata.GetInstanceIdentityDocumentWithContext(ctx)
}
