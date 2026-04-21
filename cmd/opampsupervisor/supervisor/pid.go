// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import "os"

// pidProvider provides the PID of the current process
type pidProvider interface {
	PID() int
}

type defaultPIDProvider struct{}

func (defaultPIDProvider) PID() int {
	return os.Getpid()
}
