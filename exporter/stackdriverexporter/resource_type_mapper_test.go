package stackdriverexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opencensus.io/resource/resourcekeys"
)

func TestInferResourceType(t *testing.T) {
	tests := []struct {
		name             string
		labels           map[string]string
		wantResourceType string
		wantOk           bool
	}{
		{
			name:   "empty labels",
			labels: nil,
			wantOk: false,
		},
		{
			name: "container",
			labels: map[string]string{
				resourcekeys.K8SKeyClusterName:   "cluster1",
				resourcekeys.K8SKeyPodName:       "pod1",
				resourcekeys.K8SKeyNamespaceName: "namespace1",
				resourcekeys.ContainerKeyName:    "container-name1",
				resourcekeys.CloudKeyAccountID:   "proj1",
				resourcekeys.CloudKeyZone:        "zone1",
			},
			wantResourceType: resourcekeys.ContainerType,
			wantOk:           true,
		},
		{
			name: "pod",
			labels: map[string]string{
				resourcekeys.K8SKeyClusterName:   "cluster1",
				resourcekeys.K8SKeyPodName:       "pod1",
				resourcekeys.K8SKeyNamespaceName: "namespace1",
				resourcekeys.CloudKeyZone:        "zone1",
			},
			wantResourceType: resourcekeys.K8SType,
			wantOk:           true,
		},
		{
			name: "host",
			labels: map[string]string{
				resourcekeys.K8SKeyClusterName: "cluster1",
				resourcekeys.CloudKeyZone:      "zone1",
				resourcekeys.HostKeyName:       "node1",
			},
			wantResourceType: resourcekeys.HostType,
			wantOk:           true,
		},
		{
			name: "gce",
			labels: map[string]string{
				resourcekeys.CloudKeyProvider: resourcekeys.CloudProviderGCP,
				resourcekeys.HostKeyID:        "inst1",
				resourcekeys.CloudKeyZone:     "zone1",
			},
			wantResourceType: resourcekeys.CloudType,
			wantOk:           true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resourceType, ok := inferResourceType(tc.labels)
			if tc.wantOk {
				assert.True(t, ok)
				assert.Equal(t, tc.wantResourceType, resourceType)
			} else {
				assert.False(t, ok)
				assert.Equal(t, "", resourceType)
			}
		})
	}
}
