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

package ec2

import (
	"context"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
)

type ec2MetadataImpl struct {
	sess *session.Session
}

var _ ec2MetadataProvider = (*ec2MetadataImpl)(nil)

func (md *ec2MetadataImpl) available(ctx context.Context) bool {
	meta := ec2metadata.New(md.sess)
	return meta.AvailableWithContext(ctx)
}

func (md *ec2MetadataImpl) hostname(ctx context.Context) (string, error) {
	meta := ec2metadata.New(md.sess)
	return meta.GetMetadataWithContext(ctx, "hostname")
}

func (md *ec2MetadataImpl) get(ctx context.Context) (ec2metadata.EC2InstanceIdentityDocument, error) {
	meta := ec2metadata.New(md.sess)
	return meta.GetInstanceIdentityDocumentWithContext(ctx)
}
