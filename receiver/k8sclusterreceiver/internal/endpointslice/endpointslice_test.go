// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package endpointslice

import (
	"testing"

	"github.com/stretchr/testify/assert"
	discoveryv1 "k8s.io/api/discovery/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestTransform(t *testing.T) {
	original := testutils.NewEndpointSlice("1")
	transformed := Transform(original)

	assert.Equal(t, original.Name, transformed.Name)
	assert.Equal(t, original.Namespace, transformed.Namespace)
	assert.Equal(t, original.Labels["kubernetes.io/service-name"], transformed.Labels["kubernetes.io/service-name"])
	assert.Equal(t, discoveryv1.AddressTypeIPv4, transformed.AddressType, "AddressType must be preserved")
	assert.Len(t, transformed.Endpoints, 1)
	assert.Equal(t, original.Endpoints[0].Addresses[0], transformed.Endpoints[0].Addresses[0])
	assert.Equal(t, original.Endpoints[0].Zone, transformed.Endpoints[0].Zone, "Zone must be preserved")
	assert.Equal(t, original.Endpoints[0].Conditions.Ready, transformed.Endpoints[0].Conditions.Ready)
	assert.Equal(t, original.Endpoints[0].Conditions.Serving, transformed.Endpoints[0].Conditions.Serving)
	assert.Equal(t, original.Endpoints[0].Conditions.Terminating, transformed.Endpoints[0].Conditions.Terminating)
}
