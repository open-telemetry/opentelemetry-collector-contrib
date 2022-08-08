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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/model/internal/testutils"
)

const (
	testShortHostname = "hostname"
	testHostID        = "hostID"
	testCloudZone     = "zone"
	testHostType      = "machineType"
	testCloudAccount  = "projectID"
	testHostname      = testShortHostname + ".c." + testCloudAccount + ".internal"
	testBadHostname   = "badhostname"
)

var (
	testFullMap = testutils.NewAttributeMap(map[string]string{
		conventions.AttributeCloudProvider:         conventions.AttributeCloudProviderGCP,
		conventions.AttributeHostID:                testHostID,
		conventions.AttributeHostName:              testHostname,
		conventions.AttributeCloudAvailabilityZone: testCloudZone,
		conventions.AttributeHostType:              testHostType,
		conventions.AttributeCloudAccountID:        testCloudAccount,
	})

	testFullBadMap = testutils.NewAttributeMap(map[string]string{
		conventions.AttributeCloudProvider:         conventions.AttributeCloudProviderGCP,
		conventions.AttributeHostID:                testHostID,
		conventions.AttributeHostName:              testBadHostname,
		conventions.AttributeCloudAvailabilityZone: testCloudZone,
		conventions.AttributeHostType:              testHostType,
		conventions.AttributeCloudAccountID:        testCloudAccount,
	})

	testGCPIntegrationHostname    = fmt.Sprintf("%s.%s", testShortHostname, testCloudAccount)
	testGCPIntegrationBadHostname = fmt.Sprintf("%s.%s", testBadHostname, testCloudAccount)
)

func TestInfoFromAttributes(t *testing.T) {
	tags := []string{"instance-id:hostID", "zone:zone", "instance-type:machineType", "project:projectID"}
	tests := []struct {
		name       string
		attrs      pcommon.Map
		usePreview bool

		ok          bool
		hostname    string
		hostAliases []string
		gcpTags     []string
	}{
		{
			name:        "no preview",
			attrs:       testFullMap,
			ok:          true,
			hostname:    testHostname,
			hostAliases: []string{testGCPIntegrationHostname},
			gcpTags:     tags,
		},
		{
			name:  "no hostname, no preview",
			attrs: testutils.NewAttributeMap(map[string]string{}),
		},
		{
			name:       "preview",
			attrs:      testFullMap,
			usePreview: true,
			ok:         true,
			hostname:   testGCPIntegrationHostname,
			gcpTags:    tags,
		},
		{
			name:        "bad hostname, no preview",
			attrs:       testFullBadMap,
			ok:          true,
			hostname:    testBadHostname,
			hostAliases: []string{testGCPIntegrationBadHostname},
			gcpTags:     tags,
		},
		{
			name:       "bad hostname, preview",
			attrs:      testFullBadMap,
			usePreview: true,
			ok:         true,
			hostname:   testGCPIntegrationBadHostname,
			gcpTags:    tags,
		},
		{
			name:       "no hostname, preview",
			attrs:      testutils.NewAttributeMap(map[string]string{}),
			usePreview: true,
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			hostname, ok := HostnameFromAttributes(testInstance.attrs, testInstance.usePreview)
			assert.Equal(t, testInstance.ok, ok)
			assert.Equal(t, testInstance.hostname, hostname)

			hostInfo := HostInfoFromAttributes(testInstance.attrs, testInstance.usePreview)
			assert.ElementsMatch(t, testInstance.hostAliases, hostInfo.HostAliases)
			assert.ElementsMatch(t, testInstance.gcpTags, hostInfo.GCPTags)
		})
	}
}
