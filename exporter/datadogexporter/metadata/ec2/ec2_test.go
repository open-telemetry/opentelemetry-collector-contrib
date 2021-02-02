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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/testutils"
)

const (
	testIP         = "ip-12-34-56-78.us-west-2.compute.internal"
	testDomu       = "domu-12-34-56-78.us-west-2.compute.internal"
	testEC2        = "ec2amaz-12-34-56-78.us-west-2.compute.internal"
	customHost     = "custom-hostname"
	testInstanceID = "i-0123456789"
)

func TestDefaultHostname(t *testing.T) {
	assert.True(t, isDefaultHostname(testIP))
	assert.True(t, isDefaultHostname(testDomu))
	assert.True(t, isDefaultHostname(testEC2))
	assert.False(t, isDefaultHostname(customHost))
}

func TestGetHostname(t *testing.T) {
	logger := zap.NewNop()

	hostInfo := &HostInfo{
		InstanceID:  testInstanceID,
		EC2Hostname: testIP,
	}
	assert.Equal(t, testInstanceID, hostInfo.GetHostname(logger))

	hostInfo = &HostInfo{
		InstanceID:  testInstanceID,
		EC2Hostname: customHost,
	}
	assert.Equal(t, customHost, hostInfo.GetHostname(logger))
}

func TestHostnameFromAttributes(t *testing.T) {
	attrs := testutils.NewAttributeMap(map[string]string{
		conventions.AttributeCloudProvider: conventions.AttributeCloudProviderAWS,
		conventions.AttributeHostID:        testInstanceID,
		conventions.AttributeHostName:      testIP,
	})
	hostname, ok := HostnameFromAttributes(attrs)
	assert.True(t, ok)
	assert.Equal(t, hostname, testInstanceID)
}

func TestHostInfoFromAttributes(t *testing.T) {
	attrs := testutils.NewAttributeMap(map[string]string{
		conventions.AttributeCloudProvider: conventions.AttributeCloudProviderAWS,
		conventions.AttributeHostID:        testInstanceID,
		conventions.AttributeHostName:      testIP,
		"ec2.tag.tag1":                     "val1",
		"ec2.tag.tag2":                     "val2",
		"ignored":                          "ignored",
	})

	hostInfo := HostInfoFromAttributes(attrs)

	assert.Equal(t, hostInfo.InstanceID, testInstanceID)
	assert.Equal(t, hostInfo.EC2Hostname, testIP)
	assert.ElementsMatch(t, hostInfo.EC2Tags, []string{"tag1:val1", "tag2:val2"})
}
