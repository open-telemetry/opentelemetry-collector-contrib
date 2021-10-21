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
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
)

var (
	ec2Resource = func() pdata.Resource {
		res := pdata.NewResource()
		attr := res.Attributes()
		attr.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
		attr.InsertString(conventions.AttributeCloudAccountID, "1234")
		attr.InsertString(conventions.AttributeCloudRegion, "us-west-2")
		attr.InsertString(conventions.AttributeHostID, "i-abcd")
		return res
	}()
	ec2WithHost = func() pdata.Resource {
		res := pdata.NewResource()
		attr := res.Attributes()
		attr.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
		attr.InsertString(conventions.AttributeCloudAccountID, "1234")
		attr.InsertString(conventions.AttributeCloudRegion, "us-west-2")
		attr.InsertString(conventions.AttributeHostID, "i-abcd")
		attr.InsertString(conventions.AttributeHostName, "localhost")
		return res
	}()
	ec2PartialResource = func() pdata.Resource {
		res := pdata.NewResource()
		attr := res.Attributes()
		attr.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
		attr.InsertString(conventions.AttributeHostID, "i-abcd")
		return res
	}()
	gcpResource = func() pdata.Resource {
		res := pdata.NewResource()
		attr := res.Attributes()
		attr.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderGCP)
		attr.InsertString(conventions.AttributeCloudAccountID, "1234")
		attr.InsertString(conventions.AttributeHostID, "i-abcd")
		return res
	}()
	gcpPartialResource = func() pdata.Resource {
		res := pdata.NewResource()
		attr := res.Attributes()
		attr.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderGCP)
		attr.InsertString(conventions.AttributeCloudAccountID, "1234")
		return res
	}()
	azureResource = func() pdata.Resource {
		res := pdata.NewResource()
		attrs := res.Attributes()
		attrs.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAzure)
		attrs.InsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAzureVM)
		attrs.InsertString(conventions.AttributeHostName, "myHostName")
		attrs.InsertString(conventions.AttributeCloudRegion, "myCloudRegion")
		attrs.InsertString(conventions.AttributeHostID, "myHostID")
		attrs.InsertString(conventions.AttributeCloudAccountID, "myCloudAccount")
		attrs.InsertString("azure.vm.size", "42")
		attrs.InsertString("azure.resourcegroup.name", "myResourcegroupName")
		return res
	}()
	azureScalesetResource = func() pdata.Resource {
		res := pdata.NewResource()
		attrs := res.Attributes()
		attrs.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAzure)
		attrs.InsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAzureVM)
		attrs.InsertString(conventions.AttributeHostName, "myVMScalesetName_1")
		attrs.InsertString(conventions.AttributeCloudRegion, "myCloudRegion")
		attrs.InsertString(conventions.AttributeHostID, "myHostID")
		attrs.InsertString(conventions.AttributeCloudAccountID, "myCloudAccount")
		attrs.InsertString("azure.vm.size", "42")
		attrs.InsertString("azure.vm.scaleset.name", "myVMScalesetName")
		attrs.InsertString("azure.resourcegroup.name", "myResourcegroupName")
		return res
	}()
	azureMissingCloudAcct = func() pdata.Resource {
		res := pdata.NewResource()
		attrs := res.Attributes()
		attrs.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAzure)
		attrs.InsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAzureVM)
		attrs.InsertString(conventions.AttributeCloudRegion, "myCloudRegion")
		attrs.InsertString(conventions.AttributeHostID, "myHostID")
		attrs.InsertString("azure.vm.size", "42")
		attrs.InsertString("azure.resourcegroup.name", "myResourcegroupName")
		return res
	}()
	azureMissingResourceGroup = func() pdata.Resource {
		res := pdata.NewResource()
		attrs := res.Attributes()
		attrs.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAzure)
		attrs.InsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAzureVM)
		attrs.InsertString(conventions.AttributeCloudRegion, "myCloudRegion")
		attrs.InsertString(conventions.AttributeHostID, "myHostID")
		attrs.InsertString(conventions.AttributeCloudAccountID, "myCloudAccount")
		attrs.InsertString("azure.vm.size", "42")
		return res
	}()
	azureMissingHostName = func() pdata.Resource {
		res := pdata.NewResource()
		attrs := res.Attributes()
		attrs.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAzure)
		attrs.InsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAzureVM)
		attrs.InsertString(conventions.AttributeCloudRegion, "myCloudRegion")
		attrs.InsertString(conventions.AttributeHostID, "myHostID")
		attrs.InsertString(conventions.AttributeCloudAccountID, "myCloudAccount")
		attrs.InsertString("azure.resourcegroup.name", "myResourcegroupName")
		attrs.InsertString("azure.vm.size", "42")
		return res
	}()
	hostResource = func() pdata.Resource {
		res := pdata.NewResource()
		attr := res.Attributes()
		attr.InsertString(conventions.AttributeHostName, "localhost")
		return res
	}()
	unknownResource = func() pdata.Resource {
		res := pdata.NewResource()
		attr := res.Attributes()
		attr.InsertString(conventions.AttributeCloudProvider, "unknown")
		attr.InsertString(conventions.AttributeCloudAccountID, "1234")
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
			name: "nil resource",
			args: args{pdata.NewResource()},
			want: HostID{},
			ok:   false,
		},
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
			name: "azure",
			args: args{azureResource},
			want: HostID{
				Key: "azure_resource_id",
				ID:  "mycloudaccount/myresourcegroupname/microsoft.compute/virtualmachines/myhostname",
			},
			ok: true,
		},
		{
			name: "azure scaleset",
			args: args{azureScalesetResource},
			want: HostID{
				Key: "azure_resource_id",
				ID:  "mycloudaccount/myresourcegroupname/microsoft.compute/virtualmachinescalesets/myvmscalesetname/virtualmachines/1",
			},
			ok: true,
		},
		{
			name: "azure cloud account missing",
			args: args{azureMissingCloudAcct},
			want: HostID{},
			ok:   false,
		},
		{
			name: "azure resource group missing",
			args: args{azureMissingResourceGroup},
			want: HostID{},
			ok:   false,
		},
		{
			name: "azure hostname missing",
			args: args{azureMissingHostName},
			want: HostID{},
			ok:   false,
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

func TestAzureID(t *testing.T) {
	attrs := pdata.NewAttributeMap()
	attrs.Insert("azure.resourcegroup.name", pdata.NewAttributeValueString("myResourceGroup"))
	attrs.Insert("azure.vm.scaleset.name", pdata.NewAttributeValueString("myScalesetName"))
	attrs.Insert(conventions.AttributeHostName, pdata.NewAttributeValueString("myScalesetName_1"))
	id := azureID(attrs, "myCloudAccount")
	expected := "mycloudaccount/myresourcegroup/microsoft.compute/virtualmachinescalesets/myscalesetname/virtualmachines/1"
	assert.Equal(t, expected, id)
}
