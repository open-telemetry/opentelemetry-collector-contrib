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

package splunk

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
)

var (
	ec2Resource = func() pdata.Resource {
		res := pdata.NewResource()
		res.InitEmpty()
		attr := res.Attributes()
		attr.InsertString(conventions.AttributeCloudProvider, "aws")
		attr.InsertString(conventions.AttributeCloudAccount, "1234")
		attr.InsertString(conventions.AttributeCloudRegion, "us-west-2")
		attr.InsertString(conventions.AttributeHostID, "i-abcd")
		return res
	}()
	ec2WithHost = func() pdata.Resource {
		res := pdata.NewResource()
		res.InitEmpty()
		attr := res.Attributes()
		attr.InsertString(conventions.AttributeCloudProvider, "aws")
		attr.InsertString(conventions.AttributeCloudAccount, "1234")
		attr.InsertString(conventions.AttributeCloudRegion, "us-west-2")
		attr.InsertString(conventions.AttributeHostID, "i-abcd")
		attr.InsertString(conventions.AttributeHostName, "localhost")
		return res
	}()
	ec2PartialResource = func() pdata.Resource {
		res := pdata.NewResource()
		res.InitEmpty()
		attr := res.Attributes()
		attr.InsertString(conventions.AttributeCloudProvider, "aws")
		attr.InsertString(conventions.AttributeHostID, "i-abcd")
		return res
	}()
	gcpResource = func() pdata.Resource {
		res := pdata.NewResource()
		res.InitEmpty()
		attr := res.Attributes()
		attr.InsertString(conventions.AttributeCloudProvider, "gcp")
		attr.InsertString(conventions.AttributeCloudAccount, "1234")
		attr.InsertString(conventions.AttributeHostID, "i-abcd")
		return res
	}()
	gcpPartialResource = func() pdata.Resource {
		res := pdata.NewResource()
		res.InitEmpty()
		attr := res.Attributes()
		attr.InsertString(conventions.AttributeCloudProvider, "gcp")
		attr.InsertString(conventions.AttributeCloudAccount, "1234")
		return res
	}()
	hostResource = func() pdata.Resource {
		res := pdata.NewResource()
		res.InitEmpty()
		attr := res.Attributes()
		attr.InsertString(conventions.AttributeHostName, "localhost")
		return res
	}()
	unknownResource = func() pdata.Resource {
		res := pdata.NewResource()
		res.InitEmpty()
		attr := res.Attributes()
		attr.InsertString(conventions.AttributeCloudProvider, "unknown")
		attr.InsertString(conventions.AttributeCloudAccount, "1234")
		attr.InsertString(conventions.AttributeHostID, "i-abcd")
		return res
	}()
)

func TestResourceToHostID(t *testing.T) {
	type args struct {
		res pdata.Resource
	}
	tests := []struct {
		name string
		args args
		want HostID
		ok   bool
	}{
		{
			name: "ec2",
			args: args{ec2Resource},
			want: HostID{
				Key: "AWSUniqueId",
				ID:  "i-abcd_us-west-2_1234",
			},
			ok: true,
		},
		{
			name: "ec2 with hostname prefers ec2",
			args: args{ec2WithHost},
			want: HostID{
				Key: "AWSUniqueId",
				ID:  "i-abcd_us-west-2_1234",
			},
			ok: true,
		},
		{
			name: "gcp",
			args: args{gcpResource},
			want: HostID{
				Key: "gcp_id",
				ID:  "1234_i-abcd",
			},
			ok: true,
		},
		{
			name: "ec2 attributes missing",
			args: args{ec2PartialResource},
			want: HostID{},
			ok:   false,
		},
		{
			name: "gcp attributes missing",
			args: args{gcpPartialResource},
			want: HostID{},
			ok:   false,
		},
		{
			name: "unknown provider",
			args: args{unknownResource},
			want: HostID{},
			ok:   false,
		},
		{
			name: "host provider",
			args: args{hostResource},
			want: HostID{
				Key: "host.name",
				ID:  "localhost",
			},
			ok: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hostID, ok := ResourceToHostID(tt.args.res)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, hostID)
		})
	}
}
