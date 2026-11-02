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
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	xentity "go.opentelemetry.io/collector/pdata/xpdata/entity"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/metadata"
)

type entityDetectorStub struct {
	attrs    map[string]any
	entities []internal.DetectedEntity
}

func (d *entityDetectorStub) Detect(context.Context) (pcommon.Resource, string, error) {
	res := pcommon.NewResource()
	if err := res.Attributes().FromRaw(d.attrs); err != nil {
		return res, "", err
	}
	return res, "", nil
}

func (d *entityDetectorStub) Entities(pcommon.Resource) []internal.DetectedEntity {
	return d.entities
}

type stubConfig struct{}

func (stubConfig) GetConfigFromType(internal.DetectorType) internal.DetectorConfig { return nil }

func TestProcessorAttachesScopedEntities(t *testing.T) {
	k8s := &entityDetectorStub{
		attrs: map[string]any{"k8s.node.name": "node-1"},
		entities: []internal.DetectedEntity{{Type: "k8s.node", IDKeys: []string{"k8s.node.name"},
			IDScope: xentity.IdScopeLocal, IDScopeEntityType: "k8s.cluster"}},
	}
	gcp := &entityDetectorStub{
		attrs: map[string]any{"cloud.provider": "gcp", "cloud.account.id": "proj", "k8s.cluster.name": "c1"},
		entities: []internal.DetectedEntity{
			{Type: "cloud", IDKeys: []string{"cloud.provider", "cloud.account.id"}, IDScope: xentity.IdScopeGlobal},
			{Type: "k8s.cluster", IDKeys: []string{"k8s.cluster.name"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud"},
		},
	}
	factories := map[internal.DetectorType]internal.DetectorFactory{
		"k8s": func(processor.Settings, internal.DetectorConfig, bool) (internal.Detector, error) { return k8s, nil },
		"gcp": func(processor.Settings, internal.DetectorConfig, bool) (internal.Detector, error) { return gcp, nil },
	}
	provider, err := internal.NewProviderFactory(factories).CreateResourceProvider(
		processortest.NewNopSettings(metadata.Type), configretry.BackOffConfig{}, false, stubConfig{}, "k8s", "gcp")
	require.NoError(t, err)
	require.NoError(t, provider.Refresh(context.Background(), &http.Client{Timeout: time.Second}))

	rdp := &resourceDetectionProcessor{provider: provider}
	td := ptrace.NewTraces()
	// An SDK-produced resource carrying its own entities, as in the OTEL_ENTITIES proposal.
	in := td.ResourceSpans().AppendEmpty().Resource()
	in.Attributes().PutStr("service.name", "svc")
	in.Attributes().PutStr("service.instance.id", "inst-1")
	svc := xentity.ResourceEntities(in).PutEmpty("service")
	svc.IdentifyingAttributes().PutStr("service.name", "svc")
	svc.SetIDScope(xentity.IdScopeUnspecified)
	inst := xentity.ResourceEntities(in).PutEmpty("service.instance")
	inst.IdentifyingAttributes().PutStr("service.instance.id", "inst-1")
	inst.SetIDScope(xentity.IdScopeLocal)
	inst.SetIDScopeEntityType("service")

	out, err := rdp.processTraces(context.Background(), td)
	require.NoError(t, err)
	res := out.ResourceSpans().At(0).Resource()

	entities := xentity.ResourceEntities(res)
	require.Equal(t, 5, entities.Len())
	got := map[string][2]any{}
	for typ, e := range entities.All() {
		got[typ] = [2]any{e.IDScope(), e.IDScopeEntityType()}
	}
	assert.Equal(t, map[string][2]any{
		"service":          {xentity.IdScopeUnspecified, ""},
		"service.instance": {xentity.IdScopeLocal, "service"},
		"k8s.node":         {xentity.IdScopeLocal, "k8s.cluster"},
		"k8s.cluster":      {xentity.IdScopeLocal, "cloud"},
		"cloud":            {xentity.IdScopeGlobal, ""},
	}, got)

	node := xentity.ResolveGlobalIdentity(res, "k8s.node")
	assert.True(t, node.Resolved())
	assert.Equal(t, map[string]any{"k8s.node.name": "node-1", "k8s.cluster.name": "c1", "cloud.provider": "gcp", "cloud.account.id": "proj"}, node.Attributes.AsRaw())

	// The SDK's service.instance stays unresolved because its chain ends in an
	// unspecified service scope. The processor did not touch its fields.
	svcInst := xentity.ResolveGlobalIdentity(res, "service.instance")
	assert.Equal(t, xentity.UnresolvedScopeUnspecified, svcInst.Reason)
	assert.Equal(t, []string{"service.instance", "service"}, svcInst.Chain)
}
