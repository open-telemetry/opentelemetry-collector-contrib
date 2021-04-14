// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package system

import (
	"os"
	"runtime"
	"strings"

	"github.com/Showmax/go-fqdn"
)

type systemMetadata interface {
	// Hostname returns the OS hostname
	Hostname() (string, error)

	// FQDN returns the fully qualified domain name
	FQDN() (string, error)

	// OSType returns the host operating system
	OSType() (string, error)
}

type systemMetadataImpl struct{}

// goosToOSType maps a runtime.GOOS-like value to os.type style.
func goosToOSType(goos string) string {
	switch goos {
	case "dragonfly":
		return "DRAGONFLYBSD"
	}
	return strings.ToUpper(goos)
}

func (*systemMetadataImpl) OSType() (string, error) {
	return goosToOSType(runtime.GOOS), nil
}

func (*systemMetadataImpl) FQDN() (string, error) {
	return fqdn.FqdnHostname()
}

func (*systemMetadataImpl) Hostname() (string, error) {
	return os.Hostname()
}
