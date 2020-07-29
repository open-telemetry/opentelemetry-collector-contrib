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
	"errors"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
)

type ec2MetadataImpl struct{}

func (ec2MetadataImpl) get(ctx context.Context) (doc ec2metadata.EC2InstanceIdentityDocument, err error) {
	sess, err := session.NewSession()
	if err != nil {
		return
	}
	meta := ec2metadata.New(sess)

	if !meta.AvailableWithContext(ctx) {
		err = errors.New("EC2 metadata endpoint is not available")
		return
	}

	doc, err = meta.GetInstanceIdentityDocumentWithContext(ctx)
	return
}
