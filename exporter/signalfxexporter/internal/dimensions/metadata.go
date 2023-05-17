// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dimensions // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/dimensions"

import (
	"fmt"
	"strings"

	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation"
	metadata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
)

// MetadataUpdateClient is an interface for pushing metadata updates
type MetadataUpdateClient interface {
	PushMetadata([]*metadata.MetadataUpdate) error
}

func getDimensionUpdateFromMetadata(
	metadata metadata.MetadataUpdate,
	metricsConverter translation.MetricsConverter) *DimensionUpdate {
	properties, tags := getPropertiesAndTags(metadata)

	return &DimensionUpdate{
		Name:       metricsConverter.ConvertDimension(metadata.ResourceIDKey),
		Value:      string(metadata.ResourceID),
		Properties: properties,
		Tags:       tags,
	}
}

const (
	oTelK8sServicePrefix = "k8s.service."
	sfxK8sServicePrefix  = "kubernetes_service_"
)

func sanitizeProperty(property string) string {
	if strings.HasPrefix(property, oTelK8sServicePrefix) {
		return strings.Replace(property, oTelK8sServicePrefix, sfxK8sServicePrefix, 1)
	}
	return property
}

func getPropertiesAndTags(kmu metadata.MetadataUpdate) (map[string]*string, map[string]bool) {
	properties := map[string]*string{}
	tags := map[string]bool{}

	for label, val := range kmu.MetadataToAdd {
		key := sanitizeProperty(label)
		if key == "" {
			continue
		}

		if val == "" {
			tags[key] = true
		} else {
			propVal := val
			properties[key] = &propVal
		}
	}

	for label, val := range kmu.MetadataToRemove {
		key := sanitizeProperty(label)
		if key == "" {
			continue
		}

		if val == "" {
			tags[key] = false
		} else {
			properties[key] = nil
		}
	}

	for label, val := range kmu.MetadataToUpdate {
		key := sanitizeProperty(label)
		if key == "" {
			continue
		}

		// Treat it as a remove if a property update has empty value since
		// this cannot be a tag as tags can either be added or removed but
		// not updated.
		if val == "" {
			properties[key] = nil
		} else {
			propVal := val
			properties[key] = &propVal
		}
	}
	return properties, tags
}

func (dc *DimensionClient) PushMetadata(metadata []*metadata.MetadataUpdate) error {
	var errs error
	for _, m := range metadata {
		dimensionUpdate := getDimensionUpdateFromMetadata(*m, dc.metricsConverter)

		if dimensionUpdate.Name == "" || dimensionUpdate.Value == "" {
			return fmt.Errorf("dimensionUpdate %v is missing Name or value, cannot send", dimensionUpdate)
		}

		errs = multierr.Append(errs, dc.acceptDimension(dimensionUpdate))
	}

	return errs
}
