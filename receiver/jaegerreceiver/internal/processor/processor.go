// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processor

type Processor interface {
	Start() error
	Stop() error
	Done() <-chan struct{}
}
