// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package blobuploadconnector

// Deps provides a means of mocking out dependencies in tests.
type Deps interface {
	depsUnexported()
}

// Default implementation of Deps.
type depsImpl struct{}

func (*depsImpl) depsUnexported() {}

// NewDeps provides access to the default, real version of deps.
func NewDeps() Deps {
	return &depsImpl{}
}
