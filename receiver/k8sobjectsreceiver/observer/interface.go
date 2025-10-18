// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package observer // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver/observer"

import (
	"context"
	"sync"
)

type Observer interface {
	Start(ctx context.Context, wg *sync.WaitGroup) chan struct{}
}
