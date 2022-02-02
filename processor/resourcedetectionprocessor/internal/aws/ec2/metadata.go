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

package ec2 // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ec2"

import (
	"context"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
)

type metadataProvider interface {
	get(ctx context.Context) (ec2metadata.EC2InstanceIdentityDocument, error)
	hostname(ctx context.Context) (string, error)
	instanceID(ctx context.Context) (string, error)
}

type metadataClient struct {
	metadata *ec2metadata.EC2Metadata
}

var _ metadataProvider = (*metadataClient)(nil)

func newMetadataClient(sess *session.Session) *metadataClient {
	return &metadataClient{
		metadata: ec2metadata.New(sess),
	}
}

func (c *metadataClient) instanceID(ctx context.Context) (string, error) {
	return c.metadata.GetMetadataWithContext(ctx, "instance-id")
}

func (c *metadataClient) hostname(ctx context.Context) (string, error) {
	return c.metadata.GetMetadataWithContext(ctx, "hostname")
}

func (c *metadataClient) get(ctx context.Context) (ec2metadata.EC2InstanceIdentityDocument, error) {
	return c.metadata.GetInstanceIdentityDocumentWithContext(ctx)
}
