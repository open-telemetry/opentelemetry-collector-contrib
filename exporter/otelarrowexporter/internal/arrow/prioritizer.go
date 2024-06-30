// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package arrow // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter/internal/arrow"

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ErrStreamRestarting = status.Error(codes.Aborted, "stream is restarting")

type PrioritizerName string

var _ component.ConfigValidator = PrioritizerName("")

const (
	DefaultPrioritizer         PrioritizerName = LeastLoadedPrioritizer
	LeastLoadedPrioritizer     PrioritizerName = llPrefix
	LeastLoadedTwoPrioritizer  PrioritizerName = llPrefix + "2"
	LeastLoadedFourPrioritizer PrioritizerName = llPrefix + "4"
	unsetPrioritizer           PrioritizerName = ""

	llPrefix = "leastloaded"
)

// streamPrioritizer is an interface for prioritizing multiple
// streams.
type streamPrioritizer interface {
	// nextWriter gets the next stream writer.  In case the exporter
	// was downgraded, returns nil.
	nextWriter() streamWriter

	// downgrade is called with the root context of the exporter,
	// and may block indefinitely.  this allows the prioritizer to
	// drain its channel(s) until the exporter shuts down.
	downgrade(context.Context)
}

// streamWriter is the caller's interface to a stream.
type streamWriter interface {
	// sendAndWait is called to begin a write.  After completing
	// the call, wait on writeItem.errCh for the response.
	sendAndWait(context.Context, <-chan error, writeItem) error
}

func newStreamPrioritizer(dc doneCancel, name PrioritizerName, numStreams int, maxLifetime time.Duration) (streamPrioritizer, []*streamWorkState) {
	if name == unsetPrioritizer {
		name = DefaultPrioritizer
	}
	if strings.HasPrefix(string(name), llPrefix) {
		// error was checked and reported in Validate
		n, err := strconv.Atoi(string(name[len(llPrefix):]))
		if err == nil {
			return newBestOfNPrioritizer(dc, n, numStreams, pendingRequests, maxLifetime)
		}
	}
	return newBestOfNPrioritizer(dc, numStreams, numStreams, pendingRequests, maxLifetime)
}

// pendingRequests is the load function used by leastloadedN.
func pendingRequests(sws *streamWorkState) float64 {
	sws.lock.Lock()
	defer sws.lock.Unlock()
	return float64(len(sws.waiters) + len(sws.toWrite))
}

// Validate implements component.ConfigValidator
func (p PrioritizerName) Validate() error {
	switch p {
	// Exact match cases
	case LeastLoadedPrioritizer, unsetPrioritizer:
		return nil
	}
	// "leastloadedN" cases
	if !strings.HasPrefix(string(p), llPrefix) {
		return fmt.Errorf("unrecognized prioritizer: %q", string(p))
	}
	_, err := strconv.Atoi(string(p[len(llPrefix):]))
	if err != nil {
		return fmt.Errorf("invalid prioritizer: %q", string(p))
	}
	return nil
}

// drain helps avoid a race condition when downgrade happens, it ensures that
// any late-arriving work will immediately see ErrStreamRestarting, and this
// continues until the exporter shuts down.
//
// Note: the downgrade function is a major source of complexity and it is
// probably best removed, instead of having this level of complexity.
func drain(ch <-chan writeItem, done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case item := <-ch:
			item.errCh <- ErrStreamRestarting
		}
	}
}
