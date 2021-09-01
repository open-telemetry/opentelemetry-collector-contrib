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
	"testing"

	"github.com/stretchr/testify/assert"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutils"
)

const (
	testHostname     = "hostname"
	testHostID       = "hostID"
	testCloudZone    = "zone"
	testHostType     = "machineType"
	testCloudAccount = "projectID"
)

func TestHostnameFromAttributes(t *testing.T) {
	attrs := testutils.NewAttributeMap(map[string]string{
		conventions.AttributeCloudProvider: conventions.AttributeCloudProviderGCP,
		conventions.AttributeHostID:        testHostID,
		conventions.AttributeHostName:      testHostname,
	})
	hostname, ok := HostnameFromAttributes(attrs)
	assert.True(t, ok)
	assert.Equal(t, hostname, testHostname)

	attrs = testutils.NewAttributeMap(map[string]string{})
	_, ok = HostnameFromAttributes(attrs)
	assert.False(t, ok)
}

func TestHostInfoFromAttributes(t *testing.T) {
	attrs := testutils.NewAttributeMap(map[string]string{
		conventions.AttributeCloudProvider:         conventions.AttributeCloudProviderGCP,
		conventions.AttributeHostID:                testHostID,
		conventions.AttributeHostName:              testHostname,
		conventions.AttributeCloudAvailabilityZone: testCloudZone,
		conventions.AttributeHostType:              testHostType,
		conventions.AttributeCloudAccountID:        testCloudAccount,
	})
	hostInfo := HostInfoFromAttributes(attrs)
	assert.ElementsMatch(t, hostInfo.HostAliases, []string{testHostID})
	assert.ElementsMatch(t, hostInfo.GCPTags,
		[]string{"instance-id:hostID", "zone:zone", "instance-type:machineType", "project:projectID"})
}
