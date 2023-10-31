// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunk // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
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
func ResourceToHostID(res pcommon.Resource) (HostID, bool) {
	var cloudAccount, hostID, provider string

	attrs := res.Attributes()

	if attrs.Len() == 0 {
		return HostID{}, false
	}

	if attr, ok := attrs.Get(conventions.AttributeCloudAccountID); ok {
		cloudAccount = attr.Str()
	}

	if attr, ok := attrs.Get(conventions.AttributeHostID); ok {
		hostID = attr.Str()
	}

	if attr, ok := attrs.Get(conventions.AttributeCloudProvider); ok {
		provider = attr.Str()
	}

	switch provider {
	case conventions.AttributeCloudProviderAWS:
		var region string
		if attr, ok := attrs.Get(conventions.AttributeCloudRegion); ok {
			region = attr.Str()
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
			ID:  attr.Str(),
		}, true
	}

	return HostID{}, false
}

func azureID(attrs pcommon.Map, cloudAccount string) string {
	var resourceGroupName string
	if attr, ok := attrs.Get("azure.resourcegroup.name"); ok {
		resourceGroupName = attr.Str()
	}
	if resourceGroupName == "" {
		return ""
	}

	var hostname string
	if attr, ok := attrs.Get("azure.vm.name"); ok {
		hostname = attr.Str()
	}
	if hostname == "" {
		return ""
	}

	var vmScaleSetName string
	if attr, ok := attrs.Get("azure.vm.scaleset.name"); ok {
		vmScaleSetName = attr.Str()
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
