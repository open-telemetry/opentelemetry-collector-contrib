// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package handles

type Manager interface {
	Refresh() error
	GetProcessHandleCount(pid int64) (uint32, error)
}
