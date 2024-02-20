package streams

import (
	"context"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/staleness"
)

func ExpireAfter[T any](ctx context.Context, streams Map[T], ttl time.Duration) Map[T] {
	stale := staleness.NewStaleness(ttl, streams)
	go stale.Start(ctx)
	return stale
}
