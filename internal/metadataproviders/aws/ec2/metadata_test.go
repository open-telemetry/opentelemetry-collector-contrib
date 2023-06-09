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

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/awstesting/mock"
	"github.com/aws/smithy-go/middleware"
	"github.com/stretchr/testify/assert"
)

var _ Provider = (*MetadataClient)(nil)

type MockedIMDSAPI struct {
}

func (m *MockedIMDSAPI) GetInstanceIdentityDocument(ctx context.Context, params *imds.GetInstanceIdentityDocumentInput, optFns ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error) {
	return nil, errors.New("any error")
}

func (m *MockedIMDSAPI) GetMetadata(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
	return &imds.GetMetadataOutput{
		Content:        nil,
		ResultMetadata: middleware.Metadata{},
	}, nil
}

func TestMetadataProviderGetError(t *testing.T) {
	type args struct {
		ctx  context.Context
		sess *session.Session
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "mock session",
			args: args{
				ctx:  context.Background(),
				sess: mock.Session,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := config.LoadDefaultConfig(context.Background())
			c := NewProvider(cfg)
			c.GetMetadataClient().clientIMDSV2Only = &MockedIMDSAPI{}
			c.GetMetadataClient().clientIMDSV1Fallback = &MockedIMDSAPI{}
			_, err := c.Get(tt.args.ctx)
			assert.Error(t, err)
		})
	}
}

func TestMetadataProvider_available(t *testing.T) {
	type fields struct {
	}
	type args struct {
		ctx  context.Context
		sess *session.Session
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   error
	}{
		{
			name:   "mock session",
			fields: fields{},
			args:   args{ctx: context.Background(), sess: mock.Session},
			want:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := config.LoadDefaultConfig(context.Background())
			c := NewProvider(cfg)
			c.GetMetadataClient().clientIMDSV1Fallback = &MockedIMDSAPI{}
			c.GetMetadataClient().clientIMDSV1Fallback = &MockedIMDSAPI{}
			_, err := c.Hostname(tt.args.ctx)
			assert.ErrorIs(t, err, tt.want)
		})
	}
}
