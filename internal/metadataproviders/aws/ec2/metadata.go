// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ec2 // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/aws/ec2"

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
)

type Provider interface {
	Get(ctx context.Context) (imds.InstanceIdentityDocument, error)
	Hostname(ctx context.Context) (string, error)
	InstanceID(ctx context.Context) (string, error)
}

type metadataClient struct {
	client *imds.Client
}

var _ Provider = (*metadataClient)(nil)

func NewProvider(cfg aws.Config) Provider {
	return &metadataClient{
		client: imds.NewFromConfig(cfg),
	}
}

func (c *metadataClient) getMetadata(ctx context.Context, path string) (string, error) {
	output, err := c.client.GetMetadata(ctx, &imds.GetMetadataInput{Path: path})
	if err != nil {
		return "", fmt.Errorf("failed to get %s from IMDS: %w", path, err)
	}
	defer output.Content.Close()

	data, err := io.ReadAll(output.Content)
	if err != nil {
		return "", fmt.Errorf("failed to read %s response: %w", path, err)
	}

	return string(data), nil
}

func (c *metadataClient) InstanceID(ctx context.Context) (string, error) {
	return c.getMetadata(ctx, "instance-id")
}

func (c *metadataClient) Hostname(ctx context.Context) (string, error) {
	return c.getMetadata(ctx, "hostname")
}

func (c *metadataClient) Get(ctx context.Context) (imds.InstanceIdentityDocument, error) {
	output, err := c.client.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		return imds.InstanceIdentityDocument{}, fmt.Errorf("failed to get instance identity document: %w", err)
	}

	return output.InstanceIdentityDocument, nil
}
