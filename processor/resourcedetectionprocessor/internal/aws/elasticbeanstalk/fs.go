// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
