// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collection // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver/internal/collection"

import (
	"os"
	"runtime"
)

// packageInfoCollection represents OS package information.
// Supports multiple package managers across different operating systems:
//   - Darwin: homebrew_packages
//   - Linux (Debian): deb_packages
//   - Linux (RPM): rpm_packages
type packageInfoCollection struct{}

func (packageInfoCollection) GetName() string {
	return packageInfoCollectionName
}

func (p packageInfoCollection) GetQuery() string {
	return p.queryForOS(runtime.GOOS)
}

func (packageInfoCollection) queryForOS(goos string) string {
	switch goos {
	case "darwin":
		return packageInfoCollectionQueryHomebrew
	case "linux":
		if fileExistsFn("/etc/debian_version") {
			return packageInfoCollectionQueryDebian
		}
		if fileExistsFn("/etc/redhat-release") {
			return packageInfoCollectionQueryRPM
		}
		return packageInfoCollectionQueryDebian
	default:
		return packageInfoCollectionQueryHomebrew
	}
}

// fileExistsFn is a variable so tests can simulate different Linux package managers
// without requiring debian_version/redhat-release to exist on the test host.
var fileExistsFn = func(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func newPackageInfoCollection() Collection {
	return packageInfoCollection{}
}
