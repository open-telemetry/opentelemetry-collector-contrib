// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package servicegraphconnector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestGetFirstMatchingValue(t *testing.T) {
	attr1 := pcommon.NewMap()
	attr1.PutStr("key1", "value1")
	attr1.PutStr("key2", "value2")
	attrs := []pcommon.Map{attr1}

	tests := []struct {
		name      string
		keys      []string
		want      string
		wantFound bool
	}{
		{
			name:      "Found in first attribute",
			keys:      []string{"key1"},
			want:      "value1",
			wantFound: true,
		},
		{
			name:      "Found in second attribute",
			keys:      []string{"key2"},
			want:      "value2",
			wantFound: true,
		},
		{
			name:      "Not found",
			keys:      []string{"key3"},
			want:      "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotFound := getFirstMatchingValue(tt.keys, attrs...)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantFound, gotFound)
		})
	}
}

// TestDefaultAttributesSupportLegacyAndCurrentSemconv verifies that the default
// peer and database-name attribute lists match spans emitted with either the
// legacy or the current semantic conventions (dual-support).
func TestDefaultAttributesSupportLegacyAndCurrentSemconv(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		attr string
	}{
		// defaultDatabaseNameAttributes
		{name: "database name - legacy db.name", keys: defaultDatabaseNameAttributes, attr: "db.name"},
		{name: "database name - current db.namespace", keys: defaultDatabaseNameAttributes, attr: "db.namespace"},
		// defaultPeerAttributes
		{name: "peer - peer.service", keys: defaultPeerAttributes, attr: "peer.service"},
		{name: "peer - legacy db.name", keys: defaultPeerAttributes, attr: "db.name"},
		{name: "peer - current db.namespace", keys: defaultPeerAttributes, attr: "db.namespace"},
		{name: "peer - legacy db.system", keys: defaultPeerAttributes, attr: "db.system"},
		{name: "peer - current db.system.name", keys: defaultPeerAttributes, attr: "db.system.name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := pcommon.NewMap()
			attrs.PutStr(tt.attr, "expected-value")
			got, found := getFirstMatchingValue(tt.keys, attrs)
			assert.True(t, found)
			assert.Equal(t, "expected-value", got)
		})
	}
}

// TestDefaultAttributesLegacyPrecedence documents that when a span carries both
// the legacy and the current key, the legacy value wins — preserving the exact
// behavior components had before the semconv migration.
func TestDefaultAttributesLegacyPrecedence(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("db.name", "legacy")
	attrs.PutStr("db.namespace", "current")

	got, found := getFirstMatchingValue(defaultDatabaseNameAttributes, attrs)
	assert.True(t, found)
	assert.Equal(t, "legacy", got)
}
