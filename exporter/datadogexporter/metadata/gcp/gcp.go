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
package gcp

import (
	"fmt"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
)

type HostInfo struct {
	HostAliases []string
	GCPTags     []string
}

// HostnameFromAttributes gets a valid hostname from labels
// if available
func HostnameFromAttributes(attrs pdata.AttributeMap) (string, bool) {
	if hostName, ok := attrs.Get(conventions.AttributeHostName); ok {
		return hostName.StringVal(), true
	}

	return "", false
}

// HostInfoFromAttributes gets GCP host info from attributes following
// OpenTelemetry semantic conventions
func HostInfoFromAttributes(attrs pdata.AttributeMap) (hostInfo *HostInfo) {
	hostInfo = &HostInfo{}

	if hostID, ok := attrs.Get(conventions.AttributeHostID); ok {
		// Add host id as a host alias to preserve backwards compatibility
		// The Datadog Agent does not do this
		hostInfo.HostAliases = append(hostInfo.HostAliases, hostID.StringVal())
		hostInfo.GCPTags = append(hostInfo.GCPTags, fmt.Sprintf("instance-id:%s", hostID.StringVal()))
	}

	if cloudZone, ok := attrs.Get(conventions.AttributeCloudZone); ok {
		hostInfo.GCPTags = append(hostInfo.GCPTags, fmt.Sprintf("zone:%s", cloudZone.StringVal()))
	}

	if hostType, ok := attrs.Get(conventions.AttributeHostType); ok {
		hostInfo.GCPTags = append(hostInfo.GCPTags, fmt.Sprintf("instance-type:%s", hostType.StringVal()))
	}

	if cloudAccount, ok := attrs.Get(conventions.AttributeCloudAccount); ok {
		hostInfo.GCPTags = append(hostInfo.GCPTags, fmt.Sprintf("project:%s", cloudAccount.StringVal()))
	}

	return
}
