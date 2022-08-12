// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resourcehasherprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcehasherprocessor"

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/cespare/xxhash"
	"go.opentelemetry.io/collector/component"
	pcommon "go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	lru "github.com/hashicorp/golang-lru"
)

const LUMIGO_RESOURCE_HASH_ATTRIBUTE_NAME = "lumigo.resource.hash"

type resourceHasherProcessor struct {
	config    Config
	logger    *zap.Logger
	hashCache lru.Cache
}

// Start is invoked during service startup.
func (rhp *resourceHasherProcessor) Start(ctx context.Context, host component.Host) error {
	return nil
}

// processLogs implements the ProcessLogsFunc type.
func (rhp *resourceHasherProcessor) processLogs(_ context.Context, ld plog.Logs) (plog.Logs, error) {
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		mapCopy := pcommon.NewMap()

		// If for some reason we already have a hash (e.g., misconfiguration
		// or misunderstanding by users), remove it
		rls.At(i).Resource().Attributes().Remove(LUMIGO_RESOURCE_HASH_ATTRIBUTE_NAME)

		// Sort attributes to ensure the maximum chance of hash-cache hit
		rls.At(i).Resource().Attributes().Sort()
		rls.At(i).Resource().Attributes().CopyTo(mapCopy)

		hash, keepResourceAttributes := rhp.hashAndCacheResourceAttributes(mapCopy)

		if !keepResourceAttributes {
			rls.At(i).Resource().Attributes().Clear()
		}

		rls.At(i).Resource().Attributes().InsertString(LUMIGO_RESOURCE_HASH_ATTRIBUTE_NAME, hash)
	}
	return ld, nil
}

// processMetrics implements the ProcessMetricsFunc type.
func (rhp *resourceHasherProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		mapCopy := pcommon.NewMap()

		// If for some reason we already have a hash (e.g., misconfiguration
		// or misunderstanding by users), remove it
		rms.At(i).Resource().Attributes().Remove(LUMIGO_RESOURCE_HASH_ATTRIBUTE_NAME)
		rms.At(i).Resource().Attributes().CopyTo(mapCopy)

		hash, keepResourceAttributes := rhp.hashAndCacheResourceAttributes(mapCopy)

		if !keepResourceAttributes {
			rms.At(i).Resource().Attributes().Clear()
		}

		rms.At(i).Resource().Attributes().InsertString(LUMIGO_RESOURCE_HASH_ATTRIBUTE_NAME, hash)
	}
	return md, nil
}

// processTraces implements the ProcessTracesFunc type.
func (rhp *resourceHasherProcessor) processTraces(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		mapCopy := pcommon.NewMap()

		// If for some reason we already have a hash (e.g., misconfiguration
		// or misunderstanding by users), remove it
		rss.At(i).Resource().Attributes().Remove(LUMIGO_RESOURCE_HASH_ATTRIBUTE_NAME)
		// Sort attributes to ensure the maximum chance of hash-cache hit
		rss.At(i).Resource().Attributes().CopyTo(mapCopy)

		hash, keepResourceAttributes := rhp.hashAndCacheResourceAttributes(mapCopy)

		if !keepResourceAttributes {
			rss.At(i).Resource().Attributes().Clear()
		}

		rss.At(i).Resource().Attributes().InsertString(LUMIGO_RESOURCE_HASH_ATTRIBUTE_NAME, hash)
	}

	return td, nil
}

func (rhp *resourceHasherProcessor) hashAndCacheResourceAttributes(m pcommon.Map) (string, bool) {
	digest := xxhash.New()

	rawMap := m.AsRaw()
	sortedKeys := make([]string, 0, len(rawMap))

	for k := range rawMap {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)
	for _, key := range sortedKeys {
		// TODO Optimize and preserve value encoding
		digest.Write([]byte(fmt.Sprintf("%s=\"%s\"\n", key, rawMap[key])))
	}

	hash := fmt.Sprintf("%v", digest.Sum64())
	now := time.Now()
	previous, hashKnown, evicted := rhp.hashCache.PeekOrAdd(hash, now)

	// The processor keeps the original attributes in the resource
	// if the cache has not seen this hash before (and so we are not
	// certain that downstream we have a copy of it) or if the hash we
	// had was stale, and for good measure we will send this copy
	// downstream anew.

	// TODO Make eviction decision based on the difference between `now`
	// and `previous` to ensure we send the full resource at regular
	// intervals (to be made configurable)

	// TODO Expose eviction metrics for the Collector's own monitoring:
	// If we are processing more resources than we can keep hashes for,
	// we will effectively keep evicting entries and the cache will be
	// of little use.
	resendFullResource := false
	if previous != nil {
		elapsedTime := now.Sub(previous.(time.Time))

		resendFullResource = rhp.config.MaximumCacheEntryAge.Seconds() < elapsedTime.Seconds()

		rhp.logger.Debug("resendFullResource", zap.Bool("resendFullResource", resendFullResource), zap.Time("now", now), zap.Time("previous", previous.(time.Time)), zap.Duration("elapsedTime", elapsedTime), zap.Duration("MaximumCacheEntryAge", rhp.config.MaximumCacheEntryAge))
	}

	if resendFullResource {
		// Reset the clock
		rhp.hashCache.Add(hash, now)
	}

	keepAttributes := resendFullResource || !hashKnown

	if entry := rhp.logger.Check(zapcore.DebugLevel, "resourcehasher cache decision"); entry != nil {
		fields := []zap.Field{
			zap.Any("attributes", AttributesToMap(m)),
			zap.Bool("keepAttributes", keepAttributes),
			zap.Bool("hashKnown", hashKnown),
			zap.Bool("evicted", evicted),
			zap.Bool("resendFullResource", resendFullResource),
			zap.Time("now", now),
		}

		if previous != nil {
			fields = append(fields, zap.Time("previous", previous.(time.Time)))
		}

		entry.Write(fields...)
	}

	return hash, keepAttributes
}

func AttributesToMap(am pcommon.Map) map[string]interface{} {
	mp := make(map[string]interface{}, am.Len())
	am.Range(func(k string, v pcommon.Value) bool {
		mp[k] = unwrapAttribute(v)
		return true
	})
	return mp
}

func unwrapAttribute(v pcommon.Value) interface{} {
	switch v.Type() {
	case pcommon.ValueTypeBool:
		return v.BoolVal()
	case pcommon.ValueTypeInt:
		return v.IntVal()
	case pcommon.ValueTypeDouble:
		return v.DoubleVal()
	case pcommon.ValueTypeString:
		return v.StringVal()
	case pcommon.ValueTypeSlice:
		return getSerializableArray(v.SliceVal())
	case pcommon.ValueTypeMap:
		return AttributesToMap(v.MapVal())
	default:
		return nil
	}
}

func getSerializableArray(inArr pcommon.Slice) []interface{} {
	var outArr []interface{}
	for i := 0; i < inArr.Len(); i++ {
		outArr = append(outArr, unwrapAttribute(inArr.At(i)))
	}

	return outArr
}
