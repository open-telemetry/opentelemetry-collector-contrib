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
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
)

// HostIDKey represents a host identifier.
type HostIDKey string

const (
	// HostIDKeyAWS AWS HostIDKey.
	HostIDKeyAWS HostIDKey = "AWSUniqueId"
	// HostIDKeyGCP GCP HostIDKey.
	HostIDKeyGCP HostIDKey = "gcp_id"
	// HostIDKeyAzure Azure HostIDKey.
	HostIDKeyAzure HostIDKey = "azure_resource_id"
	// HostIDKeyHost Host HostIDKey.
	HostIDKeyHost HostIDKey = conventions.AttributeHostName
)

// HostID is a unique key and value (usually used as a dimension) to uniquely identify a host
// using metadata about a cloud instance.
type HostID struct {
	// Key is the key name/type.
	Key HostIDKey
	// Value is the unique ID.
	ID string
}

// ResourceToHostID returns a boolean determining whether or not a HostID was able to be
// computed or not.
func ResourceToHostID(res pdata.Resource) (HostID, bool) {
	var cloudAccount, hostID, provider string

	attrs := res.Attributes()

	if attrs.Len() == 0 {
		return HostID{}, false
	}

	if attr, ok := attrs.Get(conventions.AttributeCloudAccountID); ok {
		cloudAccount = attr.StringVal()
	}

	if attr, ok := attrs.Get(conventions.AttributeHostID); ok {
		hostID = attr.StringVal()
	}

	if attr, ok := attrs.Get(conventions.AttributeCloudProvider); ok {
		provider = attr.StringVal()
	}

	switch provider {
	case conventions.AttributeCloudProviderAWS:
		var region string
		if attr, ok := attrs.Get(conventions.AttributeCloudRegion); ok {
			region = attr.StringVal()
		}
		if hostID == "" || region == "" || cloudAccount == "" {
			break
		}
		return HostID{
			Key: HostIDKeyAWS,
			ID:  fmt.Sprintf("%s_%s_%s", hostID, region, cloudAccount),
		}, true
	case conventions.AttributeCloudProviderGCP:
		if cloudAccount == "" || hostID == "" {
			break
		}
		return HostID{
			Key: HostIDKeyGCP,
			ID:  fmt.Sprintf("%s_%s", cloudAccount, hostID),
		}, true
	case conventions.AttributeCloudProviderAzure:
		if cloudAccount == "" {
			break
		}
		id := azureID(attrs, cloudAccount)
		if id == "" {
			break
		}
		return HostID{
			Key: HostIDKeyAzure,
			ID:  id,
		}, true
	}

	if attr, ok := attrs.Get(conventions.AttributeHostName); ok {
		return HostID{
			Key: HostIDKeyHost,
			ID:  attr.StringVal(),
		}, true
	}

	return HostID{}, false
}

func azureID(attrs pdata.AttributeMap, cloudAccount string) string {
	var resourceGroupName string
	if attr, ok := attrs.Get("azure.resourcegroup.name"); ok {
		resourceGroupName = attr.StringVal()
	}
	if resourceGroupName == "" {
		return ""
	}

	var hostname string
	if attr, ok := attrs.Get(conventions.AttributeHostName); ok {
		hostname = attr.StringVal()
	}
	if hostname == "" {
		return ""
	}

	var vmScaleSetName string
	if attr, ok := attrs.Get("azure.vm.scaleset.name"); ok {
		vmScaleSetName = attr.StringVal()
	}
	if vmScaleSetName == "" {
		return strings.ToLower(fmt.Sprintf(
			"%s/%s/microsoft.compute/virtualmachines/%s",
			cloudAccount,
			resourceGroupName,
			hostname,
		))
	}

	instanceID := strings.TrimPrefix(hostname, vmScaleSetName+"_")
	return strings.ToLower(fmt.Sprintf(
		"%s/%s/microsoft.compute/virtualmachinescalesets/%s/virtualmachines/%s",
		cloudAccount,
		resourceGroupName,
		vmScaleSetName,
		instanceID,
	))
}
