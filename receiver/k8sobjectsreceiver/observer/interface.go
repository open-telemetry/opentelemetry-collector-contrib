package observer

import "context"

type Observer interface {
	Start(ctx context.Context) chan struct{}
}
