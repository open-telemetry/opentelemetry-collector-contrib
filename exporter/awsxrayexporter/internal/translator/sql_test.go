// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/metadata"
)

func enableGate(t *testing.T, gate *featuregate.Gate) {
	prev := gate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), true))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), prev))
	})
}

func TestClientSpanWithNewAndLegacyDBAttributesPrefersNew(t *testing.T) {
	enableGate(t, metadata.ExporterAwsxrayEmitV1DBConventionsFeatureGate)

	attributes := make(map[string]pcommon.Value)
	attributes["db.system.name"] = pcommon.NewValueStr("postgresql")
	attributes["db.namespace"] = pcommon.NewValueStr("orders")
	attributes["db.query.text"] = pcommon.NewValueStr("SELECT 1")
	attributes["user.name"] = pcommon.NewValueStr("new_user")
	attributes["db.system"] = pcommon.NewValueStr("mysql")
	attributes["db.name"] = pcommon.NewValueStr("customers")
	attributes["db.statement"] = pcommon.NewValueStr("SELECT 2")
	attributes["db.user"] = pcommon.NewValueStr("legacy_user")
	span := constructSQLSpan(attributes)

	// Map iteration order is random; repeat to make an order-dependent bug surface.
	for range 50 {
		_, sqlData := makeSQL(span, attributes)
		require.NotNil(t, sqlData)
		assert.Equal(t, "postgresql", *sqlData.DatabaseType)
		assert.Equal(t, "localhost/orders", *sqlData.ConnectionString)
		assert.Equal(t, "SELECT 1", *sqlData.SanitizedQuery)
		assert.Equal(t, "new_user", *sqlData.User)
	}
}

func TestClientSpanWithLegacyDBAttributesOnlyWithEmitV1(t *testing.T) {
	enableGate(t, metadata.ExporterAwsxrayEmitV1DBConventionsFeatureGate)

	attributes := make(map[string]pcommon.Value)
	attributes["db.system"] = pcommon.NewValueStr("mysql")
	attributes["db.name"] = pcommon.NewValueStr("customers")
	span := constructSQLSpan(attributes)

	_, sqlData := makeSQL(span, attributes)
	require.NotNil(t, sqlData)
	assert.Equal(t, "mysql", *sqlData.DatabaseType)
	assert.Equal(t, "localhost/customers", *sqlData.ConnectionString)
}

func TestClientSpanWithStatementAttribute(t *testing.T) {
	attributes := make(map[string]pcommon.Value)
	attributes["db.system"] = pcommon.NewValueStr("mysql")
	attributes["db.name"] = pcommon.NewValueStr("customers")
	attributes["db.statement"] = pcommon.NewValueStr("SELECT * FROM user WHERE user_id = ?")
	attributes["db.user"] = pcommon.NewValueStr("readonly_user")
	attributes["db.connection_string"] = pcommon.NewValueStr("mysql://db.example.com:3306")
	attributes["net.peer.name"] = pcommon.NewValueStr("db.example.com")
	attributes["net.peer.port"] = pcommon.NewValueStr("3306")
	span := constructSQLSpan(attributes)

	filtered, sqlData := makeSQL(span, attributes)

	assert.NotNil(t, filtered)
	assert.NotNil(t, sqlData)

	w := testWriters.borrow()
	require.NoError(t, w.Encode(sqlData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "mysql://db.example.com:3306/customers")
}

func TestClientSpanWithNonSQLDatabase(t *testing.T) {
	attributes := make(map[string]pcommon.Value)
	attributes["db.system"] = pcommon.NewValueStr("redis")
	attributes["db.name"] = pcommon.NewValueStr("0")
	attributes["db.statement"] = pcommon.NewValueStr("SET key value")
	attributes["db.user"] = pcommon.NewValueStr("readonly_user")
	attributes["db.connection_string"] = pcommon.NewValueStr("redis://db.example.com:3306")
	attributes["net.peer.name"] = pcommon.NewValueStr("db.example.com")
	attributes["net.peer.port"] = pcommon.NewValueStr("3306")
	span := constructSQLSpan(attributes)

	filtered, sqlData := makeSQL(span, attributes)
	assert.Nil(t, sqlData)
	assert.NotNil(t, filtered)
}

func TestClientSpanWithoutDBurlAttribute(t *testing.T) {
	attributes := make(map[string]pcommon.Value)
	attributes["db.system"] = pcommon.NewValueStr("postgresql")
	attributes["db.name"] = pcommon.NewValueStr("customers")
	attributes["db.statement"] = pcommon.NewValueStr("SELECT * FROM user WHERE user_id = ?")
	attributes["db.user"] = pcommon.NewValueStr("readonly_user")
	attributes["db.connection_string"] = pcommon.NewValueStr("")
	attributes["net.peer.name"] = pcommon.NewValueStr("db.example.com")
	attributes["net.peer.port"] = pcommon.NewValueStr("3306")
	span := constructSQLSpan(attributes)

	filtered, sqlData := makeSQL(span, attributes)
	assert.NotNil(t, filtered)
	assert.NotNil(t, sqlData)

	assert.Equal(t, "users.findUnique", *sqlData.URL)
}

func constructSQLSpan(attributes map[string]pcommon.Value) ptrace.Span {
	endTime := time.Now().Round(time.Second)
	startTime := endTime.Add(-90 * time.Second)

	// constructSpanAttributes() in segment_test accepts a map of interfaces...
	interfaceAttributes := make(map[string]any)
	for k, v := range attributes {
		interfaceAttributes[k] = v
	}
	spanAttributes := constructSpanAttributes(interfaceAttributes)

	span := ptrace.NewSpan()
	span.SetTraceID(newTraceID())
	span.SetSpanID(newSegmentID())
	span.SetParentSpanID(newSegmentID())
	span.SetName("users.findUnique")
	span.SetKind(ptrace.SpanKindClient)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

	status := ptrace.NewStatus()
	status.SetCode(0)
	status.SetMessage("OK")
	status.CopyTo(span.Status())

	spanAttributes.CopyTo(span.Attributes())
	return span
}
