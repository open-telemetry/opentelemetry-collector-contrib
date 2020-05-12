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

	rm := getGenericMetadata(om, "resourcetype")

	assert.Equal(t, "k8s.resourcetype.uid", rm.resourceIDKey)
	assert.Equal(t, ResourceID("test-uid"), rm.resourceID)
	assert.Equal(t, map[string]string{
		"k8s.workload.name":               "test-name",
		"k8s.workload.kind":               "resourcetype",
		"resourcetype.creation_timestamp": now.Format(time.RFC3339),
		"owner-kind-1":                    "owner1",
		"owner-kind-1_uid":                "owner1",
		"owner-kind-2":                    "owner2",
		"owner-kind-2_uid":                "owner2",
		"foo":                             "bar",
		"foo1":                            "",
	}, rm.metadata)
}

func TestGetPropertiesDelta(t *testing.T) {
	type args struct {
		oldProps map[string]string
		newProps map[string]string
	}
	tests := []struct {
		name          string
		args          args
		metadataDelta *MetadataDelta
	}{
		{
			"Add to new",
			args{
				oldProps: map[string]string{},
				newProps: map[string]string{
					"foo": "bar",
				},
			},
			&MetadataDelta{
				map[string]string{
					"foo": "bar",
				},
				map[string]string{},
				map[string]string{},
			},
		},
		{
			"Add to existing",
			args{
				oldProps: map[string]string{
					"oldfoo": "bar",
				},
				newProps: map[string]string{
					"oldfoo": "bar",
					"foo":    "bar",
				},
			},
			&MetadataDelta{
				map[string]string{
					"foo": "bar",
				},
				map[string]string{},
				map[string]string{},
			},
		},
		{
			"Modify existing",
			args{
				oldProps: map[string]string{
					"foo": "bar",
				},
				newProps: map[string]string{
					"foo": "newbar",
				},
			},
			&MetadataDelta{
				map[string]string{},
				map[string]string{},
				map[string]string{
					"foo": "newbar",
				},
			},
		},
		{
			"Remove existing",
			args{
				oldProps: map[string]string{
					"foo":  "bar",
					"foo1": "bar1",
				},
				newProps: map[string]string{
					"foo1": "bar1",
				},
			},
			&MetadataDelta{
				map[string]string{},
				map[string]string{
					"foo": "bar",
				},
				map[string]string{},
			},
		},
		{
			"Properties with empty values",
			args{
				oldProps: map[string]string{
					"foo":         "bar",
					"foo2":        "bar2",
					"service_abc": "",
					"admin":       "",
					"test":        "",
				},
				newProps: map[string]string{
					"foo":         "bar2",
					"foo1":        "bar1",
					"service_def": "",
					"test":        "",
				},
			},
			&MetadataDelta{
				map[string]string{
					"service_def": "",
					"foo1":        "bar1",
				},
				map[string]string{
					"foo2":        "bar2",
					"service_abc": "",
					"admin":       "",
				},
				map[string]string{
					"foo": "bar2",
				},
			},
		},
		{
			"No update",
			args{
				oldProps: map[string]string{
					"foo":  "bar",
					"foo1": "bar1",
				},
				newProps: map[string]string{
					"foo":  "bar",
					"foo1": "bar1",
				},
			},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delta := getMetadataDelta(tt.args.oldProps, tt.args.newProps)
			require.Equal(t, tt.metadataDelta, delta)
		})
	}
}
