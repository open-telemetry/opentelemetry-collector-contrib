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

package gcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/model/attributes/gcp"

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

// HostInfo holds the GCP host information.
type HostInfo struct {
	HostAliases []string
	GCPTags     []string
}

// HostnameFromAttributes gets a valid hostname from labels
// if available
func HostnameFromAttributes(attrs pcommon.Map) (string, bool) {
	if hostName, ok := attrs.Get(conventions.AttributeHostName); ok {
		return hostName.StringVal(), true
	}

	return "", false
}

// HostInfoFromAttributes gets GCP host info from attributes following
// OpenTelemetry semantic conventions
func HostInfoFromAttributes(attrs pcommon.Map) (hostInfo *HostInfo) {
	hostInfo = &HostInfo{}

	if hostID, ok := attrs.Get(conventions.AttributeHostID); ok {
		hostInfo.GCPTags = append(hostInfo.GCPTags, fmt.Sprintf("instance-id:%s", hostID.StringVal()))
	}

	if cloudZone, ok := attrs.Get(conventions.AttributeCloudAvailabilityZone); ok {
		hostInfo.GCPTags = append(hostInfo.GCPTags, fmt.Sprintf("zone:%s", cloudZone.StringVal()))
	}

	if hostType, ok := attrs.Get(conventions.AttributeHostType); ok {
		hostInfo.GCPTags = append(hostInfo.GCPTags, fmt.Sprintf("instance-type:%s", hostType.StringVal()))
	}

	if cloudAccount, ok := attrs.Get(conventions.AttributeCloudAccountID); ok {
		hostInfo.GCPTags = append(hostInfo.GCPTags, fmt.Sprintf("project:%s", cloudAccount.StringVal()))
	}

	if hostName, ok := attrs.Get(conventions.AttributeHostName); ok {
		if cloudAccount, ok := attrs.Get(conventions.AttributeCloudAccountID); ok {
			name := hostName.StringVal()
			if strings.Count(name, ".") >= 3 {
				// Unless the host.name attribute has been tampered with, use the same logic as the Agent to
				// extract the hostname: https://github.com/DataDog/datadog-agent/blob/7.36.0/pkg/util/cloudproviders/gce/gce.go#L106
				name = strings.SplitN(name, ".", 2)[0]
			}
			alias := fmt.Sprintf("%s.%s", name, cloudAccount.StringVal())
			hostInfo.HostAliases = append(hostInfo.HostAliases, alias)
		}

	}

	return
}
