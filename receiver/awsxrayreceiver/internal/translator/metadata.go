// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/translator"

import (
	"encoding/json"

	"go.opentelemetry.io/collector/pdata/pcommon"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

func addMetadata(meta map[string]map[string]interface{}, attrs pcommon.Map) error {
	for k, v := range meta {
		val, err := json.Marshal(v)
		if err != nil {
			return err
		}
		attrs.PutStr(
			awsxray.AWSXraySegmentMetadataAttributePrefix+k, string(val))
	}
	return nil
}
