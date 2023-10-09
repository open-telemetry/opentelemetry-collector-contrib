// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splitter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/splitter"

import (
	"bufio"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/flush"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

type Factory interface {
	SplitFunc() bufio.SplitFunc
}

type factory struct {
	splitFunc   bufio.SplitFunc
	trimFunc    trim.Func
	flushPeriod time.Duration
	maxLength   int
}

var _ Factory = (*factory)(nil)

func NewFactory(splitFunc bufio.SplitFunc, trimFunc trim.Func, flushPeriod time.Duration, maxLength int) Factory {
	return &factory{
		splitFunc:   splitFunc,
		trimFunc:    trimFunc,
		flushPeriod: flushPeriod,
		maxLength:   maxLength,
	}
}

// SplitFunc builds a bufio.SplitFunc based on the configuration
func (f *factory) SplitFunc() bufio.SplitFunc {
	// First apply the base splitFunc.
	// If no token is found, we may still flush one based on timing.
	// If a token is emitted for any reason, we must then apply trim rules.
	// We must trim to max length _before_ trimming whitespace because otherwise we
	// cannot properly keep track of the number of bytes to advance.
	// For instance, if we have advance: 5, token: []byte(" foo "):
	//   Trimming whitespace first would result in advance: 5, token: []byte("foo")
	//   Then if we trim to max length of 2, we don't know whether or not to reduce advance.
	return trim.WithFunc(trim.ToLength(flush.WithPeriod(f.splitFunc, f.flushPeriod), f.maxLength), f.trimFunc)
}
