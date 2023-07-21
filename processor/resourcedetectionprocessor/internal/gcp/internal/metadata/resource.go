// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp/internal/metadata"

import (
	"fmt"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp"
)

func (rb *ResourceBuilder) SetFromCallable(set func(string), detect func() (string, error)) error {
	v, err := detect()
	if err != nil {
		return err
	}
	set(v)
	return nil
}

func (rb *ResourceBuilder) SetZoneAndRegion(detect func() (string, string, error)) error {
	zone, region, err := detect()
	if err != nil {
		return err
	}
	rb.SetCloudAvailabilityZone(zone)
	rb.SetCloudRegion(region)
	return nil
}

func (rb *ResourceBuilder) SetZoneOrRegion(detect func() (string, gcp.LocationType, error)) error {
	v, locType, err := detect()
	if err != nil {
		return err
	}
	switch locType {
	case gcp.Zone:
		rb.SetCloudAvailabilityZone(v)
	case gcp.Region:
		rb.SetCloudRegion(v)
	default:
		return fmt.Errorf("location must be zone or region. Got %v", locType)
	}
	return nil
}
