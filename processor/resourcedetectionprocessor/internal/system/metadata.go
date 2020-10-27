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
	"context"
	"fmt"
	"os"
	"runtime"
)

type systemMetadata interface {
	// FQDNAvailable states whether the FQDN can be retrieved
	FQDNAvailable() bool

	// FQDN returns the fully qualified domain name
	FQDN(ctx context.Context) (string, error)

	// Hostname returns the system hostname
	Hostname() (string, error)

	// HostType returns the host operating system and architecture
	HostType() (string, error)
}

type systemMetadataImpl struct{}

func (*systemMetadataImpl) Hostname() (string, error) {
	return os.Hostname()
}

func (*systemMetadataImpl) HostType() (string, error) {
	return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH), nil
}
