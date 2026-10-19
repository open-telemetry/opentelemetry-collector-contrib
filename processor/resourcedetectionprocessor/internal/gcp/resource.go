// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_gcp) && !omit_detector_gcp

package gcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp"

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp"

	localMetadata "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp/internal/metadata"
)

// These helpers depend on the GCP detector library, so they live with the
// detector implementation rather than making the metadata package import a
// runtime SDK. They only use exported ResourceBuilder methods.

func setFromCallable(set func(string), detect func() (string, error)) error {
	v, err := detect()
	if err != nil {
		return err
	}
	set(v)
	return nil
}

func setZoneAndRegion(rb *localMetadata.ResourceBuilder, detect func() (string, string, error)) error {
	zone, region, err := detect()
	if err != nil {
		return err
	}
	rb.SetCloudAvailabilityZone(zone)
	rb.SetCloudRegion(region)
	return nil
}

func setZoneOrRegion(rb *localMetadata.ResourceBuilder, detect func() (string, gcp.LocationType, error)) error {
	v, locType, err := detect()
	if err != nil {
		return err
	}
	switch locType {
	case gcp.Zone:
		rb.SetCloudAvailabilityZone(v)
		if idx := strings.LastIndex(v, "-"); idx != -1 {
			rb.SetCloudRegion(v[:idx])
		}
	case gcp.Region:
		rb.SetCloudRegion(v)
	default:
		return fmt.Errorf("location must be zone or region. Got %v", locType)
	}
	return nil
}

func setManagedInstanceGroup(rb *localMetadata.ResourceBuilder, detect func() (gcp.ManagedInstanceGroup, error)) error {
	v, err := detect()
	if err != nil {
		return err
	}
	if v.Name != "" {
		rb.SetGcpGceInstanceGroupManagerName(v.Name)
	}
	switch v.Type {
	case gcp.Zone:
		rb.SetGcpGceInstanceGroupManagerZone(v.Location)
	case gcp.Region:
		rb.SetGcpGceInstanceGroupManagerRegion(v.Location)
	}
	return nil
}
