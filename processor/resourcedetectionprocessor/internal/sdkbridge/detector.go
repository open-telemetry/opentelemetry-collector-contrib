// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package sdkbridge provides a generic bridge between go.opentelemetry.io/contrib
// resource detectors and the collector's pdata resource format.
package sdkbridge // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/sdkbridge"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
)

// Detect calls sdkDetector.Detect and copies the returned attributes to a pcommon.Resource.
// Only attributes whose key appears in enabledAttrs with a true value are included. Returns an empty
// resource when the detector determines the process is not running on the
// target platform.
func Detect(ctx context.Context, sdkDetector sdkresource.Detector, enabledAttrs map[string]bool) (pcommon.Resource, string, error) {
	sdkRes, err := sdkDetector.Detect(ctx)
	if err != nil && !errors.Is(err, sdkresource.ErrPartialResource) {
		return pcommon.NewResource(), "", err
	}

	if sdkRes.Len() == 0 {
		return pcommon.NewResource(), "", nil
	}

	res := pcommon.NewResource()
	iter := sdkRes.Iter()
	for iter.Next() {
		kv := iter.Attribute()
		key := string(kv.Key)
		if enabledAttrs[key] {
			res.Attributes().PutStr(key, kv.Value.AsString())
		}
	}
	return res, sdkRes.SchemaURL(), nil
}
