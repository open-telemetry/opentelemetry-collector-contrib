// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunk

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
)

var (
	ec2Resource = func() pcommon.Resource {
		res := pcommon.NewResource()
		attr := res.Attributes()
		attr.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
		attr.PutStr(string(conventions.CloudAccountIDKey), "1234")
		attr.PutStr(string(conventions.CloudRegionKey), "us-west-2")
		attr.PutStr(string(conventions.HostIDKey), "i-abcd")
		return res
	}()
	ec2WithHost = func() pcommon.Resource {
		res := pcommon.NewResource()
		attr := res.Attributes()
		attr.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
		attr.PutStr(string(conventions.CloudAccountIDKey), "1234")
		attr.PutStr(string(conventions.CloudRegionKey), "us-west-2")
		attr.PutStr(string(conventions.HostIDKey), "i-abcd")
		attr.PutStr(string(conventions.HostNameKey), "localhost")
		return res
	}()
	ec2PartialResource = func() pcommon.Resource {
		res := pcommon.NewResource()
		attr := res.Attributes()
		attr.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
		attr.PutStr(string(conventions.HostIDKey), "i-abcd")
		return res
	}()
	gcpResource = func() pcommon.Resource {
		res := pcommon.NewResource()
		attr := res.Attributes()
		attr.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderGCP.Value.AsString())
		attr.PutStr(string(conventions.CloudAccountIDKey), "1234")
		attr.PutStr(string(conventions.HostIDKey), "i-abcd")
		return res
	}()
	gcpPartialResource = func() pcommon.Resource {
		res := pcommon.NewResource()
		attr := res.Attributes()
		attr.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderGCP.Value.AsString())
		attr.PutStr(string(conventions.CloudAccountIDKey), "1234")
		return res
	}()
	azureResource = func() pcommon.Resource {
		res := pcommon.NewResource()
		attrs := res.Attributes()
		attrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAzure.Value.AsString())
		attrs.PutStr(string(conventions.CloudPlatformKey), conventions.CloudPlatformAzureVM.Value.AsString())
		attrs.PutStr(string(conventions.HostNameKey), "myHostName")
		attrs.PutStr(string(conventions.CloudRegionKey), "myCloudRegion")
		attrs.PutStr(string(conventions.HostIDKey), "myHostID")
		attrs.PutStr(string(conventions.CloudAccountIDKey), "myCloudAccount")
		attrs.PutStr("azure.vm.name", "myVMName")
		attrs.PutStr("azure.vm.size", "42")
		attrs.PutStr("azure.resourcegroup.name", "myResourcegroupName")
		return res
	}()
	azureScalesetResource = func() pcommon.Resource {
		res := pcommon.NewResource()
		attrs := res.Attributes()
		attrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAzure.Value.AsString())
		attrs.PutStr(string(conventions.CloudPlatformKey), conventions.CloudPlatformAzureVM.Value.AsString())
		attrs.PutStr(string(conventions.HostNameKey), "my.fq.host.name")
		attrs.PutStr(string(conventions.CloudRegionKey), "myCloudRegion")
		attrs.PutStr(string(conventions.HostIDKey), "myHostID")
		attrs.PutStr(string(conventions.CloudAccountIDKey), "myCloudAccount")
		attrs.PutStr("azure.vm.name", "myVMScalesetName_1")
		attrs.PutStr("azure.vm.size", "42")
		attrs.PutStr("azure.vm.scaleset.name", "myVMScalesetName")
		attrs.PutStr("azure.resourcegroup.name", "myResourcegroupName")
		return res
	}()
	azureMissingCloudAcct = func() pcommon.Resource {
		res := pcommon.NewResource()
		attrs := res.Attributes()
		attrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAzure.Value.AsString())
		attrs.PutStr(string(conventions.CloudPlatformKey), conventions.CloudPlatformAzureVM.Value.AsString())
		attrs.PutStr(string(conventions.CloudRegionKey), "myCloudRegion")
		attrs.PutStr(string(conventions.HostIDKey), "myHostID")
		attrs.PutStr("azure.vm.size", "42")
		attrs.PutStr("azure.resourcegroup.name", "myResourcegroupName")
		return res
	}()
	azureMissingResourceGroup = func() pcommon.Resource {
		res := pcommon.NewResource()
		attrs := res.Attributes()
		attrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAzure.Value.AsString())
		attrs.PutStr(string(conventions.CloudPlatformKey), conventions.CloudPlatformAzureVM.Value.AsString())
		attrs.PutStr(string(conventions.CloudRegionKey), "myCloudRegion")
		attrs.PutStr(string(conventions.HostIDKey), "myHostID")
		attrs.PutStr(string(conventions.CloudAccountIDKey), "myCloudAccount")
		attrs.PutStr("azure.vm.size", "42")
		return res
	}()
	azureMissingHostName = func() pcommon.Resource {
		res := pcommon.NewResource()
		attrs := res.Attributes()
		attrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAzure.Value.AsString())
		attrs.PutStr(string(conventions.CloudPlatformKey), conventions.CloudPlatformAzureVM.Value.AsString())
		attrs.PutStr(string(conventions.CloudRegionKey), "myCloudRegion")
		attrs.PutStr(string(conventions.HostIDKey), "myHostID")
		attrs.PutStr(string(conventions.CloudAccountIDKey), "myCloudAccount")
		attrs.PutStr("azure.resourcegroup.name", "myResourcegroupName")
		attrs.PutStr("azure.vm.size", "42")
		return res
	}()
	hostResource = func() pcommon.Resource {
		res := pcommon.NewResource()
		attr := res.Attributes()
		attr.PutStr(string(conventions.HostNameKey), "localhost")
		return res
	}()
	unknownResource = func() pcommon.Resource {
		res := pcommon.NewResource()
		attr := res.Attributes()
		attr.PutStr(string(conventions.CloudProviderKey), "unknown")
		attr.PutStr(string(conventions.CloudAccountIDKey), "1234")
		attr.PutStr(string(conventions.HostIDKey), "i-abcd")
		return res
	}()
)

func TestResourceToHostID(t *testing.T) {
	type args struct {
		res pcommon.Resource
	}
	tests := []struct {
		name string
		args args
		want HostID
		ok   bool
	}{
		{
			name: "nil resource",
			args: args{pcommon.NewResource()},
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
				ID:  "mycloudaccount/myresourcegroupname/microsoft.compute/virtualmachines/myvmname",
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
