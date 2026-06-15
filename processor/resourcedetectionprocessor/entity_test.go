// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcedetectionprocessor

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	xentity "go.opentelemetry.io/collector/pdata/xpdata/entity"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

type entityDetectorStub struct {
	attrs    map[string]string
	entities []internal.DetectedEntity
}

func (d *entityDetectorStub) Detect(context.Context) (pcommon.Resource, string, error) {
	res := pcommon.NewResource()
	for k, v := range d.attrs {
		res.Attributes().PutStr(k, v)
	}
	return res, "", nil
}

func (d *entityDetectorStub) EntityRefs(pcommon.Resource) []internal.DetectedEntity {
	return d.entities
}

func TestProcessorAttachesEntityRefs(t *testing.T) {
	k8sDetector := &entityDetectorStub{
		attrs: map[string]string{"k8s.node.name": "node-1"},
		entities: []internal.DetectedEntity{{
			Type:                    "k8s.node",
			IDKeys:                  []string{"k8s.node.name"},
			IDContextTypeCandidates: []string{"k8s.cluster"},
		}},
	}
	cloudDetector := &entityDetectorStub{
		attrs: map[string]string{"k8s.cluster.name": "cluster-1"},
		entities: []internal.DetectedEntity{{
			Type:   "k8s.cluster",
			IDKeys: []string{"k8s.cluster.name"},
		}},
	}

	provider := internal.NewResourceProvider(zap.NewNop(), time.Second, k8sDetector, cloudDetector)
	require.NoError(t, provider.Refresh(context.Background(), &http.Client{Timeout: time.Second}))

	rdp := &resourceDetectionProcessor{provider: provider}
	td := ptrace.NewTraces()
	td.ResourceSpans().AppendEmpty().Resource().Attributes().PutStr("service.name", "svc")

	out, err := rdp.processTraces(context.Background(), td)
	require.NoError(t, err)

	res := out.ResourceSpans().At(0).Resource()
	assert.Equal(t, "node-1", mustStr(t, res.Attributes(), "k8s.node.name"))

	entities := xentity.ResourceEntities(res)
	require.Equal(t, 2, entities.Len())
	got := map[string]string{}
	for typ, entity := range entities.All() {
		got[typ] = entity.IDContextType()
	}
	assert.Equal(t, map[string]string{
		"k8s.node":    "k8s.cluster",
		"k8s.cluster": "",
	}, got)
}

func mustStr(t *testing.T, attrs pcommon.Map, key string) string {
	v, ok := attrs.Get(key)
	require.True(t, ok, "missing attribute %q", key)
	return v.Str()
}
