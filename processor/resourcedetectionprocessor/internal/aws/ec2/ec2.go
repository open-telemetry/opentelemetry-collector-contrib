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

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
)

const (
	TypeStr          = "ec2"
	cloudProviderAWS = "aws"
)

type Detector struct {
	provider ec2MetadataProvider
}

func NewDetector() *Detector {
	return &Detector{provider: ec2MetadataImpl{}}
}

func (d *Detector) Detect(ctx context.Context) (pdata.Resource, error) {
	res := pdata.NewResource()
	res.InitEmpty()
	attr := res.Attributes()
	attr.InsertString(conventions.AttributeCloudProvider, cloudProviderAWS)

	meta, err := d.provider.get(ctx)
	if err != nil {
		return res, err
	}

	attr.InsertString(conventions.AttributeCloudRegion, meta.Region)
	attr.InsertString(conventions.AttributeCloudAccount, meta.AccountID)
	attr.InsertString(conventions.AttributeCloudZone, meta.AvailabilityZone)
	attr.InsertString(conventions.AttributeHostID, meta.InstanceID)
	attr.InsertString(conventions.AttributeHostImageID, meta.ImageID)
	attr.InsertString(conventions.AttributeHostType, meta.InstanceType)

	return res, nil
}
