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

	override "github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws"
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

func NewProvider(sess *session.Session) Provider {
	return &metadataClient{
		metadata: ec2metadata.New(sess, &aws.Config{
			Retryer: override.IMDSRetryer,
		}),
	}
}

func (c *metadataClient) InstanceID(ctx context.Context) (string, error) {
	childCtx, cancel := context.WithTimeout(ctx, override.TimePerCall)
	defer cancel()
	return c.metadata.GetMetadataWithContext(childCtx, "instance-id")
}

func (c *metadataClient) Hostname(ctx context.Context) (string, error) {
	childCtx, cancel := context.WithTimeout(ctx, override.TimePerCall)
	defer cancel()
	return c.metadata.GetMetadataWithContext(childCtx, "hostname")
}

func (c *metadataClient) Get(ctx context.Context) (ec2metadata.EC2InstanceIdentityDocument, error) {
	childCtx, cancel := context.WithTimeout(ctx, override.TimePerCall)
	defer cancel()
	return c.metadata.GetInstanceIdentityDocumentWithContext(childCtx)
}
