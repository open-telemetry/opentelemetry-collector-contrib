// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	xentity "go.opentelemetry.io/collector/pdata/xpdata/entity"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/metadata"
)

// refView is a plain view of an entity ref for assertions.
type refView struct {
	SchemaURL string
	IDKeys    []string
	DescKeys  []string
	Scope     xentity.IdScope
	ScopeType string
}

func viewRefs(res pcommon.Resource) map[string]refView {
	out := map[string]refView{}
	refs := xentity.ResourceEntityRefs(res)
	for i := 0; i < refs.Len(); i++ {
		r := refs.At(i)
		desc := r.DescriptionKeys().AsRaw()
		if desc == nil {
			desc = []string{}
		}
		out[r.Type()] = refView{
			SchemaURL: r.SchemaUrl(),
			IDKeys:    r.IdKeys().AsRaw(),
			DescKeys:  desc,
			Scope:     r.IdScope(),
			ScopeType: r.IdScopeEntityType(),
		}
	}
	return out
}

func resWith(attrs map[string]any) pcommon.Resource {
	res := pcommon.NewResource()
	require.NoError(nil, res.Attributes().FromRaw(attrs))
	return res
}

func TestAttachDetectedEntities(t *testing.T) {
	host := DetectedEntity{SchemaURL: "s", Type: "host", IDKeys: []string{"host.id"}, IDScope: xentity.IdScopeGlobal, DescriptionKeys: []string{"host.name", "os.type"}}
	cloudHost := DetectedEntity{SchemaURL: "s", Type: "host", IDKeys: []string{"host.id"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud", DescriptionKeys: []string{"host.type"}}

	tests := []struct {
		name     string
		merged   map[string]any
		detected map[string]any
		entities []DetectedEntity
		existing []DetectedEntity // refs already on merged (from an earlier detector)
		want     map[string]refView
	}{
		{
			name:     "attached with present description keys only",
			merged:   map[string]any{"host.id": "m1", "host.name": "h"},
			detected: map[string]any{"host.id": "m1", "host.name": "h"},
			entities: []DetectedEntity{host},
			want:     map[string]refView{"host": {SchemaURL: "s", IDKeys: []string{"host.id"}, DescKeys: []string{"host.name"}, Scope: xentity.IdScopeGlobal}},
		},
		{
			name:     "identity lost precedence to an earlier detector",
			merged:   map[string]any{"host.id": "i-123", "host.name": "h"},
			detected: map[string]any{"host.id": "m1", "host.name": "h"},
			entities: []DetectedEntity{host},
			want:     map[string]refView{},
		},
		{
			name:     "description key that lost precedence is not referenced",
			merged:   map[string]any{"host.id": "m1", "host.name": "other"},
			detected: map[string]any{"host.id": "m1", "host.name": "h"},
			entities: []DetectedEntity{host},
			want:     map[string]refView{"host": {SchemaURL: "s", IDKeys: []string{"host.id"}, DescKeys: []string{}, Scope: xentity.IdScopeGlobal}},
		},
		{
			name:     "same type and identity merges description keys",
			merged:   map[string]any{"host.id": "m1", "host.name": "h", "os.type": "linux"},
			detected: map[string]any{"host.id": "m1", "host.name": "h", "os.type": "linux"},
			existing: []DetectedEntity{{SchemaURL: "s", Type: "host", IDKeys: []string{"host.id"}, IDScope: xentity.IdScopeGlobal, DescriptionKeys: []string{"host.name"}}},
			entities: []DetectedEntity{host},
			want:     map[string]refView{"host": {SchemaURL: "s", IDKeys: []string{"host.id"}, DescKeys: []string{"host.name", "os.type"}, Scope: xentity.IdScopeGlobal}},
		},
		{
			name:     "same type and identity but different scope is a conflict: first wins",
			merged:   map[string]any{"host.id": "m1", "host.type": "n2"},
			detected: map[string]any{"host.id": "m1", "host.type": "n2"},
			existing: []DetectedEntity{{SchemaURL: "s", Type: "host", IDKeys: []string{"host.id"}, IDScope: xentity.IdScopeGlobal}},
			entities: []DetectedEntity{cloudHost},
			want:     map[string]refView{"host": {SchemaURL: "s", IDKeys: []string{"host.id"}, DescKeys: []string{}, Scope: xentity.IdScopeGlobal}},
		},
		{
			name:     "description key already owned by another entity is not referenced twice",
			merged:   map[string]any{"host.id": "m1", "cloud.provider": "aws", "cloud.account.id": "1", "cloud.region": "r"},
			detected: map[string]any{"host.id": "m1", "cloud.provider": "aws", "cloud.account.id": "1", "cloud.region": "r"},
			existing: []DetectedEntity{{SchemaURL: "s", Type: "cloud", IDKeys: []string{"cloud.provider", "cloud.account.id"}, IDScope: xentity.IdScopeGlobal, DescriptionKeys: []string{"cloud.region"}}},
			entities: []DetectedEntity{{SchemaURL: "s", Type: "host", IDKeys: []string{"host.id"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud", DescriptionKeys: []string{"cloud.region"}}},
			want: map[string]refView{
				"cloud": {SchemaURL: "s", IDKeys: []string{"cloud.provider", "cloud.account.id"}, DescKeys: []string{"cloud.region"}, Scope: xentity.IdScopeGlobal},
				"host":  {SchemaURL: "s", IDKeys: []string{"host.id"}, DescKeys: []string{}, Scope: xentity.IdScopeLocal, ScopeType: "cloud"},
			},
		},
		{
			name:     "non-conforming declarations are skipped",
			merged:   map[string]any{"a.id": "1", "b.id": "2", "c.id": "3", "d.id": "4"},
			detected: map[string]any{"a.id": "1", "b.id": "2", "c.id": "3", "d.id": "4"},
			entities: []DetectedEntity{
				{Type: "a", IDKeys: []string{"a.id"}, IDScope: xentity.IdScopeLocal},                              // local without scope type
				{Type: "b", IDKeys: []string{"b.id"}, IDScope: xentity.IdScopeGlobal, IDScopeEntityType: "x"},     // global with scope type
				{Type: "c", IDKeys: []string{"c.id"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "c"},      // self-scoped
				{Type: "d", IDKeys: []string{"d.id"}, IDScope: xentity.IdScopeUnspecified, IDScopeEntityType: ""}, // fine
				{Type: "", IDKeys: []string{"d.id"}},
				{Type: "e"},
			},
			want: map[string]refView{"d": {IDKeys: []string{"d.id"}, DescKeys: []string{}, Scope: xentity.IdScopeUnspecified}},
		},
		{
			name:     "local entity is skipped when its scoping entity from the same detector lost precedence",
			merged:   map[string]any{"host.id": "i-1", "cloud.provider": "aws", "cloud.account.id": "other"},
			detected: map[string]any{"host.id": "i-1", "cloud.provider": "aws", "cloud.account.id": "mine"},
			entities: []DetectedEntity{
				{Type: "cloud", IDKeys: []string{"cloud.provider", "cloud.account.id"}, IDScope: xentity.IdScopeGlobal},
				{Type: "host", IDKeys: []string{"host.id"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud"},
			},
			want: map[string]refView{},
		},
		{
			name:     "duplicate or overlapping keys are rejected",
			merged:   map[string]any{"a.id": "1", "b.id": "2", "b.x": "3"},
			detected: map[string]any{"a.id": "1", "b.id": "2", "b.x": "3"},
			entities: []DetectedEntity{
				{Type: "a", IDKeys: []string{"a.id", "a.id"}, IDScope: xentity.IdScopeGlobal},
				{Type: "b", IDKeys: []string{"b.id"}, DescriptionKeys: []string{"b.id", "b.x"}, IDScope: xentity.IdScopeGlobal},
			},
			want: map[string]refView{},
		},
		{
			name:     "scoping entity need not be present",
			merged:   map[string]any{"k8s.node.name": "n1"},
			detected: map[string]any{"k8s.node.name": "n1"},
			entities: []DetectedEntity{{Type: "k8s.node", IDKeys: []string{"k8s.node.name"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "k8s.cluster"}},
			want:     map[string]refView{"k8s.node": {IDKeys: []string{"k8s.node.name"}, DescKeys: []string{}, Scope: xentity.IdScopeLocal, ScopeType: "k8s.cluster"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := resWith(tt.merged)
			detected := resWith(tt.detected)
			for _, e := range tt.existing {
				putRef(xentity.ResourceEntityRefs(merged).AppendEmpty(), e, e.DescriptionKeys)
			}
			attachDetectedEntities(merged, detected, tt.entities, zap.NewNop())
			assert.Equal(t, tt.want, viewRefs(merged))
			// Attributes are never touched by entity attachment.
			assert.Equal(t, tt.merged, merged.Attributes().AsRaw())
		})
	}
}

func TestMergeResourceAndEntities(t *testing.T) {
	detected := func() pcommon.Resource {
		res := resWith(map[string]any{
			"host.id": "m1", "host.name": "h",
			"cloud.provider": "gcp", "cloud.account.id": "p", "cloud.region": "r",
		})
		refs := xentity.ResourceEntityRefs(res)
		putRef(refs.AppendEmpty(), DetectedEntity{SchemaURL: "s", Type: "cloud", IDKeys: []string{"cloud.provider", "cloud.account.id"}, IDScope: xentity.IdScopeGlobal}, []string{"cloud.region"})
		putRef(refs.AppendEmpty(), DetectedEntity{SchemaURL: "s", Type: "host", IDKeys: []string{"host.id"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud"}, []string{"host.name"})
		return res
	}
	wantCloud := refView{SchemaURL: "s", IDKeys: []string{"cloud.provider", "cloud.account.id"}, DescKeys: []string{"cloud.region"}, Scope: xentity.IdScopeGlobal}
	wantHost := refView{SchemaURL: "s", IDKeys: []string{"host.id"}, DescKeys: []string{"host.name"}, Scope: xentity.IdScopeLocal, ScopeType: "cloud"}

	tests := []struct {
		name      string
		override  bool
		incoming  func() pcommon.Resource
		wantAttrs map[string]any
		wantRefs  map[string]refView
	}{
		{
			name:     "empty incoming resource gets everything",
			incoming: func() pcommon.Resource { return pcommon.NewResource() },
			wantAttrs: map[string]any{
				"host.id": "m1", "host.name": "h",
				"cloud.provider": "gcp", "cloud.account.id": "p", "cloud.region": "r",
			},
			wantRefs: map[string]refView{"cloud": wantCloud, "host": wantHost},
		},
		{
			name: "incoming entity of another type is left untouched including its scope",
			incoming: func() pcommon.Resource {
				res := resWith(map[string]any{"service.name": "svc", "service.instance.id": "i1"})
				refs := xentity.ResourceEntityRefs(res)
				putRef(refs.AppendEmpty(), DetectedEntity{Type: "service", IDKeys: []string{"service.name"}, IDScope: xentity.IdScopeUnspecified}, nil)
				putRef(refs.AppendEmpty(), DetectedEntity{Type: "service.instance", IDKeys: []string{"service.instance.id"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "service"}, nil)
				return res
			},
			wantAttrs: map[string]any{
				"service.name": "svc", "service.instance.id": "i1",
				"host.id": "m1", "host.name": "h",
				"cloud.provider": "gcp", "cloud.account.id": "p", "cloud.region": "r",
			},
			wantRefs: map[string]refView{
				"service":          {IDKeys: []string{"service.name"}, DescKeys: []string{}, Scope: xentity.IdScopeUnspecified},
				"service.instance": {IDKeys: []string{"service.instance.id"}, DescKeys: []string{}, Scope: xentity.IdScopeLocal, ScopeType: "service"},
				"cloud":            wantCloud,
				"host":             wantHost,
			},
		},
		{
			name: "no override: incoming identity wins and the detected entity with a different identity value is skipped",
			incoming: func() pcommon.Resource {
				res := resWith(map[string]any{"host.id": "sdk-host"})
				putRef(xentity.ResourceEntityRefs(res).AppendEmpty(), DetectedEntity{Type: "host", IDKeys: []string{"host.id"}, IDScope: xentity.IdScopeGlobal}, nil)
				return res
			},
			wantAttrs: map[string]any{
				"host.id": "sdk-host", "host.name": "h",
				"cloud.provider": "gcp", "cloud.account.id": "p", "cloud.region": "r",
			},
			wantRefs: map[string]refView{
				"host":  {IDKeys: []string{"host.id"}, DescKeys: []string{}, Scope: xentity.IdScopeGlobal},
				"cloud": wantCloud,
			},
		},
		{
			name: "no override: plain incoming attribute with a different value hides the detected entity",
			incoming: func() pcommon.Resource {
				return resWith(map[string]any{"host.id": "sdk-host"})
			},
			wantAttrs: map[string]any{
				"host.id": "sdk-host", "host.name": "h",
				"cloud.provider": "gcp", "cloud.account.id": "p", "cloud.region": "r",
			},
			wantRefs: map[string]refView{"cloud": wantCloud},
		},
		{
			name:     "override: detected entity replaces incoming entity of the same type with different identity",
			override: true,
			incoming: func() pcommon.Resource {
				res := resWith(map[string]any{"host.id": "sdk-host", "host.arch": "arm64"})
				putRef(xentity.ResourceEntityRefs(res).AppendEmpty(), DetectedEntity{Type: "host", IDKeys: []string{"host.id"}, IDScope: xentity.IdScopeGlobal}, []string{"host.arch"})
				return res
			},
			wantAttrs: map[string]any{
				"host.id": "m1", "host.name": "h", "host.arch": "arm64",
				"cloud.provider": "gcp", "cloud.account.id": "p", "cloud.region": "r",
			},
			wantRefs: map[string]refView{"cloud": wantCloud, "host": wantHost},
		},
		{
			name:     "override: overwriting an attribute owned by an incoming entity of another type drops that entity ref",
			override: true,
			incoming: func() pcommon.Resource {
				res := resWith(map[string]any{"k8s.node.name": "n", "host.name": "node-host"})
				putRef(xentity.ResourceEntityRefs(res).AppendEmpty(), DetectedEntity{Type: "k8s.node", IDKeys: []string{"k8s.node.name"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "k8s.cluster"}, []string{"host.name"})
				return res
			},
			wantAttrs: map[string]any{
				"k8s.node.name": "n", "host.id": "m1", "host.name": "h",
				"cloud.provider": "gcp", "cloud.account.id": "p", "cloud.region": "r",
			},
			wantRefs: map[string]refView{"cloud": wantCloud, "host": wantHost},
		},
		{
			name: "no override: incoming cloud with a different identity hides the detected host scoped by cloud",
			incoming: func() pcommon.Resource {
				res := resWith(map[string]any{"cloud.provider": "gcp", "cloud.account.id": "sdk-project"})
				putRef(xentity.ResourceEntityRefs(res).AppendEmpty(), DetectedEntity{Type: "cloud", IDKeys: []string{"cloud.provider", "cloud.account.id"}, IDScope: xentity.IdScopeGlobal}, nil)
				return res
			},
			wantAttrs: map[string]any{
				"host.id": "m1", "host.name": "h",
				"cloud.provider": "gcp", "cloud.account.id": "sdk-project", "cloud.region": "r",
			},
			wantRefs: map[string]refView{
				"cloud": {IDKeys: []string{"cloud.provider", "cloud.account.id"}, DescKeys: []string{}, Scope: xentity.IdScopeGlobal},
			},
		},
		{
			name: "same type, identity and scope merge description keys",
			incoming: func() pcommon.Resource {
				res := resWith(map[string]any{"host.id": "m1", "host.arch": "arm64"})
				putRef(xentity.ResourceEntityRefs(res).AppendEmpty(), DetectedEntity{SchemaURL: "s", Type: "host", IDKeys: []string{"host.id"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud"}, []string{"host.arch"})
				return res
			},
			wantAttrs: map[string]any{
				"host.id": "m1", "host.name": "h", "host.arch": "arm64",
				"cloud.provider": "gcp", "cloud.account.id": "p", "cloud.region": "r",
			},
			wantRefs: map[string]refView{
				"cloud": wantCloud,
				"host":  {SchemaURL: "s", IDKeys: []string{"host.id"}, DescKeys: []string{"host.arch", "host.name"}, Scope: xentity.IdScopeLocal, ScopeType: "cloud"},
			},
		},
		{
			name: "same type and identity but incoming has a different scope: kept as is without override",
			incoming: func() pcommon.Resource {
				res := resWith(map[string]any{"host.id": "m1"})
				putRef(xentity.ResourceEntityRefs(res).AppendEmpty(), DetectedEntity{SchemaURL: "s", Type: "host", IDKeys: []string{"host.id"}, IDScope: xentity.IdScopeGlobal}, nil)
				return res
			},
			wantAttrs: map[string]any{
				"host.id": "m1", "host.name": "h",
				"cloud.provider": "gcp", "cloud.account.id": "p", "cloud.region": "r",
			},
			wantRefs: map[string]refView{
				"cloud": wantCloud,
				"host":  {SchemaURL: "s", IDKeys: []string{"host.id"}, DescKeys: []string{}, Scope: xentity.IdScopeGlobal},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from := detected()
			to := tt.incoming()
			MergeResourceAndEntities(to, from, tt.override)
			assert.Equal(t, tt.wantAttrs, to.Attributes().AsRaw())
			assert.Equal(t, tt.wantRefs, viewRefs(to))
			// The detected resource is shared across calls and must stay intact.
			assert.Equal(t, viewRefs(detected()), viewRefs(from))
			assert.Equal(t, detected().Attributes().AsRaw(), from.Attributes().AsRaw())
		})
	}
}

type mockEntityDetector struct {
	attrs    map[string]any
	entities []DetectedEntity
}

func (d *mockEntityDetector) Detect(context.Context) (pcommon.Resource, string, error) {
	return resWith(d.attrs), "", nil
}

func (d *mockEntityDetector) Entities(pcommon.Resource) []DetectedEntity {
	return d.entities
}

func newProviderWith(t *testing.T, order []string, detectors map[string]*mockEntityDetector) pcommon.Resource {
	factories := map[DetectorType]DetectorFactory{}
	for name, d := range detectors {
		factories[DetectorType(name)] = func(processor.Settings, DetectorConfig, bool) (Detector, error) { return d, nil }
	}
	types := make([]DetectorType, len(order))
	for i, o := range order {
		types[i] = DetectorType(o)
	}
	p, err := NewProviderFactory(factories).CreateResourceProvider(processortest.NewNopSettings(metadata.Type), fastRetryConfig, false, &mockDetectorConfig{}, types...)
	require.NoError(t, err)
	require.NoError(t, p.Refresh(context.Background(), &http.Client{Timeout: time.Second}))
	res, _, err := p.Get(context.Background(), nil)
	require.NoError(t, err)
	return res
}

// A k8s.node scoped by k8s.cluster and a k8s.cluster scoped by cloud, each
// produced by a different detector. Neither detector knows about the other, no
// detector modifies another's entity, and the result is the same for every
// detector order because each declaration describes only its own ID.
func TestDetectResourceEntitiesOrderIndependent(t *testing.T) {
	detectors := map[string]*mockEntityDetector{
		"k8s": {
			attrs: map[string]any{"k8s.node.name": "node-1"},
			entities: []DetectedEntity{{Type: "k8s.node", IDKeys: []string{"k8s.node.name"},
				IDScope: xentity.IdScopeLocal, IDScopeEntityType: "k8s.cluster"}},
		},
		"gcp": {
			attrs: map[string]any{"cloud.provider": "gcp", "cloud.account.id": "proj", "cloud.region": "r", "k8s.cluster.name": "c1"},
			entities: []DetectedEntity{
				{Type: "cloud", IDKeys: []string{"cloud.provider", "cloud.account.id"}, IDScope: xentity.IdScopeGlobal, DescriptionKeys: []string{"cloud.region"}},
				{Type: "k8s.cluster", IDKeys: []string{"k8s.cluster.name"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud"},
			},
		},
		"system": {
			attrs:    map[string]any{"host.id": "machine-1", "host.name": "h"},
			entities: []DetectedEntity{{Type: "host", IDKeys: []string{"host.id"}, IDScope: xentity.IdScopeGlobal, DescriptionKeys: []string{"host.name"}}},
		},
	}
	want := map[string]refView{
		"k8s.node":    {IDKeys: []string{"k8s.node.name"}, DescKeys: []string{}, Scope: xentity.IdScopeLocal, ScopeType: "k8s.cluster"},
		"k8s.cluster": {IDKeys: []string{"k8s.cluster.name"}, DescKeys: []string{}, Scope: xentity.IdScopeLocal, ScopeType: "cloud"},
		"cloud":       {IDKeys: []string{"cloud.provider", "cloud.account.id"}, DescKeys: []string{"cloud.region"}, Scope: xentity.IdScopeGlobal},
		"host":        {IDKeys: []string{"host.id"}, DescKeys: []string{"host.name"}, Scope: xentity.IdScopeGlobal},
	}
	wantGID := map[string]any{"k8s.node.name": "node-1", "k8s.cluster.name": "c1", "cloud.provider": "gcp", "cloud.account.id": "proj"}

	for _, order := range [][]string{{"k8s", "gcp", "system"}, {"gcp", "k8s", "system"}, {"system", "gcp", "k8s"}, {"system", "k8s", "gcp"}, {"gcp", "system", "k8s"}, {"k8s", "system", "gcp"}} {
		t.Run(fmt.Sprint(order), func(t *testing.T) {
			res := newProviderWith(t, order, detectors)
			assert.Equal(t, want, viewRefs(res))
			gid := xentity.ResolveGlobalIdentity(res, "k8s.node")
			assert.True(t, gid.Resolved(), gid.Reason.String())
			assert.Equal(t, wantGID, gid.Attributes.AsRaw())
			assert.Equal(t, []string{"k8s.node", "k8s.cluster", "cloud"}, gid.Chain)
		})
	}

	// Without the gcp detector the node is still reported, with an unresolved identity.
	res := newProviderWith(t, []string{"k8s", "system"}, detectors)
	gid := xentity.ResolveGlobalIdentity(res, "k8s.node")
	assert.False(t, gid.Resolved())
	assert.Equal(t, xentity.UnresolvedScopeEntityMissing, gid.Reason)
	assert.Equal(t, map[string]any{"k8s.node.name": "node-1"}, gid.Attributes.AsRaw())
}

// ec2 and k8s_api both produce host.id with different values and different
// scopes. The attribute follows detector precedence, and so does the entity.
func TestDetectResourceEntitiesIdentityConflictFollowsPrecedence(t *testing.T) {
	detectors := map[string]*mockEntityDetector{
		"ec2": {
			attrs: map[string]any{"cloud.provider": "aws", "cloud.account.id": "1", "host.id": "i-abc", "host.type": "m5"},
			entities: []DetectedEntity{
				{Type: "cloud", IDKeys: []string{"cloud.provider", "cloud.account.id"}, IDScope: xentity.IdScopeGlobal},
				{Type: "host", IDKeys: []string{"host.id"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud", DescriptionKeys: []string{"host.type"}},
			},
		},
		"k8s": {
			attrs: map[string]any{"host.id": "machine-1", "k8s.node.uid": "u1"},
			entities: []DetectedEntity{
				{Type: "host", IDKeys: []string{"host.id"}, IDScope: xentity.IdScopeGlobal},
				{Type: "k8s.node", IDKeys: []string{"k8s.node.uid"}, IDScope: xentity.IdScopeGlobal},
			},
		},
	}

	res := newProviderWith(t, []string{"ec2", "k8s"}, detectors)
	assert.Equal(t, "i-abc", res.Attributes().AsRaw()["host.id"])
	assert.Equal(t, refView{IDKeys: []string{"host.id"}, DescKeys: []string{"host.type"}, Scope: xentity.IdScopeLocal, ScopeType: "cloud"}, viewRefs(res)["host"])
	assert.Contains(t, viewRefs(res), "k8s.node")

	res = newProviderWith(t, []string{"k8s", "ec2"}, detectors)
	assert.Equal(t, "machine-1", res.Attributes().AsRaw()["host.id"])
	assert.Equal(t, refView{IDKeys: []string{"host.id"}, DescKeys: []string{}, Scope: xentity.IdScopeGlobal}, viewRefs(res)["host"])
	// host.type came from ec2 but ec2's host entity was not attached; the attribute stays unassociated.
	assert.Equal(t, "m5", res.Attributes().AsRaw()["host.type"])
	assert.Contains(t, viewRefs(res), "cloud")
}

// Composite uniqueness scopes expressed through the cloud location hierarchy:
// a zonal GKE cluster name is unique within project and zone, an EC2 instance
// ID within account and region, a Lambda function name within account and
// region. Each is a single scope entity type whose chain carries the rest.
func TestDetectResourceEntitiesCloudHierarchy(t *testing.T) {
	gkeAttrs := map[string]any{"cloud.provider": "gcp", "cloud.account.id": "proj", "cloud.region": "us-central1", "cloud.availability_zone": "us-central1-a", "k8s.cluster.name": "prod", "host.id": "42"}
	gkeRes := resWith(gkeAttrs)
	gke := &mockEntityDetector{attrs: gkeAttrs, entities: append(CloudEntities(gkeRes.Attributes(), "s"),
		DetectedEntity{SchemaURL: "s", Type: "k8s.cluster", IDKeys: []string{"k8s.cluster.name"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud.availability_zone"},
		DetectedEntity{SchemaURL: "s", Type: "host", IDKeys: []string{"host.id"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud"},
	)}
	k8s := &mockEntityDetector{
		attrs:    map[string]any{"k8s.node.name": "node-1"},
		entities: []DetectedEntity{{SchemaURL: "s", Type: "k8s.node", IDKeys: []string{"k8s.node.name"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "k8s.cluster"}},
	}

	for _, order := range [][]string{{"k8s", "gke"}, {"gke", "k8s"}} {
		res := newProviderWith(t, order, map[string]*mockEntityDetector{"k8s": k8s, "gke": gke})
		node := xentity.ResolveGlobalIdentity(res, "k8s.node")
		require.True(t, node.Resolved(), node.Reason.String())
		assert.Equal(t, []string{"k8s.node", "k8s.cluster", "cloud.availability_zone", "cloud"}, node.Chain)
		assert.Equal(t, map[string]any{"k8s.node.name": "node-1", "k8s.cluster.name": "prod", "cloud.availability_zone": "us-central1-a", "cloud.provider": "gcp", "cloud.account.id": "proj"}, node.Attributes.AsRaw())
		host := xentity.ResolveGlobalIdentity(res, "host")
		assert.Equal(t, []string{"host", "cloud"}, host.Chain)
		// cloud.region is on the resource and resolvable on its own, but not part of the zonal cluster's identity.
		assert.Equal(t, []string{"cloud.region", "cloud"}, xentity.ResolveGlobalIdentity(res, "cloud.region").Chain)
	}

	// Lambda knows the region but not the account: faas -> cloud.region -> (cloud missing) is unresolved.
	lambdaAttrs := map[string]any{"cloud.provider": "aws", "cloud.region": "eu-west-1", "faas.name": "fn"}
	lambda := &mockEntityDetector{attrs: lambdaAttrs, entities: []DetectedEntity{
		{SchemaURL: "s", Type: "cloud.region", IDKeys: []string{"cloud.region"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud"},
		{SchemaURL: "s", Type: "faas", IDKeys: []string{"faas.name"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud.region"},
	}}
	res := newProviderWith(t, []string{"lambda"}, map[string]*mockEntityDetector{"lambda": lambda})
	fn := xentity.ResolveGlobalIdentity(res, "faas")
	assert.Equal(t, xentity.UnresolvedScopeEntityMissing, fn.Reason)
	assert.Equal(t, []string{"faas", "cloud.region"}, fn.Chain)

	// A detector that knows the account (same provider) completes the chain without touching faas.
	acct := &mockEntityDetector{attrs: map[string]any{"cloud.provider": "aws", "cloud.account.id": "123"}, entities: []DetectedEntity{
		{SchemaURL: "s", Type: "cloud", IDKeys: []string{"cloud.provider", "cloud.account.id"}, IDScope: xentity.IdScopeGlobal},
	}}
	res = newProviderWith(t, []string{"lambda", "acct"}, map[string]*mockEntityDetector{"lambda": lambda, "acct": acct})
	fn = xentity.ResolveGlobalIdentity(res, "faas")
	require.True(t, fn.Resolved(), fn.Reason.String())
	assert.Equal(t, map[string]any{"faas.name": "fn", "cloud.region": "eu-west-1", "cloud.provider": "aws", "cloud.account.id": "123"}, fn.Attributes.AsRaw())
}

// The scenario from the review thread: a generic faas entity whose scope differs
// per platform, scoped on Azure by a generic "cloud.app" whose Azure instance is
// in turn scoped by a resource group. Nothing in the chain needs to know which
// platform refinement produced the entities: each producer recorded its own
// entity's scope, and the consumer walks by type.
func TestDetectResourceEntitiesPlatformRefinementChain(t *testing.T) {
	azure := &mockEntityDetector{
		attrs: map[string]any{
			"cloud.provider": "azure", "cloud.account.id": "sub-1", "cloud.region": "eastus",
			"azure.resource_group.name": "rg-1", "service.name": "my-func-app", "faas.name": "my-func-app/HttpTrigger",
			"faas.instance": "a1b2c3",
		},
		entities: []DetectedEntity{
			{SchemaURL: "s", Type: "cloud", IDKeys: []string{"cloud.provider", "cloud.account.id"}, IDScope: xentity.IdScopeGlobal},
			{SchemaURL: "s", Type: "azure.resource_group", IDKeys: []string{"azure.resource_group.name"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud"},
			// Generic app entity; on Azure its producer scopes it by the resource group.
			{SchemaURL: "s", Type: "cloud.app", IDKeys: []string{"service.name"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "azure.resource_group"},
			// Generic function entity; on Azure its producer scopes it by the app.
			{SchemaURL: "s", Type: "faas", IDKeys: []string{"faas.name"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud.app"},
			{SchemaURL: "s", Type: "faas.instance", IDKeys: []string{"faas.instance"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "faas"},
		},
	}
	res := newProviderWith(t, []string{"azure"}, map[string]*mockEntityDetector{"azure": azure})
	inst := xentity.ResolveGlobalIdentity(res, "faas.instance")
	require.True(t, inst.Resolved(), inst.Reason.String())
	assert.Equal(t, []string{"faas.instance", "faas", "cloud.app", "azure.resource_group", "cloud"}, inst.Chain)
	assert.Equal(t, map[string]any{
		"faas.instance": "a1b2c3", "faas.name": "my-func-app/HttpTrigger", "service.name": "my-func-app",
		"azure.resource_group.name": "rg-1", "cloud.provider": "azure", "cloud.account.id": "sub-1",
	}, inst.Attributes.AsRaw())

	// Same generic types on Lambda, where the producer scopes faas by the region
	// and there is no app. The consumer code is identical.
	lambda := &mockEntityDetector{
		attrs: map[string]any{"cloud.provider": "aws", "cloud.account.id": "123", "cloud.region": "us-east-1", "faas.name": "fn", "faas.instance": "2026/11/01/[$LATEST]abc"},
		entities: append(CloudEntities(resWith(map[string]any{"cloud.provider": "aws", "cloud.account.id": "123", "cloud.region": "us-east-1"}).Attributes(), "s"),
			DetectedEntity{SchemaURL: "s", Type: "faas", IDKeys: []string{"faas.name"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "cloud.region"},
			DetectedEntity{SchemaURL: "s", Type: "faas.instance", IDKeys: []string{"faas.instance"}, IDScope: xentity.IdScopeLocal, IDScopeEntityType: "faas"},
		),
	}
	res = newProviderWith(t, []string{"lambda"}, map[string]*mockEntityDetector{"lambda": lambda})
	inst = xentity.ResolveGlobalIdentity(res, "faas.instance")
	require.True(t, inst.Resolved(), inst.Reason.String())
	assert.Equal(t, []string{"faas.instance", "faas", "cloud.region", "cloud"}, inst.Chain)
}
