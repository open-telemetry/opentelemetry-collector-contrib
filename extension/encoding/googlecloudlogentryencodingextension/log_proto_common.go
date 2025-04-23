// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension"

import (
	// support decoding audit logs
	_ "google.golang.org/genproto/googleapis/appengine/logging/v1"
	_ "google.golang.org/genproto/googleapis/cloud/audit"
)
