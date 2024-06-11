// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension"

import "os"

func init() {
	// fqdn seems to hang running in github actions on darwin amd64. This
	// bypasses it.
	// https://github.com/SumoLogic/sumologic-otel-collector/issues/1295
	hostname = os.Hostname
}
