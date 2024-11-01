// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package blobuploadprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/blobuploadprocessor"

// Deps provides a means of mocking out dependencies in tests.
type deps interface {
	depsUnexported()
}

// Default implementation of deps.
type depsImpl struct{}

func (*depsImpl) depsUnexported() {}

// NewDeps provides access to the default, real version of deps.
func newDeps() deps {
	return &depsImpl{}
}
