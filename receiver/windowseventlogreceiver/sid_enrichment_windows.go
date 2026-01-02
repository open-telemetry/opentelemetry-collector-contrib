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

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/observiq/bindplane-otel-collector/receiver/windowseventlogreceiver/internal/sidcache"
)

// sidEnrichingConsumer wraps a logs consumer to enrich Windows events with SID resolution
type sidEnrichingConsumer struct {
	next     consumer.Logs
	sidCache sidcache.Cache
	logger   *zap.Logger
}

// newSIDEnrichingConsumer creates a new SID enriching consumer wrapper
func newSIDEnrichingConsumer(next consumer.Logs, cache sidcache.Cache, logger *zap.Logger) *sidEnrichingConsumer {
	return &sidEnrichingConsumer{
		next:     next,
		sidCache: cache,
		logger:   logger,
	}
}

// Capabilities returns the consumer capabilities
func (s *sidEnrichingConsumer) Capabilities() consumer.Capabilities {
	return s.next.Capabilities()
}

// ConsumeLogs enriches logs with SID resolution before passing to the next consumer
func (s *sidEnrichingConsumer) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	if s.sidCache == nil {
		// SID resolution disabled, pass through
		return s.next.ConsumeLogs(ctx, logs)
	}

	// Iterate through all log records and enrich them
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		resourceLogs := logs.ResourceLogs().At(i)
		for j := 0; j < resourceLogs.ScopeLogs().Len(); j++ {
			scopeLogs := resourceLogs.ScopeLogs().At(j)
			for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
				logRecord := scopeLogs.LogRecords().At(k)
				s.enrichLogRecord(ctx, logRecord)
			}
		}
	}

	return s.next.ConsumeLogs(ctx, logs)
}

// enrichLogRecord enriches a single log record with SID resolution
func (s *sidEnrichingConsumer) enrichLogRecord(ctx context.Context, record plog.LogRecord) {
	body := record.Body()
	if body.Type() != pcommon.ValueTypeMap {
		return
	}

	bodyMap := body.Map()

	// Enrich security.user_id field
	s.enrichSecurityField(bodyMap)

	// Enrich SID fields in event_data
	s.enrichEventDataFields(bodyMap)
}

// enrichSecurityField enriches the security.user_id field
func (s *sidEnrichingConsumer) enrichSecurityField(bodyMap pcommon.Map) {
	securityVal, ok := bodyMap.Get("security")
	if !ok || securityVal.Type() != pcommon.ValueTypeMap {
		return
	}

	securityMap := securityVal.Map()
	userIDVal, ok := securityMap.Get("user_id")
	if !ok || userIDVal.Type() != pcommon.ValueTypeStr {
		return
	}

	sid := userIDVal.Str()
	if sid == "" {
		return
	}

	// Resolve SID
	resolved, err := s.sidCache.Resolve(sid)
	if err != nil {
		s.logger.Debug("Failed to resolve SID in security field",
			zap.String("sid", sid),
			zap.Error(err))
		return
	}

	// Add resolved fields
	securityMap.PutStr("user_name", resolved.AccountName)
	securityMap.PutStr("domain", resolved.Domain)
	securityMap.PutStr("account", resolved.Username)
	securityMap.PutStr("account_type", resolved.AccountType)
}

// enrichEventDataFields enriches SID fields within event_data
func (s *sidEnrichingConsumer) enrichEventDataFields(bodyMap pcommon.Map) {
	eventDataVal, ok := bodyMap.Get("event_data")
	if !ok || eventDataVal.Type() != pcommon.ValueTypeMap {
		return
	}

	eventDataMap := eventDataVal.Map()

	// Check if event_data has a "data" array (old format)
	dataVal, hasDataArray := eventDataMap.Get("data")
	if hasDataArray && dataVal.Type() == pcommon.ValueTypeSlice {
		s.enrichEventDataArray(dataVal.Slice())
		return
	}

	// Otherwise, treat event_data as a flat map and enrich fields directly
	s.enrichEventDataMap(eventDataMap)
}

// enrichEventDataArray enriches SID fields in the event_data.data array format
func (s *sidEnrichingConsumer) enrichEventDataArray(dataSlice pcommon.Slice) {
	// Track which SIDs we've seen to add companion fields after the original
	sidsToEnrich := make(map[string]*sidcache.ResolvedSID)

	// First pass: identify SID fields and resolve them
	for i := 0; i < dataSlice.Len(); i++ {
		item := dataSlice.At(i)
		if item.Type() != pcommon.ValueTypeMap {
			continue
		}

		itemMap := item.Map()
		itemMap.Range(func(key string, value pcommon.Value) bool {
			// Check if this field name indicates it's a SID
			if !sidcache.IsSIDField(key) {
				return true
			}

			// Check if value is a string that looks like a SID
			if value.Type() != pcommon.ValueTypeStr {
				return true
			}

			sid := value.Str()
			if sid == "" {
				return true
			}

			// Resolve SID
			resolved, err := s.sidCache.Resolve(sid)
			if err != nil {
				s.logger.Debug("Failed to resolve SID in event_data",
					zap.String("field", key),
					zap.String("sid", sid),
					zap.Error(err))
				return true
			}

			sidsToEnrich[key] = resolved
			return true
		})
	}

	// Second pass: add companion fields for each resolved SID
	for fieldName, resolved := range sidsToEnrich {
		// Add {field}_Resolved
		newItem := dataSlice.AppendEmpty()
		newItemMap := newItem.SetEmptyMap()
		newItemMap.PutStr(fieldName+"_Resolved", resolved.AccountName)

		// Add {field}_Domain
		newItem = dataSlice.AppendEmpty()
		newItemMap = newItem.SetEmptyMap()
		newItemMap.PutStr(fieldName+"_Domain", resolved.Domain)

		// Add {field}_Account
		newItem = dataSlice.AppendEmpty()
		newItemMap = newItem.SetEmptyMap()
		newItemMap.PutStr(fieldName+"_Account", resolved.Username)

		// Add {field}_Type
		newItem = dataSlice.AppendEmpty()
		newItemMap = newItem.SetEmptyMap()
		newItemMap.PutStr(fieldName+"_Type", resolved.AccountType)
	}
}

// enrichEventDataMap enriches SID fields in a flat event_data map
func (s *sidEnrichingConsumer) enrichEventDataMap(eventDataMap pcommon.Map) {
	// Track SIDs to enrich (we'll add fields after iteration to avoid modifying during range)
	sidsToEnrich := make(map[string]*sidcache.ResolvedSID)

	// Find all SID fields
	eventDataMap.Range(func(key string, value pcommon.Value) bool {
		// Check if this field name indicates it's a SID
		if !sidcache.IsSIDField(key) {
			return true
		}

		// Check if value is a string that looks like a SID
		if value.Type() != pcommon.ValueTypeStr {
			return true
		}

		sid := value.Str()
		if sid == "" {
			return true
		}

		// Resolve SID
		resolved, err := s.sidCache.Resolve(sid)
		if err != nil {
			s.logger.Debug("Failed to resolve SID in event_data",
				zap.String("field", key),
				zap.String("sid", sid),
				zap.Error(err))
			return true
		}

		sidsToEnrich[key] = resolved
		return true
	})

	// Add companion fields for each resolved SID
	for fieldName, resolved := range sidsToEnrich {
		eventDataMap.PutStr(fieldName+"_Resolved", resolved.AccountName)
		eventDataMap.PutStr(fieldName+"_Domain", resolved.Domain)
		eventDataMap.PutStr(fieldName+"_Account", resolved.Username)
		eventDataMap.PutStr(fieldName+"_Type", resolved.AccountType)
	}
}
