// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/xpdata/entity"
)

func TestEncodeEntityKeys(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want string
	}{
		{
			name: "nil slice encodes as empty array",
			keys: nil,
			want: `[]`,
		},
		{
			name: "empty slice encodes as empty array",
			keys: []string{},
			want: `[]`,
		},
		{
			name: "single key",
			keys: []string{"k8s_pod_uid"},
			want: `["k8s_pod_uid"]`,
		},
		{
			name: "multiple keys",
			keys: []string{"service_instance_id", "service_name", "service_namespace"},
			want: `["service_instance_id","service_name","service_namespace"]`,
		},
		{
			name: "special utf8 characters without html escaping",
			keys: []string{"a,b", "c<d>&e", `f"g`},
			want: `["a,b","c<d>&e","f\"g"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodeEntityKeys(tt.keys)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseEntityInfo(t *testing.T) {
	tests := []struct {
		name        string
		entityType  string
		schemaURL   string
		rawIDKeys   string
		rawDescKeys string
		want        EntityInfo
		wantOK      bool
	}{
		{
			name:        "valid full entity",
			entityType:  "service.instance",
			schemaURL:   "https://opentelemetry.io/schemas/1.31.0",
			rawIDKeys:   `["service_name", "service_instance_id", "service_name"]`,
			rawDescKeys: `["service_version", "service_version"]`,
			want: EntityInfo{
				Type:            "service.instance",
				SchemaURL:       "https://opentelemetry.io/schemas/1.31.0",
				IDKeys:          []string{"service_name", "service_instance_id"},
				DescriptionKeys: []string{"service_version"},
			},
			wantOK: true,
		},
		{
			name:       "valid minimal entity with empty optional fields",
			entityType: "k8s.pod",
			rawIDKeys:  `["k8s_pod_uid"]`,
			want: EntityInfo{
				Type:   "k8s.pod",
				IDKeys: []string{"k8s_pod_uid"},
			},
			wantOK: true,
		},
		{
			name:        "empty json array description keys accepted",
			entityType:  "k8s.pod",
			rawIDKeys:   `["k8s_pod_uid"]`,
			rawDescKeys: `[]`,
			want: EntityInfo{
				Type:            "k8s.pod",
				IDKeys:          []string{"k8s_pod_uid"},
				DescriptionKeys: []string{},
			},
			wantOK: true,
		},
		{
			name:      "missing type rejected",
			rawIDKeys: `["k8s_pod_uid"]`,
			wantOK:    false,
		},
		{
			name:       "empty id keys rejected",
			entityType: "k8s.pod",
			rawIDKeys:  `[]`,
			wantOK:     false,
		},
		{
			name:       "null id keys rejected",
			entityType: "k8s.pod",
			rawIDKeys:  `null`,
			wantOK:     false,
		},
		{
			name:       "non-string id key rejected",
			entityType: "k8s.pod",
			rawIDKeys:  `[123]`,
			wantOK:     false,
		},
		{
			name:       "empty string element in id keys rejected",
			entityType: "k8s.pod",
			rawIDKeys:  `["k8s_pod_uid",""]`,
			wantOK:     false,
		},
		{
			name:        "malformed description keys rejected",
			entityType:  "k8s.pod",
			rawIDKeys:   `["k8s_pod_uid"]`,
			rawDescKeys: `not-json`,
			wantOK:      false,
		},
		{
			name:        "null description keys rejected",
			entityType:  "k8s.pod",
			rawIDKeys:   `["k8s_pod_uid"]`,
			rawDescKeys: `null`,
			wantOK:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseEntityInfo(tt.entityType, tt.schemaURL, tt.rawIDKeys, tt.rawDescKeys)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestBuildEntityInfos(t *testing.T) {
	res := pcommon.NewResource()
	res.Attributes().PutStr("service.namespace", "shop")
	res.Attributes().PutStr("service.name", "checkout")
	res.Attributes().PutStr("service.instance.id", "id-1")
	res.Attributes().PutStr("service.version", "2.4.0")
	res.Attributes().PutStr("k8s.pod.uid", "pod-uid-123")
	res.Attributes().PutStr("k8s.pod.name", "checkout-7f9")
	res.Attributes().PutStr("foo.bar", "id-val")
	res.Attributes().PutStr("foo_bar", "desc-val")
	res.Attributes().PutStr("empty.id", "")
	res.Attributes().PutStr("empty.desc", "")

	refs := entity.ResourceEntityRefs(res)

	svc := refs.AppendEmpty()
	svc.SetType("service.instance")
	svc.SetSchemaUrl("https://opentelemetry.io/schemas/1.31.0")
	svc.IdKeys().Append("service.namespace", "service.name", "service.instance.id")
	svc.DescriptionKeys().Append("service.version", "missing.attr", "empty.desc")

	// Duplicate same-type entity with same identity merges description keys.
	pod1 := refs.AppendEmpty()
	pod1.SetType("k8s.pod")
	pod1.SetSchemaUrl("https://opentelemetry.io/schemas/1.31.0")
	pod1.IdKeys().Append("k8s.pod.uid")
	pod1.DescriptionKeys().Append("k8s.pod.name")

	pod2 := refs.AppendEmpty()
	pod2.SetType("k8s.pod")
	pod2.SetSchemaUrl("https://opentelemetry.io/schemas/1.31.0")
	pod2.IdKeys().Append("k8s.pod.uid")
	pod2.DescriptionKeys().Append("service.version")

	// Key collision after underscore escaping: foo.bar (ID) vs foo_bar (desc) -> keep only in IDKeys.
	collide := refs.AppendEmpty()
	collide.SetType("custom")
	collide.IdKeys().Append("foo.bar")
	collide.DescriptionKeys().Append("foo_bar")

	// Invalid entity with missing ID key is dropped.
	invalid := refs.AppendEmpty()
	invalid.SetType("host")
	invalid.IdKeys().Append("host.id")

	// Entity with empty-valued ID key is dropped (omitted from target_info in Prometheus).
	emptyID := refs.AppendEmpty()
	emptyID.SetType("cloud")
	emptyID.IdKeys().Append("empty.id")

	underscore := func(k string) (string, error) {
		return strings.ReplaceAll(k, ".", "_"), nil
	}

	got, err := BuildEntityInfos(res, underscore)
	require.NoError(t, err)
	assert.Equal(t, []EntityInfo{
		{
			Type:            "custom",
			IDKeys:          []string{"foo_bar"},
			DescriptionKeys: []string{},
		},
		{
			Type:            "k8s.pod",
			SchemaURL:       "https://opentelemetry.io/schemas/1.31.0",
			IDKeys:          []string{"k8s_pod_uid"},
			DescriptionKeys: []string{"k8s_pod_name", "service_version"},
		},
		{
			Type:            "service.instance",
			SchemaURL:       "https://opentelemetry.io/schemas/1.31.0",
			IDKeys:          []string{"service_instance_id", "service_name", "service_namespace"},
			DescriptionKeys: []string{"service_version"},
		},
	}, got)
}

func TestApplyEntityInfos(t *testing.T) {
	res := pcommon.NewResource()
	res.Attributes().PutStr("k8s_pod_uid", "pod-uid-123")
	res.Attributes().PutStr("k8s_pod_name", "checkout-7f9")
	res.Attributes().PutStr("k8s_pod_label_env", "prod")
	res.Attributes().PutStr("host_id", "h-1")
	res.Attributes().PutStr("host_name", "node-1")

	infos := []EntityInfo{
		// Same type & identity (different order) -> description keys merged.
		{
			Type:            "k8s.pod",
			SchemaURL:       "https://opentelemetry.io/schemas/1.31.0",
			IDKeys:          []string{"k8s_pod_uid"},
			DescriptionKeys: []string{"k8s_pod_name", "missing_desc", "k8s_pod_uid"},
		},
		{
			Type:            "k8s.pod",
			SchemaURL:       "https://opentelemetry.io/schemas/1.31.0",
			IDKeys:          []string{"k8s_pod_uid"},
			DescriptionKeys: []string{"k8s_pod_label_env"},
		},
		// Same type, different schema_url -> later entry replaces earlier entry.
		{
			Type:            "host",
			SchemaURL:       "https://opentelemetry.io/schemas/1.30.0",
			IDKeys:          []string{"host_id"},
			DescriptionKeys: []string{"host_name"},
		},
		{
			Type:      "host",
			SchemaURL: "https://opentelemetry.io/schemas/1.31.0",
			IDKeys:    []string{"host_id"},
		},
		// Missing id_key in resource attributes -> ignored.
		{
			Type:   "container",
			IDKeys: []string{"container_id"},
		},
	}

	ApplyEntityInfos(res, infos)

	refs := entity.ResourceEntityRefs(res)
	require.Equal(t, 2, refs.Len())

	assert.Equal(t, "k8s.pod", refs.At(0).Type())
	assert.Equal(t, "https://opentelemetry.io/schemas/1.31.0", refs.At(0).SchemaUrl())
	assert.Equal(t, []string{"k8s_pod_uid"}, refs.At(0).IdKeys().AsRaw())
	assert.Equal(t, []string{"k8s_pod_label_env", "k8s_pod_name"}, refs.At(0).DescriptionKeys().AsRaw())

	assert.Equal(t, "host", refs.At(1).Type())
	assert.Equal(t, "https://opentelemetry.io/schemas/1.31.0", refs.At(1).SchemaUrl())
	assert.Equal(t, []string{"host_id"}, refs.At(1).IdKeys().AsRaw())
	assert.Empty(t, refs.At(1).DescriptionKeys().AsRaw())

	// Passing nil/empty infos consistently clears existing EntityRefs.
	ApplyEntityInfos(res, nil)
	assert.Equal(t, 0, entity.ResourceEntityRefs(res).Len())
}
