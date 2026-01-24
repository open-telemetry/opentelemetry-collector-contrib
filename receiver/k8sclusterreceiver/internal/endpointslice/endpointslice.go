// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package endpointslice // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/endpointslice"

import (
	discoveryv1 "k8s.io/api/discovery/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

// Transform transforms the endpointslice to remove the fields that we don't use to reduce RAM utilization.
// IMPORTANT: Make sure to update this function before using new endpointslice fields.
func Transform(eps *discoveryv1.EndpointSlice) *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta:  metadata.TransformObjectMeta(eps.ObjectMeta),
		AddressType: eps.AddressType,
		Endpoints:   eps.Endpoints,
	}
}
