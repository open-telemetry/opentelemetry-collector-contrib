// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/translator"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

func addAnnotations(annos map[string]any, attrs pcommon.Map) {
	if len(annos) > 0 {
		keys := attrs.PutEmptySlice(awsxray.AWSXraySegmentAnnotationsAttribute)
		keys.EnsureCapacity(len(annos))
		for k, v := range annos {
			keys.AppendEmpty().SetStr(k)
			switch t := v.(type) {
			case int:
				attrs.PutInt(k, int64(t))
			case int32:
				attrs.PutInt(k, int64(t))
			case int64:
				attrs.PutInt(k, t)
			case string:
				attrs.PutStr(k, t)
			case bool:
				attrs.PutBool(k, t)
			case float32:
				attrs.PutDouble(k, float64(t))
			case float64:
				attrs.PutDouble(k, t)
			default:
			}
		}
	}
}
