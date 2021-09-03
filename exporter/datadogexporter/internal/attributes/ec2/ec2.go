// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ec2

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
)

var (
	defaultPrefixes  = [3]string{"ip-", "domu", "ec2amaz-"}
	ec2TagPrefix     = "ec2.tag."
	clusterTagPrefix = ec2TagPrefix + "kubernetes.io/cluster/"
)

type HostInfo struct {
	InstanceID  string
	EC2Hostname string
	EC2Tags     []string
}

// isDefaultHostname checks if a hostname is an EC2 default
func isDefaultHostname(hostname string) bool {
	for _, val := range defaultPrefixes {
		if strings.HasPrefix(hostname, val) {
			return true
		}
	}

	return false
}

// HostnameFromAttributes gets a valid hostname from labels
// if available
func HostnameFromAttributes(attrs pdata.AttributeMap) (string, bool) {
	hostName, ok := attrs.Get(conventions.AttributeHostName)
	if ok && !isDefaultHostname(hostName.StringVal()) {
		return hostName.StringVal(), true
	}

	if hostID, ok := attrs.Get(conventions.AttributeHostID); ok {
		return hostID.StringVal(), true
	}

	return "", false
}

// HostInfoFromAttributes gets EC2 host info from attributes following
// OpenTelemetry semantic conventions
func HostInfoFromAttributes(attrs pdata.AttributeMap) (hostInfo *HostInfo) {
	hostInfo = &HostInfo{}

	if hostID, ok := attrs.Get(conventions.AttributeHostID); ok {
		hostInfo.InstanceID = hostID.StringVal()
	}

	if hostName, ok := attrs.Get(conventions.AttributeHostName); ok {
		hostInfo.EC2Hostname = hostName.StringVal()
	}

	attrs.Range(func(k string, v pdata.AttributeValue) bool {
		if strings.HasPrefix(k, ec2TagPrefix) {
			tag := fmt.Sprintf("%s:%s", strings.TrimPrefix(k, ec2TagPrefix), v.StringVal())
			hostInfo.EC2Tags = append(hostInfo.EC2Tags, tag)
		}
		return true
	})

	return
}

// ClusterNameFromAttributes gets the AWS cluster name from attributes
func ClusterNameFromAttributes(attrs pdata.AttributeMap) (clusterName string, ok bool) {
	// Get cluster name from tag keys
	// https://github.com/DataDog/datadog-agent/blob/1c94b11/pkg/util/ec2/ec2.go#L238
	attrs.Range(func(k string, _ pdata.AttributeValue) bool {
		if strings.HasPrefix(k, clusterTagPrefix) {
			clusterName = strings.Split(k, "/")[2]
			ok = true
		}
		return true
	})
	return
}
