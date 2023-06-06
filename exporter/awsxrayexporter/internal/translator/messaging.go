// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

const messagingPrefix = "messaging."

func makeMessaging(attributes map[string]pcommon.Value) (map[string]pcommon.Value, map[string]interface{}) {
	var (
		filtered  = make(map[string]pcommon.Value)
		messaging = make(map[string]interface{})
	)

	for key, value := range attributes {
		if strings.HasPrefix(key, messagingPrefix) {
			messaging[strings.TrimPrefix(key, messagingPrefix)] = value.AsRaw()
		} else {
			filtered[key] = value
		}
	}

	return filtered, messaging
}
