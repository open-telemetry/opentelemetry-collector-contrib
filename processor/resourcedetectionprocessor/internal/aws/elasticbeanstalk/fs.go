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

package elasticbeanstalk // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/elasticbeanstalk"

import (
	"io"
	"os"
	"runtime"
)

type fileSystem interface {
	Open(name string) (io.ReadCloser, error)
	IsWindows() bool
}

type ebFileSystem struct{}

func (fs ebFileSystem) Open(name string) (io.ReadCloser, error) {
	return os.Open(name)
}

func (fs ebFileSystem) IsWindows() bool {
	return runtime.GOOS == "windows"
}
