// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dimensions // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/dimensions"

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"go.uber.org/multierr"

	metadata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
)

// MetadataUpdateClient is an interface for pushing metadata updates
type MetadataUpdateClient interface {
	PushMetadata([]*metadata.MetadataUpdate) error
}

// resourceIDsSkipSanitization lists resource ID keys whose properties should
// pass through without label-prefix stripping or k8s.service.* rewriting.
var resourceIDsSkipSanitization = map[string]bool{
	"k8s.service.uid":               true,
	"k8s.persistentvolume.uid":      true,
	"k8s.persistentvolumeclaim.uid": true,
}

func getDimensionUpdateFromMetadata(
	defaults map[string]string,
	metadata metadata.MetadataUpdate,
	nonAlphanumericDimChars string,
	stripK8sLabelPrefix bool,
) *DimensionUpdate {
	skipSanitization := resourceIDsSkipSanitization[metadata.ResourceIDKey]
	properties, tags := getPropertiesAndTags(defaults, metadata, skipSanitization, stripK8sLabelPrefix)

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

var oTelK8sLabelRe = regexp.MustCompile(`^k8s\.[^.]+\.label\.(.+)$`)

func sanitizeProperty(property string, stripK8sLabelPrefix bool) string {
	if strings.HasPrefix(property, oTelK8sServicePrefix) {
		return strings.Replace(property, oTelK8sServicePrefix, sfxK8sServicePrefix, 1)
	}
	if stripK8sLabelPrefix {
		if m := oTelK8sLabelRe.FindStringSubmatch(property); m != nil {
			return m[1]
		}
	}
	return property
}

func getPropertiesAndTags(defaults map[string]string, kmu metadata.MetadataUpdate, skipSanitization, stripK8sLabelPrefix bool) (map[string]*string, map[string]bool) {
	properties := map[string]*string{}
	tags := map[string]bool{}

	for k, v := range defaults {
		properties[k] = &v
	}

	for label, val := range kmu.MetadataToAdd {
		key := label
		if !skipSanitization {
			key = sanitizeProperty(label, stripK8sLabelPrefix)
		}
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
		key := label
		if !skipSanitization {
			key = sanitizeProperty(label, stripK8sLabelPrefix)
		}
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
		key := label
		if !skipSanitization {
			key = sanitizeProperty(label, stripK8sLabelPrefix)
		}
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
		dimensionUpdate := getDimensionUpdateFromMetadata(dc.DefaultProperties, *m, dc.nonAlphanumericDimChars, dc.stripK8sLabelPrefix)

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
