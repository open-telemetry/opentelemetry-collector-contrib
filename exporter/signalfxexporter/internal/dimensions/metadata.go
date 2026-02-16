// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dimensions // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/dimensions"

import (
	"fmt"
	"strings"
	"unicode"

	"go.uber.org/multierr"

	metadata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
)

// MetadataUpdateClient is an interface for pushing metadata updates
type MetadataUpdateClient interface {
	PushMetadata([]*metadata.MetadataUpdate) error
}

func getDimensionUpdateFromMetadata(
	defaults map[string]string,
	metadata metadata.MetadataUpdate,
	nonAlphanumericDimChars string,
) *DimensionUpdate {
	properties, tags := getPropertiesAndTags(defaults, metadata)

	return &DimensionUpdate{
		Name:       FilterKeyChars(metadata.ResourceIDKey, nonAlphanumericDimChars),
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

func getPropertiesAndTags(defaults map[string]string, kmu metadata.MetadataUpdate) (map[string]*string, map[string]bool) {
	properties := map[string]*string{}
	tags := map[string]bool{}

	for k, v := range defaults {
		properties[k] = &v
	}

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
		dimensionUpdate := getDimensionUpdateFromMetadata(dc.DefaultProperties, *m, dc.nonAlphanumericDimChars)

		if dimensionUpdate.Name == "" || dimensionUpdate.Value == "" {
			return fmt.Errorf("dimensionUpdate %v is missing Name or value, cannot send", dimensionUpdate)
		}

		errs = multierr.Append(errs, dc.AcceptDimension(dimensionUpdate))
	}

	return errs
}

// FilterKeyChars filters dimension key characters, replacing non-alphanumeric characters
// (except those in nonAlphanumericDimChars) with underscores.
func FilterKeyChars(str, nonAlphanumericDimChars string) string {
	filterMap := func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(nonAlphanumericDimChars, r) {
			return r
		}
		return '_'
	}

	return strings.Map(filterMap, str)
}
