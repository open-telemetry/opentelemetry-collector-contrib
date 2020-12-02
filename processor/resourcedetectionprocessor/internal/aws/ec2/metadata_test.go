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
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/awstesting/mock"
)

func TestMetadataProvider_get(t *testing.T) {
	type args struct {
		ctx  context.Context
		sess *session.Session
	}
	tests := []struct {
		name    string
		args    args
		wantDoc ec2metadata.EC2InstanceIdentityDocument
		wantErr bool
	}{
		{
			name: "mock session",
			args: args{
				ctx:  context.Background(),
				sess: mock.Session,
			},
			wantDoc: ec2metadata.EC2InstanceIdentityDocument{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newMetadataClient(tt.args.sess)
			gotDoc, err := c.get(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotDoc, tt.wantDoc) {
				t.Errorf("get() gotDoc = %v, want %v", gotDoc, tt.wantDoc)
			}
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
		want   bool
	}{
		{
			name:   "mock session",
			fields: fields{},
			args:   args{ctx: context.Background(), sess: mock.Session},
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newMetadataClient(tt.args.sess)
			if got := c.available(tt.args.ctx); got != tt.want {
				t.Errorf("available() = %v, want %v", got, tt.want)
			}
		})
	}
}
