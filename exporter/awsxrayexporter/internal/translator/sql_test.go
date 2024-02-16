// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

func TestClientSpanWithStatementAttribute(t *testing.T) {
	attributes := make(map[string]pcommon.Value)
	attributes[conventions.AttributeDBSystem] = pcommon.NewValueStr("mysql")
	attributes[conventions.AttributeDBName] = pcommon.NewValueStr("customers")
	attributes[conventions.AttributeDBStatement] = pcommon.NewValueStr("SELECT * FROM user WHERE user_id = ?")
	attributes[conventions.AttributeDBUser] = pcommon.NewValueStr("readonly_user")
	attributes[conventions.AttributeDBConnectionString] = pcommon.NewValueStr("mysql://db.example.com:3306")
	attributes[conventions.AttributeNetPeerName] = pcommon.NewValueStr("db.example.com")
	attributes[conventions.AttributeNetPeerPort] = pcommon.NewValueStr("3306")
	span := constructSQLSpan(attributes)

	filtered, sqlData := makeSQL(span, attributes)

	assert.NotNil(t, filtered)
	assert.NotNil(t, sqlData)

	w := testWriters.borrow()
	require.NoError(t, w.Encode(sqlData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.True(t, strings.Contains(jsonStr, "mysql://db.example.com:3306/customers"))
}

func TestClientSpanWithNonSQLDatabase(t *testing.T) {
	attributes := make(map[string]pcommon.Value)
	attributes[conventions.AttributeDBSystem] = pcommon.NewValueStr("redis")
	attributes[conventions.AttributeDBName] = pcommon.NewValueStr("0")
	attributes[conventions.AttributeDBStatement] = pcommon.NewValueStr("SET key value")
	attributes[conventions.AttributeDBUser] = pcommon.NewValueStr("readonly_user")
	attributes[conventions.AttributeDBConnectionString] = pcommon.NewValueStr("redis://db.example.com:3306")
	attributes[conventions.AttributeNetPeerName] = pcommon.NewValueStr("db.example.com")
	attributes[conventions.AttributeNetPeerPort] = pcommon.NewValueStr("3306")
	span := constructSQLSpan(attributes)

	filtered, sqlData := makeSQL(span, attributes)
	assert.Nil(t, sqlData)
	assert.NotNil(t, filtered)
}

func TestClientSpanWithoutDBurlAttribute(t *testing.T) {
	attributes := make(map[string]pcommon.Value)
	attributes[conventions.AttributeDBSystem] = pcommon.NewValueStr("postgresql")
	attributes[conventions.AttributeDBName] = pcommon.NewValueStr("customers")
	attributes[conventions.AttributeDBStatement] = pcommon.NewValueStr("SELECT * FROM user WHERE user_id = ?")
	attributes[conventions.AttributeDBUser] = pcommon.NewValueStr("readonly_user")
	attributes[conventions.AttributeDBConnectionString] = pcommon.NewValueStr("")
	attributes[conventions.AttributeNetPeerName] = pcommon.NewValueStr("db.example.com")
	attributes[conventions.AttributeNetPeerPort] = pcommon.NewValueStr("3306")
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
