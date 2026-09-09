// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package sdkbridge provides a generic bridge between go.opentelemetry.io/contrib
// resource detectors and the collector's pdata resource format.
package sdkbridge // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/sdkbridge"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/otel/attribute"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
)

// Detect calls sdkDetector.Detect and copies the returned attributes to a pcommon.Resource.
// Returns an empty resource when the detector determines the process is not running on the
// target platform.
func Detect(ctx context.Context, sdkDetector sdkresource.Detector) (pcommon.Resource, string, error) {
	sdkRes, err := sdkDetector.Detect(ctx)
	if err != nil && !errors.Is(err, sdkresource.ErrPartialResource) {
		return pcommon.NewResource(), "", err
	}

	if sdkRes == nil || sdkRes.Len() == 0 {
		return pcommon.NewResource(), "", nil
	}

	res := pcommon.NewResource()
	iter := sdkRes.Iter()
	for iter.Next() {
		kv := iter.Attribute()
		key := string(kv.Key)
		switch kv.Value.Type() {
		case attribute.BOOL:
			res.Attributes().PutBool(key, kv.Value.AsBool())
		case attribute.INT64:
			res.Attributes().PutInt(key, kv.Value.AsInt64())
		case attribute.FLOAT64:
			res.Attributes().PutDouble(key, kv.Value.AsFloat64())
		case attribute.STRING:
			res.Attributes().PutStr(key, kv.Value.AsString())
		case attribute.BOOLSLICE:
			slice := res.Attributes().PutEmptySlice(key)
			for _, v := range kv.Value.AsBoolSlice() {
				slice.AppendEmpty().SetBool(v)
			}
		case attribute.INT64SLICE:
			slice := res.Attributes().PutEmptySlice(key)
			for _, v := range kv.Value.AsInt64Slice() {
				slice.AppendEmpty().SetInt(v)
			}
		case attribute.FLOAT64SLICE:
			slice := res.Attributes().PutEmptySlice(key)
			for _, v := range kv.Value.AsFloat64Slice() {
				slice.AppendEmpty().SetDouble(v)
			}
		case attribute.STRINGSLICE:
			slice := res.Attributes().PutEmptySlice(key)
			for _, v := range kv.Value.AsStringSlice() {
				slice.AppendEmpty().SetStr(v)
			}
		default:
			res.Attributes().PutStr(key, kv.Value.String())
		}
	}
	return res, sdkRes.SchemaURL(), nil
}
