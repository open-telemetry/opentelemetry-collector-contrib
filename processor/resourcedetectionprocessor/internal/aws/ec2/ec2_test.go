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
	"regexp"
	"testing"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"

	ec2provider "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/aws/ec2"
)

var errUnavailable = errors.New("ec2metadata unavailable")

type mockMetadata struct {
	retIDDoc    ec2metadata.EC2InstanceIdentityDocument
	retErrIDDoc error

	retHostname    string
	retErrHostname error

	isAvailable bool
}

var _ ec2provider.Provider = (*mockMetadata)(nil)

func (mm mockMetadata) InstanceID(_ context.Context) (string, error) {
	if !mm.isAvailable {
		return "", errUnavailable
	}
	return "", nil
}

func (mm mockMetadata) Get(_ context.Context) (ec2metadata.EC2InstanceIdentityDocument, error) {
	if mm.retErrIDDoc != nil {
		return ec2metadata.EC2InstanceIdentityDocument{}, mm.retErrIDDoc
	}
	return mm.retIDDoc, nil
}

func (mm mockMetadata) Hostname(_ context.Context) (string, error) {
	if mm.retErrHostname != nil {
		return "", mm.retErrHostname
	}
	return mm.retHostname, nil
}

func TestNewDetector(t *testing.T) {
	tests := []struct {
		name        string
		cfg         Config
		shouldError bool
	}{
		{
			name:        "Success Case Empty Config",
			cfg:         Config{},
			shouldError: false,
		},
		{
			name: "Success Case Valid Config",
			cfg: Config{
				Tags: []string{"tag1"},
			},
			shouldError: false,
		},
		{
			name: "Error Case Invalid Regex",
			cfg: Config{
				Tags: []string{"*"},
			},
			shouldError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector, err := NewDetector(processortest.NewNopCreateSettings(), tt.cfg)
			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, detector)
			} else {
				assert.NotNil(t, detector)
				assert.NoError(t, err)
			}
		})
	}
}

func TestDetector_Detect(t *testing.T) {
	type fields struct {
		metadataProvider ec2provider.Provider
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    pcommon.Resource
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
			want: func() pcommon.Resource {
				res := pcommon.NewResource()
				attr := res.Attributes()
				attr.PutStr("cloud.account.id", "account1234")
				attr.PutStr("cloud.provider", "aws")
				attr.PutStr("cloud.platform", "aws_ec2")
				attr.PutStr("cloud.region", "us-west-2")
				attr.PutStr("cloud.availability_zone", "us-west-2a")
				attr.PutStr("host.id", "i-abcd1234")
				attr.PutStr("host.image.id", "abcdef")
				attr.PutStr("host.type", "c4.xlarge")
				attr.PutStr("host.name", "example-hostname")
				return res
			}()},
		{
			name: "endpoint not available",
			fields: fields{metadataProvider: &mockMetadata{
				retIDDoc:    ec2metadata.EC2InstanceIdentityDocument{},
				retErrIDDoc: errors.New("should not be called"),
				isAvailable: false,
			}},
			args:    args{ctx: context.Background()},
			want:    pcommon.NewResource(),
			wantErr: false},
		{
			name: "get fails",
			fields: fields{metadataProvider: &mockMetadata{
				retIDDoc:    ec2metadata.EC2InstanceIdentityDocument{},
				retErrIDDoc: errors.New("get failed"),
				isAvailable: true,
			}},
			args:    args{ctx: context.Background()},
			want:    pcommon.NewResource(),
			wantErr: true},
		{
			name: "hostname fails",
			fields: fields{metadataProvider: &mockMetadata{
				retIDDoc:       ec2metadata.EC2InstanceIdentityDocument{},
				retHostname:    "",
				retErrHostname: errors.New("hostname failed"),
				isAvailable:    true,
			}},
			args:    args{ctx: context.Background()},
			want:    pcommon.NewResource(),
			wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Detector{
				metadataProvider: tt.fields.metadataProvider,
				logger:           zap.NewNop(),
			}
			got, _, err := d.Detect(tt.args.ctx)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, tt.want.Attributes().AsRaw(), got.Attributes().AsRaw())
			}
		})
	}
}

// Define a mock client to mock connecting to an EC2 instance
type mockEC2Client struct {
	ec2iface.EC2API
}

// override the DescribeTags function to mock the output from an actual EC2 instance
func (m *mockEC2Client) DescribeTags(input *ec2.DescribeTagsInput) (*ec2.DescribeTagsOutput, error) {
	if *input.Filters[0].Values[0] == "error" {
		return nil, errors.New("error")
	}

	tag1 := "tag1"
	tag2 := "tag2"
	resource1 := "resource1"
	val1 := "val1"
	val2 := "val2"
	resourceType := "type"

	return &ec2.DescribeTagsOutput{
		Tags: []*ec2.TagDescription{
			{Key: &tag1, ResourceId: &resource1, ResourceType: &resourceType, Value: &val1},
			{Key: &tag2, ResourceId: &resource1, ResourceType: &resourceType, Value: &val2},
		},
	}, nil
}

func TestEC2Tags(t *testing.T) {
	tests := []struct {
		name           string
		tagKeyRegexes  []*regexp.Regexp
		resourceID     string
		expectedOutput map[string]string
		shouldError    bool
	}{
		{
			name:          "success case one tag specified",
			tagKeyRegexes: []*regexp.Regexp{regexp.MustCompile("^tag1$")},
			resourceID:    "resource1",
			expectedOutput: map[string]string{
				"tag1": "val1",
			},
			shouldError: false,
		},
		{
			name:          "success case all tags",
			tagKeyRegexes: []*regexp.Regexp{regexp.MustCompile(".*")},
			resourceID:    "resource1",
			expectedOutput: map[string]string{
				"tag1": "val1",
				"tag2": "val2",
			},
			shouldError: false,
		},
		{
			name:          "error case in DescribeTags",
			tagKeyRegexes: []*regexp.Regexp{regexp.MustCompile("^tag2$")},
			resourceID:    "error",
			expectedOutput: map[string]string{
				"tag1": "val1",
				"tag2": "val2",
			},
			shouldError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mockEC2Client{}
			output, err := fetchEC2Tags(m, tt.resourceID, tt.tagKeyRegexes)
			if tt.shouldError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, output, tt.expectedOutput)
		})
	}
}
