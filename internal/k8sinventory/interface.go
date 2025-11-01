// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sinventory

import (
	"context"
	"sync"
)

type Observer interface {
	Start(ctx context.Context, wg *sync.WaitGroup) chan struct{}
}
