// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opencensus

import (
	"strings"
	"testing"

	occommon "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	agenttracepb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/trace/v1"
	ocresource "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/occonventions"
)

func TestOcNodeResourceToInternal(t *testing.T) {
	resource := pcommon.NewResource()
	ocNodeResourceToInternal(nil, nil, resource)
	assert.Equal(t, 0, resource.Attributes().Len())

	ocNode := &occommon.Node{}
	ocResource := &ocresource.Resource{}
	ocNodeResourceToInternal(ocNode, ocResource, resource)
	assert.Equal(t, 0, resource.Attributes().Len())

	ocNode = generateOcNode()
	ocResource = generateOcResource()
	expectedAttrs := generateResourceWithOcNodeAndResource().Attributes()
	// We don't have type information in ocResource, so need to make int attr string
	expectedAttrs.PutStr("resource-int-attr", "123")
	ocNodeResourceToInternal(ocNode, ocResource, resource)
	assert.Equal(t, expectedAttrs.AsRaw(), resource.Attributes().AsRaw())

	// Make sure hard-coded fields override same-name values in Attributes.
	// To do that add Attributes with same-name.
	expectedAttrs.Range(func(k string, v pcommon.Value) bool {
		// Set all except "attr1" which is not a hard-coded field to some bogus values.
		if !strings.Contains(k, "-attr") {
			ocNode.Attributes[k] = "this will be overridden 1"
		}
		return true
	})
	ocResource.Labels[occonventions.AttributeResourceType] = "this will be overridden 2"

	// Convert again.
	resource = pcommon.NewResource()
	ocNodeResourceToInternal(ocNode, ocResource, resource)

	// And verify that same-name attributes were ignored.
	assert.Equal(t, expectedAttrs.AsRaw(), resource.Attributes().AsRaw())
}

func BenchmarkOcNodeResourceToInternal(b *testing.B) {
	ocNode := generateOcNode()
	ocResource := generateOcResource()

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		resource := pcommon.NewResource()
		ocNodeResourceToInternal(ocNode, ocResource, resource)
		if ocNode.Identifier.Pid != 123 {
			b.Fail()
		}
	}
}

func BenchmarkOcResourceNodeUnmarshal(b *testing.B) {
	oc := &agenttracepb.ExportTraceServiceRequest{
		Node:     generateOcNode(),
		Spans:    nil,
		Resource: generateOcResource(),
	}

	bytes, err := proto.Marshal(oc)
	if err != nil {
		b.Fail()
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		unmarshalOc := &agenttracepb.ExportTraceServiceRequest{}
		if err := proto.Unmarshal(bytes, unmarshalOc); err != nil {
			b.Fail()
		}
		if unmarshalOc.Node.Identifier.Pid != 123 {
			b.Fail()
		}
	}
}
