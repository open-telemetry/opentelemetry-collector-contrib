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
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"
)

var (
	defaultPrefixes = [3]string{"ip-", "domu", "ec2amaz-"}
	ec2TagPrefix    = "ec2.tag."
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

// GetHostInfo gets the hostname info from EC2 metadata
func GetHostInfo(logger *zap.Logger) (hostInfo *HostInfo) {
	sess, err := session.NewSession()
	hostInfo = &HostInfo{}

	if err != nil {
		logger.Warn("Failed to build AWS session", zap.Error(err))
		return
	}

	meta := ec2metadata.New(sess)

	if !meta.Available() {
		logger.Debug("EC2 Metadata not available")
		return
	}

	if idDoc, err := meta.GetInstanceIdentityDocument(); err == nil {
		hostInfo.InstanceID = idDoc.InstanceID
	} else {
		logger.Warn("Failed to get EC2 instance id document", zap.Error(err))
	}

	if ec2Hostname, err := meta.GetMetadata("hostname"); err == nil {
		hostInfo.EC2Hostname = ec2Hostname
	} else {
		logger.Warn("Failed to get EC2 hostname", zap.Error(err))
	}

	return
}

func (hi *HostInfo) GetHostname(logger *zap.Logger) string {
	if isDefaultHostname(hi.EC2Hostname) {
		return hi.InstanceID
	}

	return hi.EC2Hostname
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

	attrs.ForEach(func(k string, v pdata.AttributeValue) {
		if strings.HasPrefix(k, ec2TagPrefix) {
			tag := fmt.Sprintf("%s:%s", strings.TrimPrefix(k, ec2TagPrefix), v.StringVal())
			hostInfo.EC2Tags = append(hostInfo.EC2Tags, tag)
		}
	})

	return
}
