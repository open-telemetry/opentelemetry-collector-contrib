package observer

import (
	"context"
	"sync"
)

type Observer interface {
	Start(ctx context.Context, wg *sync.WaitGroup) chan struct{}
}
