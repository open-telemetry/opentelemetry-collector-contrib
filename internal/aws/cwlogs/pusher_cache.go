// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cwlogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs"

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type pusherCache interface {
	// GetPusher will check to see if there already exists a pusher configured to write to
	// the given stream and group, and that pusher will be returned.
	// If no pusher exists for a given stream and group, a new one is created, stored and returned
	GetPusher(logGroup, logStream string, client Client, retries int) (pusher Pusher)

	// Shutdown will force each stored pusher to flush any data currently buffered
	// and then clear the existing cache.
	Shutdown(ctx context.Context) error

	// ListPushers will return a list of all pushers in the cache.
	ListPushers() []Pusher

	// Flush will cause all pushers to flush to the AWS API
	Flush() error
}

type DefaultPusherCache struct {
	logger                 *zap.Logger
	pusherMapLock          sync.Mutex
	groupStreamToPusherMap map[string]map[string]Pusher
}

func (pc *DefaultPusherCache) GetPusher(logGroup, logStream string, client Client, retries int) (pusher Pusher) {
	pc.pusherMapLock.Lock()
	defer pc.pusherMapLock.Unlock()

	streamToPusherMap, ok := pc.groupStreamToPusherMap[logGroup]
	if !ok {
		streamToPusherMap = map[string]Pusher{}
		pc.groupStreamToPusherMap[logGroup] = streamToPusherMap
	}

	pusher, ok = streamToPusherMap[logStream]
	if !ok {
		pusher = NewPusher(aws.String(logGroup), aws.String(logStream), retries, client, pc.logger)
		streamToPusherMap[logStream] = pusher
	}
	return
}

func (pc *DefaultPusherCache) ListPushers() (pushers []Pusher) {
	pc.pusherMapLock.Lock()
	defer pc.pusherMapLock.Unlock()

	for _, pusherMap := range pc.groupStreamToPusherMap {
		for _, pusher := range pusherMap {
			pushers = append(pushers, pusher)
		}
	}
	return pushers
}

func (pc *DefaultPusherCache) Shutdown(_ context.Context) (errs error) {
	return pc.Flush()
}

func (pc *DefaultPusherCache) Flush() (errs error) {
	for _, pusher := range pc.ListPushers() {
		errs = multierr.Append(errs, pusher.ForceFlush())
	}
	return errs
}

func NewDefaultPusherCache(logger *zap.Logger) PusherCache {
	return &DefaultPusherCache{
		logger:                 logger,
		groupStreamToPusherMap: map[string]map[string]Pusher{},
	}
}
