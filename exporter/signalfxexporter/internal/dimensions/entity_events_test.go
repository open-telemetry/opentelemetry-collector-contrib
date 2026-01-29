// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dimensions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metadata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
)

func TestEntityEventTransformer_TransformEntityEvent(t *testing.T) {
	defaultProps := map[string]string{
		"default_prop": "default_value",
	}

	transformer := NewEntityEventTransformer(defaultProps)

	tests := []struct {
		name           string
		entityType     string
		entityIDKey    string
		entityIDValue  string
		attributes     map[string]any
		wantDimKey     string
		wantDimValue   string
		wantProperties map[string]*string
		wantTags       map[string]bool
		wantErr        bool
	}{
		{
			name:          "k8s.pod with properties and tags",
			entityType:    "k8s.pod",
			entityIDKey:   "k8s.pod.uid",
			entityIDValue: "pod-123",
			attributes: map[string]any{
				"k8s.pod.name":       "my-pod",
				"k8s.namespace.name": "default",
				"app":                "web",
				"empty_tag":          "",
			},
			wantDimKey:   "k8s.pod.uid",
			wantDimValue: "pod-123",
			wantProperties: map[string]*string{
				"default_prop":       strPtr("default_value"),
				"k8s.pod.name":       strPtr("my-pod"),
				"k8s.namespace.name": strPtr("default"),
				"app":                strPtr("web"),
			},
			wantTags: map[string]bool{
				"empty_tag": true,
			},
		},
		{
			name:          "k8s.node",
			entityType:    "k8s.node",
			entityIDKey:   "k8s.node.uid",
			entityIDValue: "node-456",
			attributes: map[string]any{
				"k8s.node.name": "worker-1",
			},
			wantDimKey:   "k8s.node.uid",
			wantDimValue: "node-456",
			wantProperties: map[string]*string{
				"default_prop":  strPtr("default_value"),
				"k8s.node.name": strPtr("worker-1"),
			},
			wantTags: map[string]bool{},
		},
		{
			name:          "k8s.namespace",
			entityType:    "k8s.namespace",
			entityIDKey:   "k8s.namespace.uid",
			entityIDValue: "ns-789",
			attributes: map[string]any{
				"k8s.namespace.name": "production",
			},
			wantDimKey:   "k8s.namespace.uid",
			wantDimValue: "ns-789",
			wantProperties: map[string]*string{
				"default_prop":       strPtr("default_value"),
				"k8s.namespace.name": strPtr("production"),
			},
			wantTags: map[string]bool{},
		},
		{
			name:          "k8s.deployment",
			entityType:    "k8s.deployment",
			entityIDKey:   "k8s.deployment.uid",
			entityIDValue: "deploy-123",
			attributes: map[string]any{
				"k8s.deployment.name": "my-deployment",
			},
			wantDimKey:   "k8s.deployment.uid",
			wantDimValue: "deploy-123",
			wantProperties: map[string]*string{
				"default_prop":        strPtr("default_value"),
				"k8s.deployment.name": strPtr("my-deployment"),
			},
			wantTags: map[string]bool{},
		},
		{
			name:          "k8s.replicaset",
			entityType:    "k8s.replicaset",
			entityIDKey:   "k8s.replicaset.uid",
			entityIDValue: "rs-123",
			attributes: map[string]any{
				"k8s.replicaset.name": "my-rs",
			},
			wantDimKey:   "k8s.replicaset.uid",
			wantDimValue: "rs-123",
			wantProperties: map[string]*string{
				"default_prop":        strPtr("default_value"),
				"k8s.replicaset.name": strPtr("my-rs"),
			},
			wantTags: map[string]bool{},
		},
		{
			name:          "k8s.statefulset",
			entityType:    "k8s.statefulset",
			entityIDKey:   "k8s.statefulset.uid",
			entityIDValue: "ss-123",
			attributes: map[string]any{
				"k8s.statefulset.name": "my-statefulset",
			},
			wantDimKey:   "k8s.statefulset.uid",
			wantDimValue: "ss-123",
			wantProperties: map[string]*string{
				"default_prop":         strPtr("default_value"),
				"k8s.statefulset.name": strPtr("my-statefulset"),
			},
			wantTags: map[string]bool{},
		},
		{
			name:          "k8s.daemonset",
			entityType:    "k8s.daemonset",
			entityIDKey:   "k8s.daemonset.uid",
			entityIDValue: "ds-123",
			attributes: map[string]any{
				"k8s.daemonset.name": "my-daemonset",
			},
			wantDimKey:   "k8s.daemonset.uid",
			wantDimValue: "ds-123",
			wantProperties: map[string]*string{
				"default_prop":       strPtr("default_value"),
				"k8s.daemonset.name": strPtr("my-daemonset"),
			},
			wantTags: map[string]bool{},
		},
		{
			name:          "k8s.cronjob",
			entityType:    "k8s.cronjob",
			entityIDKey:   "k8s.cronjob.uid",
			entityIDValue: "cj-123",
			attributes: map[string]any{
				"k8s.cronjob.name": "my-cronjob",
			},
			wantDimKey:   "k8s.cronjob.uid",
			wantDimValue: "cj-123",
			wantProperties: map[string]*string{
				"default_prop":     strPtr("default_value"),
				"k8s.cronjob.name": strPtr("my-cronjob"),
			},
			wantTags: map[string]bool{},
		},
		{
			name:          "k8s.job",
			entityType:    "k8s.job",
			entityIDKey:   "k8s.job.uid",
			entityIDValue: "job-123",
			attributes: map[string]any{
				"k8s.job.name": "my-job",
			},
			wantDimKey:   "k8s.job.uid",
			wantDimValue: "job-123",
			wantProperties: map[string]*string{
				"default_prop": strPtr("default_value"),
				"k8s.job.name": strPtr("my-job"),
			},
			wantTags: map[string]bool{},
		},
		{
			name:          "k8s.hpa",
			entityType:    "k8s.hpa",
			entityIDKey:   "k8s.hpa.uid",
			entityIDValue: "hpa-123",
			attributes: map[string]any{
				"k8s.hpa.name": "my-hpa",
			},
			wantDimKey:   "k8s.hpa.uid",
			wantDimValue: "hpa-123",
			wantProperties: map[string]*string{
				"default_prop": strPtr("default_value"),
				"k8s.hpa.name": strPtr("my-hpa"),
			},
			wantTags: map[string]bool{},
		},
		{
			name:          "k8s.replicationcontroller",
			entityType:    "k8s.replicationcontroller",
			entityIDKey:   "k8s.replicationcontroller.uid",
			entityIDValue: "rc-123",
			attributes: map[string]any{
				"k8s.replicationcontroller.name": "my-rc",
			},
			wantDimKey:   "k8s.replicationcontroller.uid",
			wantDimValue: "rc-123",
			wantProperties: map[string]*string{
				"default_prop":                   strPtr("default_value"),
				"k8s.replicationcontroller.name": strPtr("my-rc"),
			},
			wantTags: map[string]bool{},
		},
		{
			name:          "container",
			entityType:    "container",
			entityIDKey:   "container.id",
			entityIDValue: "container-123",
			attributes: map[string]any{
				"container.name": "my-container",
			},
			wantDimKey:   "container.id",
			wantDimValue: "container-123",
			wantProperties: map[string]*string{
				"default_prop":   strPtr("default_value"),
				"container.name": strPtr("my-container"),
			},
			wantTags: map[string]bool{},
		},
		{
			name:          "k8s.service property renaming",
			entityType:    "k8s.pod",
			entityIDKey:   "k8s.pod.uid",
			entityIDValue: "pod-789",
			attributes: map[string]any{
				"k8s.service.name": "my-service",
				"k8s.service.uid":  "svc-123",
			},
			wantDimKey:   "k8s.pod.uid",
			wantDimValue: "pod-789",
			wantProperties: map[string]*string{
				"default_prop":            strPtr("default_value"),
				"kubernetes_service_name": strPtr("my-service"),
				"kubernetes_service_uid":  strPtr("svc-123"),
			},
			wantTags: map[string]bool{},
		},
		{
			name:       "unsupported entity type",
			entityType: "unknown.type",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create entity event slice and get an event from it
			entityEvents := metadata.NewEntityEventsSlice()
			entityEvent := entityEvents.AppendEmpty()

			state := entityEvent.SetEntityState()
			state.SetEntityType(tt.entityType)

			// Set entity ID
			if tt.entityIDKey != "" {
				entityEvent.ID().PutStr(tt.entityIDKey, tt.entityIDValue)
			}

			// Set attributes
			attrs := state.Attributes()
			for k, v := range tt.attributes {
				switch val := v.(type) {
				case string:
					attrs.PutStr(k, val)
				case int:
					attrs.PutInt(k, int64(val))
				}
			}

			// Transform
			dimUpdate, err := transformer.TransformEntityEvent(entityEvent)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, dimUpdate)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, dimUpdate)

			assert.Equal(t, tt.wantDimKey, dimUpdate.Name)
			assert.Equal(t, tt.wantDimValue, dimUpdate.Value)
			assert.Equal(t, tt.wantProperties, dimUpdate.Properties)
			assert.Equal(t, tt.wantTags, dimUpdate.Tags)
		})
	}
}

func TestEntityEventTransformer_DeleteEvent(t *testing.T) {
	transformer := NewEntityEventTransformer(nil)

	// Create delete event
	entityEvents := metadata.NewEntityEventsSlice()
	entityEvent := entityEvents.AppendEmpty()
	deleteEvent := entityEvent.SetEntityDelete()
	deleteEvent.SetEntityType("k8s.pod")

	dimUpdate, err := transformer.TransformEntityEvent(entityEvent)
	require.NoError(t, err)
	assert.Nil(t, dimUpdate, "delete events should return nil")
}

func TestEntityEventTransformer_MissingEntityID(t *testing.T) {
	transformer := NewEntityEventTransformer(nil)

	// Create entity event without ID
	entityEvents := metadata.NewEntityEventsSlice()
	entityEvent := entityEvents.AppendEmpty()
	state := entityEvent.SetEntityState()
	state.SetEntityType("k8s.pod")

	dimUpdate, err := transformer.TransformEntityEvent(entityEvent)
	assert.Error(t, err)
	assert.Nil(t, dimUpdate)
	assert.Contains(t, err.Error(), "entity ID not found")
}

func strPtr(s string) *string {
	return &s
}
