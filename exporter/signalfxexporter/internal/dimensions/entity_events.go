// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dimensions // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/dimensions"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	metadata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
)

type EntityEventTransformer struct {
	defaultProperties map[string]string
}

func NewEntityEventTransformer(defaultProperties map[string]string) *EntityEventTransformer {
	return &EntityEventTransformer{
		defaultProperties: defaultProperties,
	}
}

func (t *EntityEventTransformer) TransformEntityEvent(event metadata.EntityEvent) (*DimensionUpdate, error) {
	if event.EventType() != metadata.EventTypeState {
		return nil, nil
	}

	state := event.EntityStateDetails()
	entityType := state.EntityType()
	entityID := event.ID()

	dimKey, dimValue, err := extractDimensionKeyValue(entityType, entityID)
	if err != nil {
		return nil, err
	}

	properties, tags := t.extractPropertiesAndTags(state.Attributes())

	return &DimensionUpdate{
		Name:       dimKey,
		Value:      dimValue,
		Properties: properties,
		Tags:       tags,
	}, nil
}

func (t *EntityEventTransformer) extractPropertiesAndTags(attrs pcommon.Map) (map[string]*string, map[string]bool) {
	properties := make(map[string]*string)
	tags := make(map[string]bool)

	for k, v := range t.defaultProperties {
		properties[k] = &v
	}

	attrs.Range(func(k string, v pcommon.Value) bool {
		key := sanitizeProperty(k)
		if key == "" {
			return true
		}

		valueStr := v.AsString()

		if valueStr == "" {
			tags[key] = true
		} else {
			propVal := valueStr
			properties[key] = &propVal
		}

		return true
	})

	return properties, tags
}

func extractDimensionKeyValue(entityType string, entityID pcommon.Map) (string, string, error) {
	switch entityType {
	case "container":
		if v, ok := entityID.Get("container.id"); ok {
			return "container.id", v.Str(), nil
		}
	case "k8s.cronjob":
		if v, ok := entityID.Get("k8s.cronjob.uid"); ok {
			return "k8s.cronjob.uid", v.Str(), nil
		}
	case "k8s.daemonset":
		if v, ok := entityID.Get("k8s.daemonset.uid"); ok {
			return "k8s.daemonset.uid", v.Str(), nil
		}
	case "k8s.deployment":
		if v, ok := entityID.Get("k8s.deployment.uid"); ok {
			return "k8s.deployment.uid", v.Str(), nil
		}
	case "k8s.hpa":
		if v, ok := entityID.Get("k8s.hpa.uid"); ok {
			return "k8s.hpa.uid", v.Str(), nil
		}
	case "k8s.job":
		if v, ok := entityID.Get("k8s.job.uid"); ok {
			return "k8s.job.uid", v.Str(), nil
		}
	case "k8s.namespace":
		if v, ok := entityID.Get("k8s.namespace.uid"); ok {
			return "k8s.namespace.uid", v.Str(), nil
		}
	case "k8s.node":
		if v, ok := entityID.Get("k8s.node.uid"); ok {
			return "k8s.node.uid", v.Str(), nil
		}
	case "k8s.pod":
		if v, ok := entityID.Get("k8s.pod.uid"); ok {
			return "k8s.pod.uid", v.Str(), nil
		}
	case "k8s.replicaset":
		if v, ok := entityID.Get("k8s.replicaset.uid"); ok {
			return "k8s.replicaset.uid", v.Str(), nil
		}
	case "k8s.replicationcontroller":
		if v, ok := entityID.Get("k8s.replicationcontroller.uid"); ok {
			return "k8s.replicationcontroller.uid", v.Str(), nil
		}
	case "k8s.statefulset":
		if v, ok := entityID.Get("k8s.statefulset.uid"); ok {
			return "k8s.statefulset.uid", v.Str(), nil
		}
	default:
		return "", "", fmt.Errorf("unsupported entity type: %s", entityType)
	}

	return "", "", fmt.Errorf("entity ID not found for type: %s", entityType)
}
