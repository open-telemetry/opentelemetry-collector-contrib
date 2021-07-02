// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collection

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	metadata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
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

	rm := getGenericMetadata(om, "ResourceType")

	assert.Equal(t, "k8s.resourcetype.uid", rm.resourceIDKey)
	assert.Equal(t, metadata.ResourceID("test-uid"), rm.resourceID)
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
	}, rm.metadata)
}

func metadataMap(mdata map[string]string) map[metadata.ResourceID]*KubernetesMetadata {
	rid := metadata.ResourceID("resource_id")
	return map[metadata.ResourceID]*KubernetesMetadata{
		rid: {
			resourceIDKey: "resource_id",
			resourceID:    rid,
			metadata:      mdata,
		},
	}
}

func TestGetMetadataUpdate(t *testing.T) {
	type args struct {
		oldMdata map[metadata.ResourceID]*KubernetesMetadata
		newMdata map[metadata.ResourceID]*KubernetesMetadata
	}
	tests := []struct {
		name          string
		args          args
		metadataDelta *metadata.MetadataDelta
	}{
		{
			"Add to new",
			args{
				oldMdata: metadataMap(map[string]string{}),
				newMdata: metadataMap(map[string]string{
					"foo": "bar",
				}),
			},
			&metadata.MetadataDelta{
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
			&metadata.MetadataDelta{
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
			&metadata.MetadataDelta{
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
			&metadata.MetadataDelta{
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
			&metadata.MetadataDelta{
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
				oldMdata: map[metadata.ResourceID]*KubernetesMetadata{},
				newMdata: metadataMap(map[string]string{
					"foo": "bar",
				}),
			},
			&metadata.MetadataDelta{
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
