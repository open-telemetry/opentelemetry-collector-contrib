// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opencensus

import (
	"strconv"
	"testing"

	occommon "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	agenttracepb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/trace/v1"
	ocresource "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"go.opencensus.io/resource/resourcekeys"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/goldendataset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/occonventions"
)

func TestResourceToOC(t *testing.T) {
	emptyResource := pcommon.NewResource()

	ocNode := generateOcNode()
	ocResource := generateOcResource()
	// We don't differentiate between Node.Attributes and Resource when converting,
	// and put everything in Resource.
	ocResource.Labels["node-str-attr"] = "node-str-attr-val"
	ocNode.Attributes = nil

	tests := []struct {
		name       string
		resource   pcommon.Resource
		ocNode     *occommon.Node
		ocResource *ocresource.Resource
	}{
		{
			name:       "nil",
			resource:   pcommon.NewResource(),
			ocNode:     nil,
			ocResource: nil,
		},

		{
			name:       "empty",
			resource:   emptyResource,
			ocNode:     nil,
			ocResource: nil,
		},

		{
			name:       "with-attributes",
			resource:   generateResourceWithOcNodeAndResource(),
			ocNode:     ocNode,
			ocResource: ocResource,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ocNode, ocResource := internalResourceToOC(test.resource)
			assert.EqualValues(t, test.ocNode, ocNode)
			assert.EqualValues(t, test.ocResource, ocResource)
		})
	}
}

func TestContainerResourceToOC(t *testing.T) {
	resource := pcommon.NewResource()
	resource.Attributes().PutStr(conventions.AttributeK8SClusterName, "cluster1")
	resource.Attributes().PutStr(conventions.AttributeK8SPodName, "pod1")
	resource.Attributes().PutStr(conventions.AttributeK8SNamespaceName, "namespace1")
	resource.Attributes().PutStr(conventions.AttributeContainerName, "container-name1")
	resource.Attributes().PutStr(conventions.AttributeCloudAccountID, "proj1")
	resource.Attributes().PutStr(conventions.AttributeCloudAvailabilityZone, "zone1")

	want := &ocresource.Resource{
		Type: resourcekeys.ContainerType, // Inferred type
		Labels: map[string]string{
			resourcekeys.K8SKeyClusterName:   "cluster1",
			resourcekeys.K8SKeyPodName:       "pod1",
			resourcekeys.K8SKeyNamespaceName: "namespace1",
			resourcekeys.ContainerKeyName:    "container-name1",
			resourcekeys.CloudKeyAccountID:   "proj1",
			resourcekeys.CloudKeyZone:        "zone1",
		},
	}

	_, ocResource := internalResourceToOC(resource)
	if diff := cmp.Diff(want, ocResource, protocmp.Transform()); diff != "" {
		t.Errorf("Unexpected difference:\n%v", diff)
	}

	// Also test that the explicit resource type is preserved if present
	resource.Attributes().PutStr(occonventions.AttributeResourceType, "other-type")
	want.Type = "other-type"

	_, ocResource = internalResourceToOC(resource)
	if diff := cmp.Diff(want, ocResource, protocmp.Transform()); diff != "" {
		t.Errorf("Unexpected difference:\n%v", diff)
	}
}

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
				conventions.AttributeK8SClusterName:        "cluster1",
				conventions.AttributeK8SPodName:            "pod1",
				conventions.AttributeK8SNamespaceName:      "namespace1",
				conventions.AttributeContainerName:         "container-name1",
				conventions.AttributeCloudAccountID:        "proj1",
				conventions.AttributeCloudAvailabilityZone: "zone1",
			},
			wantResourceType: resourcekeys.ContainerType,
			wantOk:           true,
		},
		{
			name: "pod",
			labels: map[string]string{
				conventions.AttributeK8SClusterName:        "cluster1",
				conventions.AttributeK8SPodName:            "pod1",
				conventions.AttributeK8SNamespaceName:      "namespace1",
				conventions.AttributeCloudAvailabilityZone: "zone1",
			},
			wantResourceType: resourcekeys.K8SType,
			wantOk:           true,
		},
		{
			name: "host",
			labels: map[string]string{
				conventions.AttributeK8SClusterName:        "cluster1",
				conventions.AttributeCloudAvailabilityZone: "zone1",
				conventions.AttributeHostName:              "node1",
			},
			wantResourceType: resourcekeys.HostType,
			wantOk:           true,
		},
		{
			name: "gce",
			labels: map[string]string{
				conventions.AttributeCloudProvider:         "gcp",
				conventions.AttributeHostID:                "inst1",
				conventions.AttributeCloudAvailabilityZone: "zone1",
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

func TestResourceToOCAndBack(t *testing.T) {
	tests := []goldendataset.PICTInputResource{
		goldendataset.ResourceEmpty,
		goldendataset.ResourceVMOnPrem,
		goldendataset.ResourceVMCloud,
		goldendataset.ResourceK8sOnPrem,
		goldendataset.ResourceK8sCloud,
		goldendataset.ResourceFaas,
		goldendataset.ResourceExec,
	}
	for _, test := range tests {
		t.Run(string(test), func(t *testing.T) {
			traces := ptrace.NewTraces()
			goldendataset.GenerateResource(test).CopyTo(traces.ResourceSpans().AppendEmpty().Resource())
			expected := traces.ResourceSpans().At(0).Resource()
			ocNode, ocResource := internalResourceToOC(expected)
			actual := pcommon.NewResource()
			ocNodeResourceToInternal(ocNode, ocResource, actual)
			// Remove opencensus resource type from actual. This will be added during translation.
			actual.Attributes().Remove(occonventions.AttributeResourceType)
			assert.Equal(t, expected.Attributes().Len(), actual.Attributes().Len())
			expected.Attributes().Range(func(k string, v pcommon.Value) bool {
				a, ok := actual.Attributes().Get(k)
				assert.True(t, ok)
				switch v.Type() {
				case pcommon.ValueTypeInt:
					// conventions.AttributeProcessID is special because we preserve the type for this.
					if k == conventions.AttributeProcessPID {
						assert.Equal(t, v.Int(), a.Int())
					} else {
						assert.Equal(t, strconv.FormatInt(v.Int(), 10), a.Str())
					}
				case pcommon.ValueTypeMap, pcommon.ValueTypeSlice:
					assert.Equal(t, a, a)
				default:
					assert.Equal(t, v, a)
				}
				return true
			})
		})
	}
}

func BenchmarkInternalResourceToOC(b *testing.B) {
	resource := generateResourceWithOcNodeAndResource()

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		ocNode, _ := internalResourceToOC(resource)
		if ocNode.Identifier.Pid != 123 {
			b.Fail()
		}
	}
}

func BenchmarkOcResourceNodeMarshal(b *testing.B) {
	oc := &agenttracepb.ExportTraceServiceRequest{
		Node:     generateOcNode(),
		Spans:    nil,
		Resource: generateOcResource(),
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if _, err := proto.Marshal(oc); err != nil {
			b.Fail()
		}
	}
}
