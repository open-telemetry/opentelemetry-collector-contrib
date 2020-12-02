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
	"testing"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

type mockMetadata struct {
	retIDDoc    ec2metadata.EC2InstanceIdentityDocument
	retErrIDDoc error

	retHostname    string
	retErrHostname error

	isAvailable bool
}

var _ metadataProvider = (*mockMetadata)(nil)

func (mm mockMetadata) available(ctx context.Context) bool {
	return mm.isAvailable
}

func (mm mockMetadata) get(ctx context.Context) (ec2metadata.EC2InstanceIdentityDocument, error) {
	if mm.retErrIDDoc != nil {
		return ec2metadata.EC2InstanceIdentityDocument{}, mm.retErrIDDoc
	}
	return mm.retIDDoc, nil
}

func (mm mockMetadata) hostname(ctx context.Context) (string, error) {
	if mm.retErrHostname != nil {
		return "", mm.retErrHostname
	}
	return mm.retHostname, nil
}

func TestNewDetector(t *testing.T) {
	detector, err := NewDetector(component.ProcessorCreateParams{Logger: zap.NewNop()})
	assert.NotNil(t, detector)
	assert.NoError(t, err)
}

func TestDetector_Detect(t *testing.T) {
	type fields struct {
		metadataProvider metadataProvider
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    pdata.Resource
		wantErr bool
	}{
		{
			name: "success",
			fields: fields{metadataProvider: &mockMetadata{
				retIDDoc: ec2metadata.EC2InstanceIdentityDocument{
					Region:           "us-west-2",
					AccountID:        "account1234",
					AvailabilityZone: "us-west-2a",
					InstanceID:       "i-abcd1234",
					ImageID:          "abcdef",
					InstanceType:     "c4.xlarge",
				},
				retHostname: "example-hostname",
				isAvailable: true}},
			args: args{ctx: context.Background()},
			want: func() pdata.Resource {
				res := pdata.NewResource()
				attr := res.Attributes()
				attr.InsertString("cloud.account.id", "account1234")
				attr.InsertString("cloud.provider", "aws")
				attr.InsertString("cloud.infrastructure_service", "EC2")
				attr.InsertString("cloud.region", "us-west-2")
				attr.InsertString("cloud.zone", "us-west-2a")
				attr.InsertString("host.id", "i-abcd1234")
				attr.InsertString("host.image.id", "abcdef")
				attr.InsertString("host.type", "c4.xlarge")
				attr.InsertString("host.name", "example-hostname")
				return res
			}()},
		{
			name: "endpoint not available",
			fields: fields{metadataProvider: &mockMetadata{
				retIDDoc:    ec2metadata.EC2InstanceIdentityDocument{},
				retErrIDDoc: errors.New("should not be called"),
				isAvailable: false,
			}},
			args: args{ctx: context.Background()},
			want: func() pdata.Resource {
				return pdata.NewResource()
			}(),
			wantErr: false},
		{
			name: "get fails",
			fields: fields{metadataProvider: &mockMetadata{
				retIDDoc:    ec2metadata.EC2InstanceIdentityDocument{},
				retErrIDDoc: errors.New("get failed"),
				isAvailable: true,
			}},
			args: args{ctx: context.Background()},
			want: func() pdata.Resource {
				return pdata.NewResource()
			}(),
			wantErr: true},
		{
			name: "hostname fails",
			fields: fields{metadataProvider: &mockMetadata{
				retIDDoc:       ec2metadata.EC2InstanceIdentityDocument{},
				retHostname:    "",
				retErrHostname: errors.New("hostname failed"),
				isAvailable:    true,
			}},
			args: args{ctx: context.Background()},
			want: func() pdata.Resource {
				return pdata.NewResource()
			}(),
			wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Detector{
				metadataProvider: tt.fields.metadataProvider,
			}
			got, err := d.Detect(tt.args.ctx)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, internal.AttributesToMap(tt.want.Attributes()), internal.AttributesToMap(got.Attributes()))
			}
		})
	}
}
