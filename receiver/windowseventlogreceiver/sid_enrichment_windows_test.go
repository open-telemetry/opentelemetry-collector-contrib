// Copyright observIQ, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build windows

package windowseventlogreceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/observiq/bindplane-otel-collector/receiver/windowseventlogreceiver/internal/sidcache"
)

// mockCache implements sidcache.Cache for testing
type mockCache struct {
	resolveFunc func(sid string) (*sidcache.ResolvedSID, error)
}

func (m *mockCache) Resolve(sid string) (*sidcache.ResolvedSID, error) {
	if m.resolveFunc != nil {
		return m.resolveFunc(sid)
	}
	return nil, errors.New("not implemented")
}

func (m *mockCache) Close() error {
	return nil
}

func (m *mockCache) Stats() sidcache.CacheStats {
	return sidcache.CacheStats{}
}

func TestNewSIDEnrichingConsumer(t *testing.T) {
	logger := zap.NewNop()
	nextConsumer := new(consumertest.LogsSink)
	cache := &mockCache{}

	consumer := newSIDEnrichingConsumer(nextConsumer, cache, logger)

	require.NotNil(t, consumer)
	assert.Equal(t, nextConsumer, consumer.next)
	assert.Equal(t, cache, consumer.sidCache)
	assert.Equal(t, logger, consumer.logger)
}

func TestSIDEnrichingConsumer_Capabilities(t *testing.T) {
	nextConsumer := new(consumertest.LogsSink)
	cache := &mockCache{}
	consumer := newSIDEnrichingConsumer(nextConsumer, cache, zap.NewNop())

	caps := consumer.Capabilities()
	assert.Equal(t, nextConsumer.Capabilities(), caps)
}

func TestSIDEnrichingConsumer_ConsumeLogs_NilCache(t *testing.T) {
	nextConsumer := new(consumertest.LogsSink)
	consumer := newSIDEnrichingConsumer(nextConsumer, nil, zap.NewNop())

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	lr.Body().SetStr("test log")

	err := consumer.ConsumeLogs(context.Background(), logs)
	require.NoError(t, err)
	assert.Equal(t, 1, nextConsumer.LogRecordCount())
}

func TestSIDEnrichingConsumer_EnrichSecurityField(t *testing.T) {
	testSID := "S-1-5-21-3623811015-3361044348-30300820-1013"
	cache := &mockCache{
		resolveFunc: func(sid string) (*sidcache.ResolvedSID, error) {
			if sid == testSID {
				return &sidcache.ResolvedSID{
					AccountName: "ACME\\jsmith",
					Domain:      "ACME",
					Username:    "jsmith",
					AccountType: "User",
				}, nil
			}
			return nil, errors.New("SID not found")
		},
	}

	nextConsumer := new(consumertest.LogsSink)
	consumer := newSIDEnrichingConsumer(nextConsumer, cache, zap.NewNop())

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()

	// Create body map with security.user_id
	bodyMap := lr.Body().SetEmptyMap()
	securityMap := bodyMap.PutEmptyMap("security")
	securityMap.PutStr("user_id", testSID)

	err := consumer.ConsumeLogs(context.Background(), logs)
	require.NoError(t, err)

	// Verify enrichment
	consumedLogs := nextConsumer.AllLogs()
	require.Len(t, consumedLogs, 1)

	record := consumedLogs[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	secVal, ok := record.Body().Map().Get("security")
	require.True(t, ok)

	sec := secVal.Map()
	userName, ok := sec.Get("user_name")
	require.True(t, ok)
	assert.Equal(t, "ACME\\jsmith", userName.Str())

	domain, ok := sec.Get("domain")
	require.True(t, ok)
	assert.Equal(t, "ACME", domain.Str())

	account, ok := sec.Get("account")
	require.True(t, ok)
	assert.Equal(t, "jsmith", account.Str())

	accountType, ok := sec.Get("account_type")
	require.True(t, ok)
	assert.Equal(t, "User", accountType.Str())
}

func TestSIDEnrichingConsumer_EnrichSecurityField_NoSecurityMap(t *testing.T) {
	cache := &mockCache{
		resolveFunc: func(sid string) (*sidcache.ResolvedSID, error) {
			return &sidcache.ResolvedSID{AccountName: "TEST"}, nil
		},
	}

	nextConsumer := new(consumertest.LogsSink)
	consumer := newSIDEnrichingConsumer(nextConsumer, cache, zap.NewNop())

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	lr.Body().SetStr("no security map")

	err := consumer.ConsumeLogs(context.Background(), logs)
	require.NoError(t, err)
	assert.Equal(t, 1, nextConsumer.LogRecordCount())
}

func TestSIDEnrichingConsumer_EnrichSecurityField_ResolveError(t *testing.T) {
	cache := &mockCache{
		resolveFunc: func(sid string) (*sidcache.ResolvedSID, error) {
			return nil, errors.New("lookup failed")
		},
	}

	nextConsumer := new(consumertest.LogsSink)
	consumer := newSIDEnrichingConsumer(nextConsumer, cache, zap.NewNop())

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()

	bodyMap := lr.Body().SetEmptyMap()
	securityMap := bodyMap.PutEmptyMap("security")
	securityMap.PutStr("user_id", "S-1-5-18")

	err := consumer.ConsumeLogs(context.Background(), logs)
	require.NoError(t, err)

	// Should still pass through without enrichment
	consumedLogs := nextConsumer.AllLogs()
	require.Len(t, consumedLogs, 1)

	record := consumedLogs[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	secVal, ok := record.Body().Map().Get("security")
	require.True(t, ok)

	sec := secVal.Map()
	_, hasUserName := sec.Get("user_name")
	assert.False(t, hasUserName, "Should not have user_name on resolve error")
}

func TestSIDEnrichingConsumer_EnrichEventDataArray(t *testing.T) {
	testSID1 := "S-1-5-18"
	testSID2 := "S-1-5-21-3623811015-3361044348-30300820-1013"

	cache := &mockCache{
		resolveFunc: func(sid string) (*sidcache.ResolvedSID, error) {
			switch sid {
			case testSID1:
				return &sidcache.ResolvedSID{
					AccountName: "NT AUTHORITY\\SYSTEM",
					Domain:      "NT AUTHORITY",
					Username:    "SYSTEM",
					AccountType: "WellKnownGroup",
				}, nil
			case testSID2:
				return &sidcache.ResolvedSID{
					AccountName: "ACME\\jsmith",
					Domain:      "ACME",
					Username:    "jsmith",
					AccountType: "User",
				}, nil
			}
			return nil, errors.New("SID not found")
		},
	}

	nextConsumer := new(consumertest.LogsSink)
	consumer := newSIDEnrichingConsumer(nextConsumer, cache, zap.NewNop())

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()

	// Create body with event_data.data array format
	bodyMap := lr.Body().SetEmptyMap()
	eventDataMap := bodyMap.PutEmptyMap("event_data")
	dataArray := eventDataMap.PutEmptySlice("data")

	// Add item with SubjectUserSid
	item1 := dataArray.AppendEmpty()
	item1Map := item1.SetEmptyMap()
	item1Map.PutStr("SubjectUserSid", testSID1)

	// Add item with TargetUserSid
	item2 := dataArray.AppendEmpty()
	item2Map := item2.SetEmptyMap()
	item2Map.PutStr("TargetUserSid", testSID2)

	err := consumer.ConsumeLogs(context.Background(), logs)
	require.NoError(t, err)

	// Verify enrichment
	consumedLogs := nextConsumer.AllLogs()
	require.Len(t, consumedLogs, 1)

	record := consumedLogs[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	eventDataVal, ok := record.Body().Map().Get("event_data")
	require.True(t, ok)

	eventData := eventDataVal.Map()
	dataVal, ok := eventData.Get("data")
	require.True(t, ok)

	data := dataVal.Slice()
	// Original 2 items + 8 companion fields (4 per SID)
	assert.Equal(t, 10, data.Len())

	// Verify companion fields exist
	hasSubjectResolved := false
	hasTargetResolved := false
	for i := 0; i < data.Len(); i++ {
		item := data.At(i)
		if item.Type() != pcommon.ValueTypeMap {
			continue
		}
		itemMap := item.Map()
		if val, ok := itemMap.Get("SubjectUserSid_Resolved"); ok {
			assert.Equal(t, "NT AUTHORITY\\SYSTEM", val.Str())
			hasSubjectResolved = true
		}
		if val, ok := itemMap.Get("TargetUserSid_Resolved"); ok {
			assert.Equal(t, "ACME\\jsmith", val.Str())
			hasTargetResolved = true
		}
	}
	assert.True(t, hasSubjectResolved, "Should have SubjectUserSid_Resolved field")
	assert.True(t, hasTargetResolved, "Should have TargetUserSid_Resolved field")
}

func TestSIDEnrichingConsumer_EnrichEventDataMap(t *testing.T) {
	testSID := "S-1-5-18"

	cache := &mockCache{
		resolveFunc: func(sid string) (*sidcache.ResolvedSID, error) {
			if sid == testSID {
				return &sidcache.ResolvedSID{
					AccountName: "NT AUTHORITY\\SYSTEM",
					Domain:      "NT AUTHORITY",
					Username:    "SYSTEM",
					AccountType: "WellKnownGroup",
				}, nil
			}
			return nil, errors.New("SID not found")
		},
	}

	nextConsumer := new(consumertest.LogsSink)
	consumer := newSIDEnrichingConsumer(nextConsumer, cache, zap.NewNop())

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()

	// Create body with event_data as flat map
	bodyMap := lr.Body().SetEmptyMap()
	eventDataMap := bodyMap.PutEmptyMap("event_data")
	eventDataMap.PutStr("SubjectUserSid", testSID)
	eventDataMap.PutStr("OtherField", "some value")

	err := consumer.ConsumeLogs(context.Background(), logs)
	require.NoError(t, err)

	// Verify enrichment
	consumedLogs := nextConsumer.AllLogs()
	require.Len(t, consumedLogs, 1)

	record := consumedLogs[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	eventDataVal, ok := record.Body().Map().Get("event_data")
	require.True(t, ok)

	eventData := eventDataVal.Map()

	// Original field should still exist
	subjectSid, ok := eventData.Get("SubjectUserSid")
	require.True(t, ok)
	assert.Equal(t, testSID, subjectSid.Str())

	// Companion fields should exist
	resolved, ok := eventData.Get("SubjectUserSid_Resolved")
	require.True(t, ok)
	assert.Equal(t, "NT AUTHORITY\\SYSTEM", resolved.Str())

	domain, ok := eventData.Get("SubjectUserSid_Domain")
	require.True(t, ok)
	assert.Equal(t, "NT AUTHORITY", domain.Str())

	account, ok := eventData.Get("SubjectUserSid_Account")
	require.True(t, ok)
	assert.Equal(t, "SYSTEM", account.Str())

	accountType, ok := eventData.Get("SubjectUserSid_Type")
	require.True(t, ok)
	assert.Equal(t, "WellKnownGroup", accountType.Str())

	// Other field should be unchanged
	other, ok := eventData.Get("OtherField")
	require.True(t, ok)
	assert.Equal(t, "some value", other.Str())
}

func TestSIDEnrichingConsumer_EnrichEventDataMap_NoEventData(t *testing.T) {
	cache := &mockCache{
		resolveFunc: func(sid string) (*sidcache.ResolvedSID, error) {
			return &sidcache.ResolvedSID{AccountName: "TEST"}, nil
		},
	}

	nextConsumer := new(consumertest.LogsSink)
	consumer := newSIDEnrichingConsumer(nextConsumer, cache, zap.NewNop())

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	lr.Body().SetEmptyMap()

	err := consumer.ConsumeLogs(context.Background(), logs)
	require.NoError(t, err)
	assert.Equal(t, 1, nextConsumer.LogRecordCount())
}

func TestSIDEnrichingConsumer_EnrichEventDataMap_EmptyEventData(t *testing.T) {
	cache := &mockCache{
		resolveFunc: func(sid string) (*sidcache.ResolvedSID, error) {
			return &sidcache.ResolvedSID{AccountName: "TEST"}, nil
		},
	}

	nextConsumer := new(consumertest.LogsSink)
	consumer := newSIDEnrichingConsumer(nextConsumer, cache, zap.NewNop())

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()

	bodyMap := lr.Body().SetEmptyMap()
	bodyMap.PutEmptyMap("event_data")

	err := consumer.ConsumeLogs(context.Background(), logs)
	require.NoError(t, err)
	assert.Equal(t, 1, nextConsumer.LogRecordCount())
}

func TestSIDEnrichingConsumer_MultipleLogRecords(t *testing.T) {
	testSID := "S-1-5-18"
	cache := &mockCache{
		resolveFunc: func(sid string) (*sidcache.ResolvedSID, error) {
			if sid == testSID {
				return &sidcache.ResolvedSID{
					AccountName: "NT AUTHORITY\\SYSTEM",
					Domain:      "NT AUTHORITY",
					Username:    "SYSTEM",
					AccountType: "WellKnownGroup",
				}, nil
			}
			return nil, errors.New("SID not found")
		},
	}

	nextConsumer := new(consumertest.LogsSink)
	consumer := newSIDEnrichingConsumer(nextConsumer, cache, zap.NewNop())

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()

	// Add multiple log records
	for i := 0; i < 3; i++ {
		lr := sl.LogRecords().AppendEmpty()
		bodyMap := lr.Body().SetEmptyMap()
		securityMap := bodyMap.PutEmptyMap("security")
		securityMap.PutStr("user_id", testSID)
	}

	err := consumer.ConsumeLogs(context.Background(), logs)
	require.NoError(t, err)

	// All records should be enriched
	consumedLogs := nextConsumer.AllLogs()
	require.Len(t, consumedLogs, 1)

	records := consumedLogs[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	assert.Equal(t, 3, records.Len())

	for i := 0; i < records.Len(); i++ {
		record := records.At(i)
		secVal, ok := record.Body().Map().Get("security")
		require.True(t, ok)

		sec := secVal.Map()
		userName, ok := sec.Get("user_name")
		require.True(t, ok)
		assert.Equal(t, "NT AUTHORITY\\SYSTEM", userName.Str())
	}
}

func TestSIDEnrichingConsumer_NonMapBody(t *testing.T) {
	cache := &mockCache{
		resolveFunc: func(sid string) (*sidcache.ResolvedSID, error) {
			return &sidcache.ResolvedSID{AccountName: "TEST"}, nil
		},
	}

	nextConsumer := new(consumertest.LogsSink)
	consumer := newSIDEnrichingConsumer(nextConsumer, cache, zap.NewNop())

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	lr.Body().SetStr("string body, not a map")

	err := consumer.ConsumeLogs(context.Background(), logs)
	require.NoError(t, err)

	// Should pass through without error
	assert.Equal(t, 1, nextConsumer.LogRecordCount())
}

func TestSIDEnrichingConsumer_UserIDField(t *testing.T) {
	// Test that "UserID" field (without "Sid" suffix) is also recognized
	testSID := "S-1-5-18"

	cache := &mockCache{
		resolveFunc: func(sid string) (*sidcache.ResolvedSID, error) {
			if sid == testSID {
				return &sidcache.ResolvedSID{
					AccountName: "NT AUTHORITY\\SYSTEM",
					Domain:      "NT AUTHORITY",
					Username:    "SYSTEM",
					AccountType: "WellKnownGroup",
				}, nil
			}
			return nil, errors.New("SID not found")
		},
	}

	nextConsumer := new(consumertest.LogsSink)
	consumer := newSIDEnrichingConsumer(nextConsumer, cache, zap.NewNop())

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()

	bodyMap := lr.Body().SetEmptyMap()
	eventDataMap := bodyMap.PutEmptyMap("event_data")
	eventDataMap.PutStr("UserID", testSID)

	err := consumer.ConsumeLogs(context.Background(), logs)
	require.NoError(t, err)

	// Verify enrichment
	consumedLogs := nextConsumer.AllLogs()
	require.Len(t, consumedLogs, 1)

	record := consumedLogs[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	eventDataVal, ok := record.Body().Map().Get("event_data")
	require.True(t, ok)

	eventData := eventDataVal.Map()

	resolved, ok := eventData.Get("UserID_Resolved")
	require.True(t, ok)
	assert.Equal(t, "NT AUTHORITY\\SYSTEM", resolved.Str())
}
