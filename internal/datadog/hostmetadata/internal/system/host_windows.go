// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package system // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/internal/system"

func getSystemFQDN() (string, error) {
	// The Datadog Agent uses CGo to get the FQDN of the host
	// OpenTelemetry does not allow the use of CGo so this feature
	// is disabled on Windows
	return "", nil
}
