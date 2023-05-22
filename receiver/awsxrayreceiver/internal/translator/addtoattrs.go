// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/translator"

import "go.opentelemetry.io/collector/pdata/pcommon"

func addBool(val *bool, attrKey string, attrs pcommon.Map) {
	if val != nil {
		attrs.PutBool(attrKey, *val)
	}
}

func addString(val *string, attrKey string, attrs pcommon.Map) {
	if val != nil {
		attrs.PutStr(attrKey, *val)
	}
}

func addInt64(val *int64, attrKey string, attrs pcommon.Map) {
	if val != nil {
		attrs.PutInt(attrKey, *val)
	}
}
