// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/sumologicprocessor"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestTranslateAttributes(t *testing.T) {
	attributes := pcommon.NewMap()
	attributes.PutStr("host.name", "testing-host")
	attributes.PutStr("host.id", "my-host-id")
	attributes.PutStr("host.type", "my-host-type")
	attributes.PutStr("k8s.cluster.name", "testing-cluster")
	attributes.PutStr("k8s.deployment.name", "my-deployment-name")
	attributes.PutStr("k8s.namespace.name", "my-namespace-name")
	attributes.PutStr("k8s.service.name", "my-service-name, other-service")
	attributes.PutStr("cloud.account.id", "my-account-id")
	attributes.PutStr("cloud.availability_zone", "my-zone")
	attributes.PutStr("cloud.region", "my-region")
	require.Equal(t, 10, attributes.Len())

	translateAttributes(attributes)

	assert.Equal(t, 10, attributes.Len())
	assertAttribute(t, attributes, "host", "testing-host")
	assertAttribute(t, attributes, "host.name", "")
	assertAttribute(t, attributes, "AccountId", "my-account-id")
	assertAttribute(t, attributes, "cloud.account.id", "")
	assertAttribute(t, attributes, "AvailabilityZone", "my-zone")
	assertAttribute(t, attributes, "clout.availability_zone", "")
	assertAttribute(t, attributes, "Region", "my-region")
	assertAttribute(t, attributes, "cloud.region", "")
	assertAttribute(t, attributes, "InstanceId", "my-host-id")
	assertAttribute(t, attributes, "host.id", "")
	assertAttribute(t, attributes, "InstanceType", "my-host-type")
	assertAttribute(t, attributes, "host.type", "")
	assertAttribute(t, attributes, "Cluster", "testing-cluster")
	assertAttribute(t, attributes, "k8s.cluster.name", "")
	assertAttribute(t, attributes, "deployment", "my-deployment-name")
	assertAttribute(t, attributes, "k8s.deployment.name", "")
	assertAttribute(t, attributes, "namespace", "my-namespace-name")
	assertAttribute(t, attributes, "k8s.namespace.name", "")
	assertAttribute(t, attributes, "service", "my-service-name, other-service")
	assertAttribute(t, attributes, "k8s.service.name", "")
}

func TestTranslateAttributesDoesNothingWhenAttributeDoesNotExist(t *testing.T) {
	attributes := pcommon.NewMap()
	require.Equal(t, 0, attributes.Len())

	translateAttributes(attributes)

	assert.Equal(t, 0, attributes.Len())
	assertAttribute(t, attributes, "host", "")
}

func TestTranslateAttributesLeavesOtherAttributesUnchanged(t *testing.T) {
	attributes := pcommon.NewMap()
	attributes.PutStr("one", "one1")
	attributes.PutStr("host.name", "host1")
	attributes.PutStr("three", "three1")
	require.Equal(t, 3, attributes.Len())

	translateAttributes(attributes)

	assert.Equal(t, 3, attributes.Len())
	assertAttribute(t, attributes, "one", "one1")
	assertAttribute(t, attributes, "host", "host1")
	assertAttribute(t, attributes, "three", "three1")
}

func TestTranslateAttributesDoesNotOverwriteExistingAttribute(t *testing.T) {
	attributes := pcommon.NewMap()
	attributes.PutStr("host", "host1")
	attributes.PutStr("host.name", "hostname1")
	require.Equal(t, 2, attributes.Len())

	translateAttributes(attributes)

	assert.Equal(t, 2, attributes.Len())
	assertAttribute(t, attributes, "host", "host1")
	assertAttribute(t, attributes, "host.name", "hostname1")
}

func TestTranslateAttributesDoesNotOverwriteMultipleExistingAttributes(t *testing.T) {
	attributes := pcommon.NewMap()
	attributes.PutStr("host", "host1")
	require.Equal(t, 1, attributes.Len())
	attributes.PutStr("host.name", "hostname1")
	require.Equal(t, 2, attributes.Len())

	translateAttributes(attributes)

	assert.Equal(t, 2, attributes.Len())
	assertAttribute(t, attributes, "host", "host1")
	assertAttribute(t, attributes, "host.name", "hostname1")
}

func assertAttribute(t *testing.T, metadata pcommon.Map, attributeName string, expectedValue string) {
	value, exists := metadata.Get(attributeName)

	if expectedValue == "" {
		assert.False(t, exists)
	} else {
		assert.True(t, exists)
		assert.Equal(t, expectedValue, value.Str())

	}
}

var (
	benchPdataAttributes = map[string]any{
		"host.name":               pcommon.NewValueStr("testing-host"),
		"host.id":                 pcommon.NewValueStr("my-host-id"),
		"host.type":               pcommon.NewValueStr("my-host-type"),
		"k8s.cluster.name":        pcommon.NewValueStr("testing-cluster"),
		"k8s.deployment.name":     pcommon.NewValueStr("my-deployment-name"),
		"k8s.namespace.name":      pcommon.NewValueStr("my-namespace-name"),
		"k8s.service.name":        pcommon.NewValueStr("my-service-name"),
		"cloud.account.id":        pcommon.NewValueStr("my-account-id"),
		"cloud.availability_zone": pcommon.NewValueStr("my-zone"),
		"cloud.region":            pcommon.NewValueStr("my-region"),
		"abc":                     pcommon.NewValueStr("abc"),
		"def":                     pcommon.NewValueStr("def"),
		"xyz":                     pcommon.NewValueStr("xyz"),
		"jkl":                     pcommon.NewValueStr("jkl"),
		"dummy":                   pcommon.NewValueStr("dummy"),
	}
	attributes = pcommon.NewMap()
)

func BenchmarkTranslateAttributes(b *testing.B) {
	err := attributes.FromRaw(benchPdataAttributes)
	require.NoError(b, err)
	for i := 0; i < b.N; i++ {
		translateAttributes(attributes)
	}
}
