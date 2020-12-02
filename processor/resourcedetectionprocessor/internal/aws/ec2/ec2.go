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
	"fmt"

	"github.com/aws/aws-sdk-go/aws/session"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

const (
	TypeStr = "ec2"
)

var _ internal.Detector = (*Detector)(nil)

type Detector struct {
	metadataProvider metadataProvider
}

func NewDetector(component.ProcessorCreateParams) (internal.Detector, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}
	return &Detector{metadataProvider: newMetadataClient(sess)}, nil
}

func (d *Detector) Detect(ctx context.Context) (pdata.Resource, error) {
	res := pdata.NewResource()
	if !d.metadataProvider.available(ctx) {
		return res, nil
	}

	meta, err := d.metadataProvider.get(ctx)
	if err != nil {
		return res, fmt.Errorf("failed getting identity document: %w", err)
	}

	hostname, err := d.metadataProvider.hostname(ctx)
	if err != nil {
		return res, fmt.Errorf("failed getting hostname: %w", err)
	}

	attr := res.Attributes()
	attr.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attr.InsertString("cloud.infrastructure_service", "EC2")
	attr.InsertString(conventions.AttributeCloudRegion, meta.Region)
	attr.InsertString(conventions.AttributeCloudAccount, meta.AccountID)
	attr.InsertString(conventions.AttributeCloudZone, meta.AvailabilityZone)
	attr.InsertString(conventions.AttributeHostID, meta.InstanceID)
	attr.InsertString(conventions.AttributeHostImageID, meta.ImageID)
	attr.InsertString(conventions.AttributeHostType, meta.InstanceType)
	attr.InsertString(conventions.AttributeHostName, hostname)

	return res, nil
}
