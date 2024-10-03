// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pentity"

	metadataPkg "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
)

func Test_GetEntityEvents(t *testing.T) {
	tests := []struct {
		name     string
		old, new map[metadataPkg.ResourceID]*KubernetesMetadata
		events   pentity.EntityEventSlice
	}{
		{
			name: "new entity",
			new: map[metadataPkg.ResourceID]*KubernetesMetadata{
				"123": {
					EntityType:    "k8s.pod",
					ResourceIDKey: "k8s.pod.uid",
					ResourceID:    "123",
					Metadata: map[string]string{
						"label1": "value1",
					},
				},
			},
			events: func() pentity.EntityEventSlice {
				out := pentity.NewEntityEventSlice()
				event := out.AppendEmpty()
				event.SetEntityType("k8s.pod")
				_ = event.Id().FromRaw(map[string]any{"k8s.pod.uid": "123"})
				state := event.SetEmptyEntityState()
				_ = state.Attributes().FromRaw(map[string]any{"label1": "value1"})
				return out
			}(),
		},
		{
			name: "deleted entity",
			old: map[metadataPkg.ResourceID]*KubernetesMetadata{
				"123": {
					EntityType:    "k8s.pod",
					ResourceIDKey: "k8s.pod.uid",
					ResourceID:    "123",
					Metadata: map[string]string{
						"label1": "value1",
					},
				},
			},
			events: func() pentity.EntityEventSlice {
				out := pentity.NewEntityEventSlice()
				event := out.AppendEmpty()
				_ = event.Id().FromRaw(map[string]any{"k8s.pod.uid": "123"})
				event.SetEmptyEntityDelete()
				return out
			}(),
		},
		{
			name: "changed entity",
			old: map[metadataPkg.ResourceID]*KubernetesMetadata{
				"123": {
					EntityType:    "k8s.pod",
					ResourceIDKey: "k8s.pod.uid",
					ResourceID:    "123",
					Metadata: map[string]string{
						"label1": "value1",
						"label2": "value2",
						"label3": "value3",
					},
				},
			},
			new: map[metadataPkg.ResourceID]*KubernetesMetadata{
				"123": {
					EntityType:    "k8s.pod",
					ResourceIDKey: "k8s.pod.uid",
					ResourceID:    "123",
					Metadata: map[string]string{
						"label1": "value1",
						"label2": "foo",
						"new":    "bar",
					},
				},
			},
			events: func() pentity.EntityEventSlice {
				out := pentity.NewEntityEventSlice()
				event := out.AppendEmpty()
				event.SetEntityType("k8s.pod")
				_ = event.Id().FromRaw(map[string]any{"k8s.pod.uid": "123"})
				state := event.SetEmptyEntityState()
				_ = state.Attributes().FromRaw(map[string]any{"label1": "value1", "label2": "foo", "new": "bar"})
				return out
			}(),
		},
		{
			name: "unchanged entity",
			old: map[metadataPkg.ResourceID]*KubernetesMetadata{
				"123": {
					EntityType:    "k8s.pod",
					ResourceIDKey: "k8s.pod.uid",
					ResourceID:    "123",
					Metadata: map[string]string{
						"label1": "value1",
						"label2": "value2",
						"label3": "value3",
					},
				},
			},
			new: map[metadataPkg.ResourceID]*KubernetesMetadata{
				"123": {
					EntityType:    "k8s.pod",
					ResourceIDKey: "k8s.pod.uid",
					ResourceID:    "123",
					Metadata: map[string]string{
						"label1": "value1",
						"label2": "value2",
						"label3": "value3",
					},
				},
			},
			events: func() pentity.EntityEventSlice {
				out := pentity.NewEntityEventSlice()
				event := out.AppendEmpty()
				event.SetEntityType("k8s.pod")
				_ = event.Id().FromRaw(map[string]any{"k8s.pod.uid": "123"})
				state := event.SetEmptyEntityState()
				_ = state.Attributes().FromRaw(
					map[string]any{
						"label1": "value1", "label2": "value2", "label3": "value3",
					},
				)
				return out
			}(),
		},
		{
			name: "new and deleted entity",
			old: map[metadataPkg.ResourceID]*KubernetesMetadata{
				"123": {
					EntityType:    "k8s.pod",
					ResourceIDKey: "k8s.pod.uid",
					ResourceID:    "123",
					Metadata: map[string]string{
						"label1": "value1",
					},
				},
			},
			new: map[metadataPkg.ResourceID]*KubernetesMetadata{
				"234": {
					EntityType:    "k8s.pod",
					ResourceIDKey: "k8s.pod.uid",
					ResourceID:    "234",
					Metadata: map[string]string{
						"label2": "value2",
					},
				},
			},
			events: func() pentity.EntityEventSlice {
				out := pentity.NewEntityEventSlice()

				event := out.AppendEmpty()
				_ = event.Id().FromRaw(map[string]any{"k8s.pod.uid": "123"})
				event.SetEmptyEntityDelete()

				event = out.AppendEmpty()
				event.SetEntityType("k8s.pod")
				_ = event.Id().FromRaw(map[string]any{"k8s.pod.uid": "234"})
				state := event.SetEmptyEntityState()
				_ = state.Attributes().FromRaw(map[string]any{"label2": "value2"})
				return out
			}(),
		},
	}
	for _, test := range tests {
		tt := test
		t.Run(
			tt.name, func(t *testing.T) {
				// Make sure test data is correct.
				for k, v := range tt.old {
					assert.EqualValues(t, k, v.ResourceID)
				}
				for k, v := range tt.new {
					assert.EqualValues(t, k, v.ResourceID)
				}

				// Convert and test expected events.
				timestamp := pcommon.NewTimestampFromTime(time.Now())
				events := GetEntityEvents(tt.old, tt.new, timestamp)
				require.Equal(t, tt.events.Len(), events.Len())
				for i := 0; i < events.Len(); i++ {
					actual := events.At(i)
					expected := tt.events.At(i)
					assert.EqualValues(t, timestamp, actual.Timestamp())
					assert.EqualValues(t, expected.Type(), actual.Type())
					assert.EqualValues(t, expected.Id().AsRaw(), actual.Id().AsRaw())
					if expected.Type() == pentity.EventTypeEntityState {
						estate := expected.EntityState()
						astate := actual.EntityState()
						assert.EqualValues(t, estate.Attributes().AsRaw(), astate.Attributes().AsRaw())
					}
				}
			},
		)
	}
}
