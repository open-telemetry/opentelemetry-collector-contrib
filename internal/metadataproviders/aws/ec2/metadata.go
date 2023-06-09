// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ec2 // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/aws/ec2"

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
)

const (
	HOSTNAME = "hostname"
)

type Provider interface {
	Get(ctx context.Context) (imds.InstanceIdentityDocument, error)
	Hostname(ctx context.Context) (string, error)
	InstanceID(ctx context.Context) (string, error)
	GetMetadataClient() *MetadataClient
}

type MetadataClient struct {
	clientIMDSV2Only     IMDSAPI
	clientIMDSV1Fallback IMDSAPI
}

type IMDSAPI interface {
	GetInstanceIdentityDocument(ctx context.Context, params *imds.GetInstanceIdentityDocumentInput, optFns ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error)
	GetMetadata(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error)
}

var _ Provider = (*MetadataClient)(nil)

func NewProvider(cfg aws.Config) Provider {
	clientIMDSV2Only, clientIMDSV1Fallback := CreateIMDSV2AndFallbackClient(cfg)
	return &MetadataClient{
		clientIMDSV2Only:     clientIMDSV2Only,
		clientIMDSV1Fallback: clientIMDSV1Fallback,
	}
}

func (c *MetadataClient) GetMetadataClient() *MetadataClient {
	return c
}

func (c *MetadataClient) InstanceID(ctx context.Context) (string, error) {
	document, err := c.clientIMDSV2Only.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		document, err = c.clientIMDSV1Fallback.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
		if err != nil {
			return "", err
		}
	}
	return document.InstanceID, nil
}

func (c *MetadataClient) Hostname(ctx context.Context) (string, error) {
	getMetadataInput := imds.GetMetadataInput{
		Path: HOSTNAME,
	}
	metadata, err := c.clientIMDSV2Only.GetMetadata(context.Background(), &getMetadataInput)
	if err != nil {
		metadata, err = c.clientIMDSV1Fallback.GetMetadata(context.Background(), &getMetadataInput)
		if err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("%v", metadata.ResultMetadata.Get(HOSTNAME)), nil
}

func (c *MetadataClient) Get(ctx context.Context) (imds.InstanceIdentityDocument, error) {
	document, err := c.clientIMDSV2Only.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		document, err = c.clientIMDSV1Fallback.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
		if err != nil {
			return imds.InstanceIdentityDocument{}, err
		}
	}
	return document.InstanceIdentityDocument, nil
}

func CreateIMDSV2AndFallbackClient(cfg aws.Config) (*imds.Client, *imds.Client) {
	optionsIMDSV2Only := func(o *imds.Options) {
		o.EnableFallback = aws.FalseTernary
		o.Retryer = retry.NewStandard(func(options *retry.StandardOptions) {})
	}
	optionsIMDSV1Fallback := func(o *imds.Options) {
		o.EnableFallback = aws.TrueTernary
		o.Retryer = retry.NewStandard(func(options *retry.StandardOptions) {})
	}
	clientIMDSV2Only := imds.NewFromConfig(cfg, optionsIMDSV2Only)
	clientIMDSV1Fallback := imds.NewFromConfig(cfg, optionsIMDSV1Fallback)
	return clientIMDSV2Only, clientIMDSV1Fallback
}
