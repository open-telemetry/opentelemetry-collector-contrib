// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sinventory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestMode(t *testing.T) {
	tests := []struct {
		name string
		mode Mode
		want string
	}{
		{
			name: "PullMode",
			mode: PullMode,
			want: "pull",
		},
		{
			name: "WatchMode",
			mode: WatchMode,
			want: "watch",
		},
		{
			name: "DefaultMode",
			mode: DefaultMode,
			want: "pull",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, string(tt.mode))
		})
	}
}

func TestConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "Config with all fields",
			config: Config{
				Gvr: schema.GroupVersionResource{
					Group:    "apps",
					Version:  "v1",
					Resource: "deployments",
				},
				Namespaces:      []string{"default", "kube-system"},
				LabelSelector:   "app=test",
				FieldSelector:   "status.phase=Running",
				ResourceVersion: "12345",
			},
		},
		{
			name: "Config with minimal fields",
			config: Config{
				Gvr: schema.GroupVersionResource{
					Group:    "",
					Version:  "v1",
					Resource: "pods",
				},
			},
		},
		{
			name: "Config with empty namespaces",
			config: Config{
				Gvr: schema.GroupVersionResource{
					Group:    "",
					Version:  "v1",
					Resource: "services",
				},
				Namespaces: []string{},
			},
		},
		{
			name: "Config with selectors",
			config: Config{
				Gvr: schema.GroupVersionResource{
					Group:    "",
					Version:  "v1",
					Resource: "events",
				},
				LabelSelector: "component=apiserver",
				FieldSelector: "involvedObject.kind=Pod",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the config struct is properly constructed
			assert.NotNil(t, tt.config.Gvr)

			if len(tt.config.Namespaces) > 0 {
				assert.NotEmpty(t, tt.config.Namespaces)
			}

			// Verify GVR fields
			assert.NotEmpty(t, tt.config.Gvr.Resource)
			assert.NotEmpty(t, tt.config.Gvr.Version)
		})
	}
}

func TestConfigGvr(t *testing.T) {
	gvr := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}

	config := Config{
		Gvr: gvr,
	}

	assert.Equal(t, "apps", config.Gvr.Group)
	assert.Equal(t, "v1", config.Gvr.Version)
	assert.Equal(t, "deployments", config.Gvr.Resource)
	assert.Equal(t, "apps/v1, Resource=deployments", config.Gvr.String())
}

func TestConfigNamespaces(t *testing.T) {
	tests := []struct {
		name       string
		namespaces []string
		wantLen    int
	}{
		{
			name:       "No namespaces",
			namespaces: nil,
			wantLen:    0,
		},
		{
			name:       "Empty slice",
			namespaces: []string{},
			wantLen:    0,
		},
		{
			name:       "Single namespace",
			namespaces: []string{"default"},
			wantLen:    1,
		},
		{
			name:       "Multiple namespaces",
			namespaces: []string{"default", "kube-system", "monitoring"},
			wantLen:    3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Gvr: schema.GroupVersionResource{
					Group:    "",
					Version:  "v1",
					Resource: "pods",
				},
				Namespaces: tt.namespaces,
			}

			assert.Len(t, config.Namespaces, tt.wantLen)
			if tt.wantLen > 0 {
				assert.Equal(t, tt.namespaces, config.Namespaces)
			}
		})
	}
}

func TestConfigSelectors(t *testing.T) {
	tests := []struct {
		name          string
		labelSelector string
		fieldSelector string
	}{
		{
			name:          "Both selectors set",
			labelSelector: "app=test,version=v1",
			fieldSelector: "status.phase=Running",
		},
		{
			name:          "Only label selector",
			labelSelector: "environment=production",
			fieldSelector: "",
		},
		{
			name:          "Only field selector",
			labelSelector: "",
			fieldSelector: "spec.nodeName=node-1",
		},
		{
			name:          "No selectors",
			labelSelector: "",
			fieldSelector: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Gvr: schema.GroupVersionResource{
					Group:    "",
					Version:  "v1",
					Resource: "pods",
				},
				LabelSelector: tt.labelSelector,
				FieldSelector: tt.fieldSelector,
			}

			assert.Equal(t, tt.labelSelector, config.LabelSelector)
			assert.Equal(t, tt.fieldSelector, config.FieldSelector)
		})
	}
}
