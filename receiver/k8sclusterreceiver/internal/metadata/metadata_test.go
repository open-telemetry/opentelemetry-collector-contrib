// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
)

func Test_getGenericMetadata(t *testing.T) {
	now := time.Now()
	om := &v1.ObjectMeta{
		Name:              "test-name",
		UID:               "test-uid",
		Generation:        0,
		CreationTimestamp: v1.NewTime(now),
		Labels: map[string]string{
			"foo":  "bar",
			"foo1": "",
		},
		OwnerReferences: []v1.OwnerReference{
			{
				Kind: "Owner-kind-1",
				UID:  "owner1",
				Name: "owner1",
			},
			{
				Kind: "owner-kind-2",
				UID:  "owner2",
				Name: "owner2",
			},
		},
	}

	rm := GetGenericMetadata(om, "ResourceType")

	assert.Equal(t, "k8s.resourcetype.uid", rm.ResourceIDKey)
	assert.Equal(t, experimentalmetricmetadata.ResourceID("test-uid"), rm.ResourceID)
	assert.Equal(t, map[string]string{
		"k8s.workload.name":               "test-name",
		"k8s.workload.kind":               "ResourceType",
		"resourcetype.creation_timestamp": now.Format(time.RFC3339),
		"k8s.owner-kind-1.name":           "owner1",
		"k8s.owner-kind-1.uid":            "owner1",
		"k8s.owner-kind-2.name":           "owner2",
		"k8s.owner-kind-2.uid":            "owner2",
		"foo":                             "bar",
		"foo1":                            "",
	}, rm.Metadata)
}

func metadataMap(mdata map[string]string) map[experimentalmetricmetadata.ResourceID]*KubernetesMetadata {
	rid := experimentalmetricmetadata.ResourceID("resource_id")
	return map[experimentalmetricmetadata.ResourceID]*KubernetesMetadata{
		rid: {
			ResourceIDKey: "resource_id",
			ResourceID:    rid,
			Metadata:      mdata,
		},
	}
}

func TestGetMetadataUpdate(t *testing.T) {
	type args struct {
		oldMdata map[experimentalmetricmetadata.ResourceID]*KubernetesMetadata
		newMdata map[experimentalmetricmetadata.ResourceID]*KubernetesMetadata
	}
	tests := []struct {
		name          string
		args          args
		metadataDelta *experimentalmetricmetadata.MetadataDelta
	}{
		{
			"Add to new",
			args{
				oldMdata: metadataMap(map[string]string{}),
				newMdata: metadataMap(map[string]string{
					"foo": "bar",
				}),
			},
			&experimentalmetricmetadata.MetadataDelta{
				MetadataToAdd: map[string]string{
					"foo": "bar",
				},
				MetadataToRemove: map[string]string{},
				MetadataToUpdate: map[string]string{},
			},
		},
		{
			"Add to existing",
			args{
				oldMdata: metadataMap(map[string]string{
					"oldfoo": "bar",
				}),
				newMdata: metadataMap(map[string]string{
					"oldfoo": "bar",
					"foo":    "bar",
				}),
			},
			&experimentalmetricmetadata.MetadataDelta{
				MetadataToAdd: map[string]string{
					"foo": "bar",
				},
				MetadataToRemove: map[string]string{},
				MetadataToUpdate: map[string]string{},
			},
		},
		{
			"Modify existing",
			args{
				oldMdata: metadataMap(map[string]string{
					"foo": "bar",
				}),
				newMdata: metadataMap(map[string]string{
					"foo": "newbar",
				}),
			},
			&experimentalmetricmetadata.MetadataDelta{
				MetadataToAdd:    map[string]string{},
				MetadataToRemove: map[string]string{},
				MetadataToUpdate: map[string]string{
					"foo": "newbar",
				},
			},
		},
		{
			"Remove existing",
			args{
				oldMdata: metadataMap(map[string]string{
					"foo":  "bar",
					"foo1": "bar1",
				}),
				newMdata: metadataMap(map[string]string{
					"foo1": "bar1",
				}),
			},
			&experimentalmetricmetadata.MetadataDelta{
				MetadataToAdd: map[string]string{},
				MetadataToRemove: map[string]string{
					"foo": "bar",
				},
				MetadataToUpdate: map[string]string{},
			},
		},
		{
			"Properties with empty values",
			args{
				oldMdata: metadataMap(map[string]string{
					"foo":         "bar",
					"foo2":        "bar2",
					"service_abc": "",
					"admin":       "",
					"test":        "",
				}),
				newMdata: metadataMap(map[string]string{
					"foo":         "bar2",
					"foo1":        "bar1",
					"service_def": "",
					"test":        "",
				}),
			},
			&experimentalmetricmetadata.MetadataDelta{
				MetadataToAdd: map[string]string{
					"service_def": "",
					"foo1":        "bar1",
				},
				MetadataToRemove: map[string]string{
					"foo2":        "bar2",
					"service_abc": "",
					"admin":       "",
				},
				MetadataToUpdate: map[string]string{
					"foo": "bar2",
				},
			},
		},
		{
			"No update",
			args{
				oldMdata: metadataMap(map[string]string{
					"foo":  "bar",
					"foo1": "bar1",
				}),
				newMdata: metadataMap(map[string]string{
					"foo":  "bar",
					"foo1": "bar1",
				}),
			},
			nil,
		},
		{
			"New metadata",
			args{
				oldMdata: map[experimentalmetricmetadata.ResourceID]*KubernetesMetadata{},
				newMdata: metadataMap(map[string]string{
					"foo": "bar",
				}),
			},
			&experimentalmetricmetadata.MetadataDelta{
				MetadataToAdd: map[string]string{
					"foo": "bar",
				},
				MetadataToRemove: nil,
				MetadataToUpdate: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delta := GetMetadataUpdate(tt.args.oldMdata, tt.args.newMdata)
			if tt.metadataDelta != nil {
				require.Equal(t, 1, len(delta))
				require.Equal(t, *tt.metadataDelta, delta[0].MetadataDelta)
			} else {
				require.Zero(t, len(delta))
			}
		})
	}
}

func TestTransformObjectMeta(t *testing.T) {
	in := v1.ObjectMeta{
		Name:      "my-pod",
		UID:       "12345678-1234-1234-1234-123456789011",
		Namespace: "default",
		Labels: map[string]string{
			"app": "my-app",
		},
		Annotations: map[string]string{
			"version":     "1.0",
			"description": "Sample resource",
		},
		OwnerReferences: []v1.OwnerReference{
			{
				APIVersion: "apps/v1",
				Kind:       "ReplicaSet",
				Name:       "my-replicaset-1",
				UID:        "12345678-1234-1234-1234-123456789012",
			},
		},
	}
	want := v1.ObjectMeta{
		Name:      "my-pod",
		UID:       "12345678-1234-1234-1234-123456789011",
		Namespace: "default",
		Labels: map[string]string{
			"app": "my-app",
		},
		OwnerReferences: []v1.OwnerReference{
			{
				Kind: "ReplicaSet",
				Name: "my-replicaset-1",
				UID:  "12345678-1234-1234-1234-123456789012",
			},
		},
	}
	assert.Equal(t, want, TransformObjectMeta(in))
}
